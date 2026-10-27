package domain

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// MaxProjectName bounds a Project display name.
const MaxProjectName = 120

var (
	commitRe = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)
	// scpRe matches the scp-like Git remote form user@host:path, which has no scheme.
	scpRe = regexp.MustCompile(`^(?:[^@/:\s]+@)?([^@/:\s]+):(.+)$`)
)

// ProjectKey is a Project's identity (ADR-0046). It never contains a filesystem path: two clones
// of one repository, at any location, derive the same key.
//
// Two forms exist:
//   - "<normalized remote>#<root commit sha>" for a repository with a remote;
//   - "id:<ULID>" for a repository without one, where the ULID is the committed Project id.
type ProjectKey string

// NewProjectKey derives the key of a repository with a remote. The remote is normalized as
// ADR-0046 states: scheme and host lowercased; credentials, a ".git" suffix and a trailing slash
// removed. An scp-like remote (user@host:path) is read as ssh://host/path. The root commit is a
// full lowercase hex SHA-1 or SHA-256 object name.
//
// Different transports of one repository (https and ssh) yield different keys: ADR-0046 does not
// say they are equal, and the user links such aliases explicitly.
func NewProjectKey(remote, rootCommit string) (ProjectKey, error) {
	norm, err := NormalizeRemote(remote)
	if err != nil {
		return "", err
	}
	if !commitRe.MatchString(rootCommit) {
		return "", fmt.Errorf("%w: root commit must be a full lowercase hex object name", ErrInvalid)
	}
	return ProjectKey(norm + "#" + rootCommit), nil
}

// NewLocalProjectKey derives the key of a repository with no remote from its committed Project id.
func NewLocalProjectKey(id string) (ProjectKey, error) {
	if !ulidRe.MatchString(id) {
		return "", fmt.Errorf("%w: project id must be a ULID", ErrInvalid)
	}
	return ProjectKey("id:" + id), nil
}

// NormalizeRemote returns the normalized form of a Git remote URL (see NewProjectKey).
func NormalizeRemote(remote string) (string, error) {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return "", fmt.Errorf("%w: remote is empty", ErrInvalid)
	}
	if !strings.Contains(remote, "://") {
		m := scpRe.FindStringSubmatch(remote)
		if m == nil {
			return "", fmt.Errorf("%w: remote %q is not a URL", ErrInvalid, remote)
		}
		remote = "ssh://" + m[1] + "/" + strings.TrimPrefix(m[2], "/")
	}
	u, err := url.Parse(remote)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("%w: remote %q is not a URL", ErrInvalid, remote)
	}
	path := strings.TrimSuffix(strings.TrimRight(u.EscapedPath(), "/"), ".git")
	path = strings.TrimRight(path, "/")
	if path == "" {
		return "", fmt.Errorf("%w: remote %q has no repository path", ErrInvalid, remote)
	}
	return strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host) + path, nil
}

// Valid reports whether the key has one of the two forms.
func (k ProjectKey) Valid() bool {
	s := string(k)
	if id, ok := strings.CutPrefix(s, "id:"); ok {
		return ulidRe.MatchString(id)
	}
	i := strings.LastIndexByte(s, '#')
	if i < 0 || !commitRe.MatchString(s[i+1:]) {
		return false
	}
	norm, err := NormalizeRemote(s[:i])
	return err == nil && norm == s[:i]
}

// Project is a repository the Platform knows. It holds identity and display data only. Clone
// location, trust, settings and every kind of operational state are separate records keyed by
// ProjectKey (DATA_ARCHITECTURE; ADR-0043): nothing here is written into the Project tree, and
// no filesystem path is part of the identity.
type Project struct {
	Key       ProjectKey `json:"key"`
	Name      string     `json:"name"`
	CreatedAt time.Time  `json:"createdAt"`
}

// Validate reports why the Project cannot be stored, or nil. Errors match ErrInvalid.
func (p Project) Validate() error {
	if !p.Key.Valid() {
		return fmt.Errorf("%w: project key %q is not valid", ErrInvalid, p.Key)
	}
	name := strings.TrimSpace(p.Name)
	if name == "" || utf8.RuneCountInString(name) > MaxProjectName || name != p.Name {
		return fmt.Errorf("%w: project name must be 1 to %d characters without surrounding space", ErrInvalid, MaxProjectName)
	}
	if p.CreatedAt.IsZero() {
		return fmt.Errorf("%w: project createdAt is required", ErrInvalid)
	}
	return nil
}
