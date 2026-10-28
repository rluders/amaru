// Package attach registers an existing folder as a Project (FEAT-PRJ-001). It reads the
// repository's identity with the git CLI through the process port and stores a domain.Project
// keyed as ADR-0046 states. It writes nothing into the folder (ADR-0043): the Project record
// lives in the Platform store, and a repository with no remote must already carry its committed
// id.
//
// The folder is untrusted until a human trusts it (FEAT-PRJ-003). Attaching therefore only runs
// git commands that read: rev-parse, rev-list, config and cat-file. None of them starts a hook,
// a filter, a pager, a file-system monitor or a textconv program, and cat-file reads the stored
// blob without applying attribute filters. The package never reads a file from the working tree.
package attach

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// maxOutput bounds what is read from one git command, so a hostile repository cannot make the
// Platform buffer without limit.
const maxOutput = 1 << 20

// configPath is the committed Project configuration (AMARU_DIRECTORY §3.1).
const configPath = ".amaru/config.yaml"

// Attacher attaches folders. Git runs through Runner; Resolver finds the git executable.
type Attacher struct {
	Runner   ports.ProcessRunner
	Resolver ports.ExecutableResolver
	Projects ports.ProjectStore
	Now      func() time.Time
}

// Attach registers the repository that contains path and returns its Project. created is false
// when the Project was already attached: attaching again, from any clone or any spelling of the
// path, returns the stored Project unchanged.
//
// It returns domain.ErrInvalid, with the reason, for a folder that is not a usable repository:
// not a git repository, a bare or shallow clone, no commit, several remotes and none named
// origin, or no remote and no committed Project id.
func (a Attacher) Attach(ctx context.Context, path string) (p domain.Project, created bool, err error) {
	if strings.TrimSpace(path) == "" {
		return domain.Project{}, false, invalid("path is empty")
	}
	dir, err := filepath.Abs(path)
	if err != nil {
		return domain.Project{}, false, invalid("path %q: %v", path, err)
	}
	top, err := a.one(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return domain.Project{}, false, invalid("%s is not a usable git working tree: %v", dir, err)
	}
	if shallow, err := a.one(ctx, top, "rev-parse", "--is-shallow-repository"); err != nil {
		return domain.Project{}, false, err
	} else if shallow == "true" {
		return domain.Project{}, false, invalid("%s is a shallow clone: its root commit is not known, so its key would differ from a full clone's", top)
	}
	cfg, err := a.committedConfig(ctx, top)
	if err != nil {
		return domain.Project{}, false, err
	}
	key, err := a.key(ctx, top, cfg)
	if err != nil {
		return domain.Project{}, false, err
	}
	name := cfg.Project.Name
	if name == "" {
		name = filepath.Base(top)
	}
	p = domain.Project{Key: key, Name: name, CreatedAt: a.Now()}
	if err := p.Validate(); err != nil {
		return domain.Project{}, false, err
	}
	switch err := a.Projects.Create(ctx, p); {
	case err == nil:
		return p, true, nil
	case errors.Is(err, domain.ErrExists):
		got, gerr := a.Projects.Get(ctx, key)
		return got, false, gerr
	default:
		return domain.Project{}, false, err
	}
}

// projectConfig is the part of the committed .amaru/config.yaml that attach reads.
type projectConfig struct {
	Project struct {
		ID   string `yaml:"id"`
		Name string `yaml:"name"`
	} `yaml:"project"`
}

// committedConfig reads .amaru/config.yaml as committed at HEAD, never from the working tree.
// A missing file is an empty config.
func (a Attacher) committedConfig(ctx context.Context, top string) (projectConfig, error) {
	var cfg projectConfig
	if _, err := a.run(ctx, top, "rev-parse", "--verify", "-q", "HEAD"); err != nil {
		return cfg, invalid("%s has no commit to derive a key from", top)
	}
	listed, err := a.one(ctx, top, "ls-tree", "HEAD", "--", configPath)
	if err != nil {
		return cfg, err
	}
	// "<mode> blob <id>\t<path>": only a regular committed file is read, not a symlink or a tree.
	if !strings.HasPrefix(listed, "100644 blob ") && !strings.HasPrefix(listed, "100755 blob ") {
		return cfg, nil
	}
	out, err := a.run(ctx, top, "cat-file", "blob", "HEAD:"+configPath)
	if err != nil {
		return cfg, err
	}
	if err := yaml.NewDecoder(bytes.NewReader(out)).Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
		return cfg, invalid("committed %s: %v", configPath, err)
	}
	return cfg, nil
}

// key derives the Project key: normalized remote + root commit, or the committed id when the
// repository has no remote.
func (a Attacher) key(ctx context.Context, top string, cfg projectConfig) (domain.ProjectKey, error) {
	remote, err := a.remoteURL(ctx, top)
	if err != nil {
		return "", err
	}
	if remote == "" {
		if cfg.Project.ID == "" {
			return "", invalid("%s has no remote and no committed project.id in %s; commit one (ADR-0046), attach does not write into the Project", top, configPath)
		}
		return domain.NewLocalProjectKey(cfg.Project.ID)
	}
	root, err := a.rootCommit(ctx, top)
	if err != nil {
		return "", err
	}
	return domain.NewProjectKey(remote, root)
}

// remoteURL returns the URL of origin, or of the only remote when there is no origin, or "" when
// the repository has no remote.
func (a Attacher) remoteURL(ctx context.Context, top string) (string, error) {
	out, err := a.run(ctx, top, "config", "--local", "--get-regexp", `^remote\..+\.url$`)
	if err != nil {
		var ee exitError
		if errors.As(err, &ee) && ee.code == 1 {
			return "", nil // no remote configured
		}
		return "", err
	}
	urls := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		k, v, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(k, "remote."), ".url")
		urls[name] = strings.TrimSpace(v)
	}
	if u, ok := urls["origin"]; ok {
		return u, nil
	}
	if len(urls) == 1 {
		for _, u := range urls {
			return u, nil
		}
	}
	return "", invalid("%s has %d remotes and none is named origin; Amaru will not pick one", top, len(urls))
}

// rootCommit returns the repository's root commit. A history with several roots (merged
// unrelated histories) uses the smallest object name, which every clone computes the same way.
func (a Attacher) rootCommit(ctx context.Context, top string) (string, error) {
	out, err := a.run(ctx, top, "rev-list", "--max-parents=0", "HEAD")
	if err != nil {
		return "", invalid("%s has no commit to derive a key from: %v", top, err)
	}
	roots := strings.Fields(string(out))
	if len(roots) == 0 {
		return "", invalid("%s has no root commit", top)
	}
	return slices.Min(roots), nil
}

// one runs git and returns its trimmed output.
func (a Attacher) one(ctx context.Context, dir string, args ...string) (string, error) {
	out, err := a.run(ctx, dir, args...)
	return strings.TrimSpace(string(out)), err
}

// exitError is git exiting non-zero.
type exitError struct {
	code   int
	stderr string
}

func (e exitError) Error() string { return fmt.Sprintf("git exited %d: %s", e.code, e.stderr) }

// hardened is put before every command: it turns off the configuration that can start a program
// while git reads a repository.
var hardened = []string{"-c", "core.fsmonitor=false", "-c", "core.pager=cat", "-c", "protocol.ext.allow=never"}

func (a Attacher) run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	git, err := a.Resolver.Find("git")
	if err != nil {
		return nil, fmt.Errorf("git is required to attach a Project: %w", err)
	}
	proc, err := a.Runner.Start(ctx, ports.ProcSpec{
		Path: git,
		Args: append(slices.Clone(hardened), args...),
		Dir:  dir,
		// No prompt, no optional locks (a read never takes the index lock), no system config.
		Env: []string{"GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "GIT_CONFIG_NOSYSTEM=1", "LC_ALL=C"},
	})
	if err != nil {
		return nil, err
	}
	errc := make(chan []byte, 1)
	go func() {
		b, _ := io.ReadAll(io.LimitReader(proc.Stderr(), maxOutput))
		_, _ = io.Copy(io.Discard, proc.Stderr())
		errc <- b
	}()
	out, rerr := io.ReadAll(io.LimitReader(proc.Stdout(), maxOutput))
	_, _ = io.Copy(io.Discard, proc.Stdout())
	stderr := <-errc
	code, werr := proc.Wait()
	switch {
	case werr != nil:
		return nil, werr
	case rerr != nil:
		return nil, rerr
	case code != 0:
		return nil, exitError{code: code, stderr: strings.TrimSpace(string(stderr))}
	}
	return out, nil
}

func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: attach: "+format, append([]any{domain.ErrInvalid}, a...)...)
}
