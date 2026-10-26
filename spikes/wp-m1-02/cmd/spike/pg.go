package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/rluders/amaru/spikes/wp-m1-02/internal/pgsup"
)

// result is one line of the evidence record.
type result struct {
	Scenario string         `json:"scenario"`
	Pass     bool           `json:"pass"`
	Detail   string         `json:"detail,omitempty"`
	Metrics  map[string]any `json:"metrics,omitempty"`
}

type recorder struct {
	w   io.Writer
	all []result
}

func (r *recorder) add(x result) {
	r.all = append(r.all, x)
	b, _ := json.Marshal(x)
	fmt.Fprintln(r.w, string(b))
}

func cmdPG(args []string) error {
	if len(args) > 0 && args[0] == "owner" {
		return pgOwner(args[1:])
	}
	fl := flag.NewFlagSet("pg", flag.ExitOnError)
	bundle := fl.String("bundle", "", "bundle directory made by `spike bundle`")
	state := fl.String("state", "", "empty state directory (created)")
	out := fl.String("out", "", "also write JSON lines here")
	allowElevated := fl.Bool("allow-elevated", false, "record an elevated (admin/root) user as a limitation instead of a failure; for hosted CI runners, which cannot run as a standard user")
	fl.Parse(args)
	if *bundle == "" || *state == "" {
		return fmt.Errorf("-bundle and -state required")
	}
	if _, err := os.Stat(*state); err == nil {
		return fmt.Errorf("state dir %s exists; the scenarios need a fresh one", *state)
	}
	var w io.Writer = os.Stdout
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			return err
		}
		defer f.Close()
		w = io.MultiWriter(os.Stdout, f)
	}
	rec := &recorder{w: w}
	b := pgsup.Bundle{Dir: *bundle}
	ctx := context.Background()

	isAdmin, adminDetail := elevated()
	envPass := !isAdmin
	if isAdmin && *allowElevated {
		envPass = true
		adminDetail += "; ELEVATED run accepted by -allow-elevated: non-admin behavior is NOT validated by this run"
	}
	rec.add(result{Scenario: "env", Pass: envPass, Detail: adminDetail, Metrics: map[string]any{
		"goos": runtime.GOOS, "goarch": runtime.GOARCH, "go": runtime.Version(), "elevated": isAdmin,
	}})

	// 1. cold start (initdb) + security properties
	inst, err := pgsup.Start(ctx, b, *state)
	if err != nil {
		rec.add(result{Scenario: "cold-start", Detail: err.Error()})
		return err
	}
	conn, err := pgx.Connect(ctx, inst.URL)
	if err != nil {
		rec.add(result{Scenario: "cold-start", Detail: "connect: " + err.Error()})
		return err
	}
	var version, listen, pwEnc, sockDirs string
	conn.QueryRow(ctx, "select version()").Scan(&version)
	conn.QueryRow(ctx, "show listen_addresses").Scan(&listen)
	conn.QueryRow(ctx, "show password_encryption").Scan(&pwEnc)
	conn.QueryRow(ctx, "show unix_socket_directories").Scan(&sockDirs)
	var hbaAuth string
	conn.QueryRow(ctx, "select string_agg(distinct auth_method, ',') from pg_hba_file_rules").Scan(&hbaAuth)
	wantListen := "127.0.0.1"
	if inst.Socket {
		wantListen = ""
	}
	rec.add(result{Scenario: "cold-start", Pass: listen == wantListen && pwEnc == "scram-sha-256" && !strings.Contains(hbaAuth, "trust"),
		Detail: fmt.Sprintf("version=%q listen_addresses=%q password_encryption=%s unix_socket_directories=%q hba_auth=%s url_host=%s",
			version, listen, pwEnc, sockDirs, hbaAuth, redactURL(inst.URL)),
		Metrics: map[string]any{"start_ms": inst.StartedIn.Milliseconds(), "unix_socket": inst.Socket}})

	// 2. transactional write, then idle memory + footprint while running
	_, err = conn.Exec(ctx, "create table spike(id int primary key, v text)")
	if err == nil {
		_, err = conn.Exec(ctx, "insert into spike values (1,'committed')")
	}
	rec.add(result{Scenario: "write", Pass: err == nil, Detail: errStr(err)})
	pm := postmasterPID(*state)
	pids := append([]int{pm}, descendants(pm)...)
	rss, ok := rssKB(pids)
	rec.add(result{Scenario: "footprint", Pass: ok, Metrics: map[string]any{
		"bundle_pg_bytes":             dirSize(b.PG()),
		"bundle_supervisor_bytes":     fileSize(b.Supervisor()),
		"data_dir_bytes_after_initdb": dirSize(filepath.Join(*state, "data")),
		"idle_rss_kb_postgres_tree":   rss, "postgres_tree_pids": len(pids),
	}})
	conn.Close(ctx)

	// 3. graceful stop
	t0 := time.Now()
	err = inst.Close()
	stopMs := time.Since(t0).Milliseconds()
	rec.add(result{Scenario: "graceful-stop", Pass: err == nil && !alive(pm), Detail: errStr(err), Metrics: map[string]any{"stop_ms": stopMs}})

	// 4. warm restart: data persisted, clean shutdown recorded
	inst, err = pgsup.Start(ctx, b, *state)
	if err != nil {
		rec.add(result{Scenario: "warm-restart", Detail: err.Error()})
		return err
	}
	rows := countRows(ctx, inst.URL)
	rec.add(result{Scenario: "warm-restart", Pass: rows == 1, Detail: fmt.Sprintf("rows=%d", rows), Metrics: map[string]any{"start_ms": inst.StartedIn.Milliseconds()}})
	inst.Close()

	// 5. owner killed -9: the supervisor must stop PostgreSQL
	rec.add(ownerKill(ctx, b, *state))

	// 6. postmaster killed -9 while the owner lives
	rec.add(postmasterKill(ctx, b, *state))

	// 7. relocated binaries (same version) over the same data directory
	rec.add(relocate(ctx, b, *state))

	failed := 0
	for _, r := range rec.all {
		if !r.Pass {
			failed++
		}
	}
	fmt.Fprintf(os.Stderr, "scenarios=%d failed=%d\n", len(rec.all), failed)
	if failed > 0 {
		return fmt.Errorf("%d scenario(s) failed", failed)
	}
	return nil
}

// pgOwner is the child process that owns PostgreSQL ("the Platform").
func pgOwner(args []string) error {
	fl := flag.NewFlagSet("owner", flag.ExitOnError)
	bundle := fl.String("bundle", "", "")
	state := fl.String("state", "", "")
	fl.Parse(args)
	inst, err := pgsup.Start(context.Background(), pgsup.Bundle{Dir: *bundle}, *state)
	if err != nil {
		return err
	}
	defer inst.Close()
	fmt.Printf("READY %d %s\n", os.Getpid(), inst.URL)
	io.Copy(io.Discard, os.Stdin) // block until the parent closes stdin or kills us
	return nil
}

func startOwner(b pgsup.Bundle, state string) (*exec.Cmd, int, string, error) {
	exe, _ := os.Executable()
	cmd := exec.Command(exe, "pg", "owner", "-bundle", b.Dir, "-state", state)
	cmd.Stderr = os.Stderr
	in, _ := cmd.StdinPipe()
	_ = in
	outp, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		return nil, 0, "", err
	}
	line, err := bufio.NewReader(outp).ReadString('\n')
	if err != nil {
		cmd.Process.Kill()
		return nil, 0, "", fmt.Errorf("owner did not become ready: %w", err)
	}
	f := strings.Fields(line)
	if len(f) != 3 || f[0] != "READY" {
		return nil, 0, "", fmt.Errorf("bad ready line %q", line)
	}
	pid, _ := strconv.Atoi(f[1])
	return cmd, pid, f[2], nil
}

func ownerKill(ctx context.Context, b pgsup.Bundle, state string) result {
	cmd, _, url, err := startOwner(b, state)
	if err != nil {
		return result{Scenario: "owner-kill", Detail: err.Error()}
	}
	pm := postmasterPID(state)
	_, err = execSQL(ctx, url, "insert into spike values (2,'committed-before-owner-kill')")
	if err != nil {
		return result{Scenario: "owner-kill", Detail: "insert: " + err.Error()}
	}
	t0 := time.Now()
	cmd.Process.Kill()
	cmd.Wait()
	var gone time.Duration
	for gone = 0; gone < 30*time.Second && alive(pm); gone = time.Since(t0) {
		time.Sleep(50 * time.Millisecond)
	}
	dead := !alive(pm)
	// Restart on the same data directory and look at what PostgreSQL logged.
	inst, err := pgsup.Start(ctx, b, state)
	if err != nil {
		return result{Scenario: "owner-kill", Detail: "restart: " + err.Error()}
	}
	rows := countRows(ctx, inst.URL)
	inst.Close()
	return result{Scenario: "owner-kill", Pass: dead && rows == 2,
		Detail:  fmt.Sprintf("postmaster_gone=%v rows_after_restart=%d (want 2); log: %s", dead, rows, logHint(state)),
		Metrics: map[string]any{"postgres_exit_after_owner_kill_ms": gone.Milliseconds()}}
}

func postmasterKill(ctx context.Context, b pgsup.Bundle, state string) result {
	inst, err := pgsup.Start(ctx, b, state)
	if err != nil {
		return result{Scenario: "postmaster-kill", Detail: err.Error()}
	}
	conn, err := pgx.Connect(ctx, inst.URL)
	if err != nil {
		return result{Scenario: "postmaster-kill", Detail: err.Error()}
	}
	// an uncommitted transaction must vanish; a committed row must survive
	tx, _ := conn.Begin(ctx)
	tx.Exec(ctx, "insert into spike values (99,'uncommitted')")
	if _, err := execSQL(ctx, inst.URL, "insert into spike values (3,'committed-before-postmaster-kill')"); err != nil {
		return result{Scenario: "postmaster-kill", Detail: "insert: " + err.Error()}
	}
	pm := postmasterPID(state)
	killPID(pm)
	t0 := time.Now()
	var observed time.Duration
	select {
	case <-inst.DB.Done():
		observed = time.Since(t0)
	case <-time.After(30 * time.Second):
		observed = -1
	}
	libErr := inst.DB.Err()
	inst.Close()
	inst2, err := pgsup.Start(ctx, b, state)
	if err != nil {
		return result{Scenario: "postmaster-kill", Detail: "restart: " + err.Error()}
	}
	rows := countRows(ctx, inst2.URL)
	var uncommitted int
	if c, e := pgx.Connect(ctx, inst2.URL); e == nil {
		c.QueryRow(ctx, "select count(*) from spike where id=99").Scan(&uncommitted)
		c.Close(ctx)
	}
	inst2.Close()
	return result{Scenario: "postmaster-kill", Pass: observed >= 0 && rows == 3 && uncommitted == 0,
		Detail:  fmt.Sprintf("library_noticed_after=%v Err=%v rows=%d (want 3) uncommitted_rows=%d (want 0); log: %s", observed, libErr, rows, uncommitted, logHint(state)),
		Metrics: map[string]any{"library_done_after_ms": observed.Milliseconds()}}
}

func relocate(ctx context.Context, b pgsup.Bundle, state string) result {
	alt := filepath.Join(filepath.Dir(state), filepath.Base(state)+"-bundle2")
	if err := copyTree(b.Dir, alt); err != nil {
		return result{Scenario: "relocated-binaries", Detail: err.Error()}
	}
	defer os.RemoveAll(alt)
	inst, err := pgsup.Start(ctx, pgsup.Bundle{Dir: alt}, state)
	if err != nil {
		return result{Scenario: "relocated-binaries", Detail: err.Error()}
	}
	rows := countRows(ctx, inst.URL)
	inst.Close()
	return result{Scenario: "relocated-binaries", Pass: rows == 3,
		Detail: fmt.Sprintf("rows=%d. Same PostgreSQL version at a new path: this is NOT a minor-version upgrade test (not validated, see report)", rows)}
}

// helpers

func alive(pid int) bool { return pgsup.Alive(pid) }

func postmasterPID(state string) int {
	b, err := os.ReadFile(filepath.Join(state, "data", "postmaster.pid"))
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(strings.SplitN(string(b), "\n", 2)[0]))
	return n
}

func countRows(ctx context.Context, url string) int {
	c, err := pgx.Connect(ctx, url)
	if err != nil {
		return -1
	}
	defer c.Close(ctx)
	var n int
	c.QueryRow(ctx, "select count(*) from spike where id<99").Scan(&n)
	return n
}

func execSQL(ctx context.Context, url, sql string) (int64, error) {
	c, err := pgx.Connect(ctx, url)
	if err != nil {
		return 0, err
	}
	defer c.Close(ctx)
	t, err := c.Exec(ctx, sql)
	return t.RowsAffected(), err
}

func logHint(state string) string {
	b, err := os.ReadFile(filepath.Join(state, "postgres.log"))
	if err != nil {
		return err.Error()
	}
	var keep []string
	for _, l := range strings.Split(string(b), "\n") {
		if strings.Contains(l, "not properly shut down") || strings.Contains(l, "automatic recovery") ||
			strings.Contains(l, "shutdown requested") || strings.Contains(l, "database system is shut down") {
			keep = append(keep, strings.TrimSpace(l))
		}
	}
	if len(keep) > 6 {
		keep = keep[len(keep)-6:]
	}
	return strings.Join(keep, " | ")
}

func fileSize(p string) int64 {
	if i, err := os.Stat(p); err == nil {
		return i.Size()
	}
	return -1
}

func errStr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// redactURL hides the password in connection URLs before they reach a record.
func redactURL(u string) string {
	if i := strings.Index(u, "://"); i >= 0 {
		if j := strings.Index(u[i:], "@"); j >= 0 {
			return u[:i+3] + "***" + u[i+j:]
		}
	}
	return u
}
