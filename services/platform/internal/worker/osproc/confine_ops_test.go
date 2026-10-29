package osproc

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// runOps is the confined child of the confinement tests: it tries each operation given as an
// argument and prints one line per operation, "<op> -> ok|denied|error: ...". Operations use "|"
// to separate their paths.
func runOps(ops []string) {
	for _, op := range ops {
		fmt.Printf("%s -> %s\n", op, outcome(do(op)))
	}
	os.Exit(0)
}

func outcome(err error) string {
	switch {
	case err == nil:
		return "ok"
	// Landlock refuses a rename or link across directories with EXDEV, not EACCES; both temporary
	// directories of a test are on one filesystem, so EXDEV here is the confinement.
	case errors.Is(err, fs.ErrPermission), errors.Is(err, syscall.EACCES), errors.Is(err, syscall.EPERM), errors.Is(err, syscall.EXDEV):
		return "denied"
	}
	return "error: " + err.Error()
}

func do(op string) error {
	kind, arg, _ := strings.Cut(op, ":")
	p := strings.Split(arg, "|")
	switch kind {
	case "write":
		return os.WriteFile(p[0], []byte("agent"), 0o644)
	case "append":
		f, err := os.OpenFile(p[0], os.O_WRONLY|os.O_APPEND, 0)
		if err == nil {
			_, err = f.WriteString("agent")
			f.Close()
		}
		return err
	case "trunc":
		return os.Truncate(p[0], 0)
	case "mkdir":
		return os.Mkdir(p[0], 0o755)
	case "remove":
		return os.Remove(p[0])
	case "symlink":
		return os.Symlink(p[0], p[1])
	case "rename":
		return os.Rename(p[0], p[1])
	case "link":
		return os.Link(p[0], p[1])
	case "read":
		_, err := os.ReadFile(p[0])
		return err
	case "devnull":
		return os.WriteFile("/dev/null", []byte("x"), 0)
	case "marker":
		if os.Getenv(confineEnv) != "" {
			return errors.New("the launch-helper marker reached the agent")
		}
		return nil
	case "setsid": // the same operation from a descendant that left the session and group
		exe, _ := os.Executable()
		cmd := exec.Command(exe, arg)
		cmd.Env = []string{helperEnv + "=confine-ops"}
		detach(cmd, true) // its own session, outside the process group
		out, err := cmd.Output()
		if err != nil {
			return err
		}
		if strings.Contains(string(out), "-> ok") {
			return nil
		}
		return errors.New(strings.TrimSpace(string(out)))
	}
	return errors.New("unknown operation " + kind)
}
