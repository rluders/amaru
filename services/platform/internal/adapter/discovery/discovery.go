// Package discovery publishes compact deployment credentials privately per OS
// user. Its lock is held for the Instance lifetime, not just during a write.
package discovery

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const FileName = "instance.json"
const lockName = "instance.lock"

var ErrRunning = errors.New("an Instance already holds the discovery lock")
var errUnsafe = errors.New("unsafe discovery state")

type Info struct {
	Endpoint            string    `json:"endpoint"`
	Port                int       `json:"port"`
	PID                 int       `json:"pid"`
	AdministratorSecret string    `json:"administratorSecret"`
	ProtocolVersion     int       `json:"protocolVersion"`
	PlatformVersion     string    `json:"platformVersion"`
	StartedAt           time.Time `json:"startedAt"`
}

func (i Info) valid() bool {
	u, err := url.Parse(i.Endpoint)
	if err != nil || u.Scheme != "http" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return false
	}
	ip := net.ParseIP(u.Hostname())
	if ip == nil || !ip.IsLoopback() || i.Port < 1 || i.Port > 65535 || u.Host != net.JoinHostPort(u.Hostname(), strconv.Itoa(i.Port)) {
		return false
	}
	b, err := base64.RawURLEncoding.Strict().DecodeString(i.AdministratorSecret)
	return err == nil && len(b) == 32 && i.PID > 0 && i.ProtocolVersion == 1 && i.PlatformVersion != "" && !i.StartedAt.IsZero()
}

type Publication struct {
	root *os.Root
	lock *os.File
	info Info
}

// Publish takes ownership only after the loopback socket is bound. The caller
// must retain Publication until the server stops, then Close it. Unsafe existing
// paths are refused; supported permissions are currently native Linux only.
func Publish(dir string, listener net.Listener, secret, version string, started time.Time) (*Publication, error) {
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok || !addr.IP.IsLoopback() {
		return nil, errUnsafe
	}
	info := Info{Endpoint: "http://" + addr.String(), Port: addr.Port, PID: os.Getpid(), AdministratorSecret: secret, ProtocolVersion: 1, PlatformVersion: version, StartedAt: started.UTC()}
	if !info.valid() {
		return nil, errUnsafe
	}
	root, err := openPrivateDirectory(dir, true)
	if err != nil {
		return nil, err
	}
	lock, err := privateFile(root, lockName, os.O_RDWR|os.O_CREATE)
	if err != nil {
		root.Close()
		return nil, err
	}
	if err = lockExclusive(lock); err != nil {
		lock.Close()
		root.Close()
		return nil, err
	}
	p := &Publication{root: root, lock: lock, info: info}
	if err = p.write(); err != nil {
		p.release()
		return nil, err
	}
	return p, nil
}
func privateFile(root *os.Root, name string, flags int) (*os.File, error) {
	old, err := root.Lstat(name)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, errUnsafe
	}
	if errors.Is(err, os.ErrNotExist) && flags&os.O_CREATE == 0 {
		return nil, os.ErrNotExist
	}
	if err == nil && !private(old, false) {
		return nil, errUnsafe
	}
	f, err := root.OpenFile(name, flags, 0600)
	if err != nil {
		return nil, errUnsafe
	}
	st, err := f.Stat()
	if err != nil || !private(st, false) || (old != nil && !os.SameFile(old, st)) {
		f.Close()
		return nil, errUnsafe
	}
	return f, nil
}
func (p *Publication) write() error {
	if st, err := p.root.Lstat(FileName); err == nil {
		if !private(st, false) {
			return errUnsafe
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return errUnsafe
	}
	var name string
	for range 10 {
		var b [16]byte
		if _, err := io.ReadFull(randomSource, b[:]); err != nil {
			return errors.New("discovery entropy unavailable")
		}
		name = ".instance-" + base64.RawURLEncoding.EncodeToString(b[:]) + ".tmp"
		f, err := p.root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return errUnsafe
		}
		defer p.root.Remove(name)
		data, err := json.Marshal(p.info)
		if err == nil {
			_, err = f.Write(append(data, '\n'))
		}
		if err == nil {
			err = f.Sync()
		}
		closeErr := f.Close()
		if err != nil || closeErr != nil {
			return errors.New("discovery write failed")
		}
		if err = p.root.Rename(name, FileName); err != nil {
			return errors.New("discovery publication failed")
		}
		return nil
	}
	return errors.New("discovery temporary file unavailable")
}

// Close is called once after serving stops. The held cross-process lock keeps
// replacement publication out of the ownership-check/remove interval.
func (p *Publication) Close() error {
	if p.root == nil {
		return nil
	}
	current, err := read(p.root)
	if err == nil && current == p.info {
		err = p.root.Remove(FileName)
	}
	if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	p.release()
	return err
}
func (p *Publication) release() { p.lock.Close(); p.root.Close(); p.root = nil }

// Read never accepts a symlink, permissive mode or malformed record.
func Read(dir string) (Info, error) {
	root, err := openPrivateDirectory(dir, false)
	if err != nil {
		return Info{}, err
	}
	defer root.Close()
	return read(root)
}
func read(root *os.Root) (Info, error) {
	f, err := privateFile(root, FileName, os.O_RDONLY)
	if err != nil {
		return Info{}, err
	}
	defer f.Close()
	var info Info
	data, err := io.ReadAll(io.LimitReader(f, 8193))
	if err != nil || len(data) > 8192 {
		return Info{}, errUnsafe
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if d.Decode(&info) != nil || !info.valid() {
		return Info{}, errUnsafe
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return Info{}, errUnsafe
	}
	return info, nil
}

// Live performs a credential-free, bounded health probe without redirects,
// proxies or hostname resolution. A health response is not proof of identity;
// private discovery ownership is the compact trust boundary.
func Live(ctx context.Context, info Info) bool {
	if !info.valid() {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", info.Endpoint+"/v1/health", nil)
	if err != nil {
		return false
	}
	transport := &http.Transport{Proxy: nil}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return false
	}
	var health struct {
		Status          string    `json:"status"`
		ProtocolVersion int       `json:"protocolVersion"`
		PlatformVersion string    `json:"platformVersion"`
		StartedAt       time.Time `json:"startedAt"`
		UptimeSec       *int64    `json:"uptimeSec"`
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4097))
	if err != nil || len(data) > 4096 {
		return false
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if d.Decode(&health) != nil {
		return false
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return false
	}
	return health.Status == "ok" && health.ProtocolVersion == info.ProtocolVersion && health.PlatformVersion == info.PlatformVersion && health.StartedAt.Equal(info.StartedAt) && health.UptimeSec != nil && *health.UptimeSec >= 0 && !strings.Contains(info.Endpoint, "?")
}
