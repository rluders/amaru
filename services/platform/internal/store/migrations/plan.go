package migrations

import (
	"errors"
	"fmt"
	"sort"
)

var (
	// ErrSchemaNewer: the database records a version this build does not know.
	ErrSchemaNewer = errors.New("migrations: database schema is newer than this build")
	// ErrHistoryChanged: an applied migration file differs from what was applied.
	ErrHistoryChanged = errors.New("migrations: an applied migration file was changed")
	// ErrChecksum: the directory does not match atlas.sum.
	ErrChecksum = errors.New("migrations: directory does not match atlas.sum")
)

// Applied is one recorded, fully applied migration.
type Applied struct {
	Version string
	Hash    string
}

// Status is where a database stands relative to a directory.
type Status struct {
	Current string   // newest applied version, "" for a fresh database
	Latest  string   // newest version in the directory, "" when it has none
	Applied []string // applied versions, ascending
	Pending []string // versions to apply, ascending
}

// UpToDate reports whether nothing is pending.
func (s Status) UpToDate() bool { return len(s.Pending) == 0 }

// plan compares recorded migrations with the directory (version to file hash). It is pure.
// Versions are fixed-width timestamps, so string order is chronological order.
func plan(dir map[string]string, applied []Applied) (Status, error) {
	var st Status
	for v := range dir {
		if v > st.Latest {
			st.Latest = v
		}
	}
	recorded := map[string]bool{}
	var unknown []string
	for _, a := range applied {
		recorded[a.Version] = true
		st.Applied = append(st.Applied, a.Version)
		if a.Version > st.Current {
			st.Current = a.Version
		}
		h, known := dir[a.Version]
		switch {
		case !known:
			unknown = append(unknown, a.Version)
		case a.Hash != "" && a.Hash != h:
			return st, fmt.Errorf("%w: version %s", ErrHistoryChanged, a.Version)
		}
	}
	sort.Strings(st.Applied)
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return st, fmt.Errorf("%w: it records version %s but the newest this build knows is %s; upgrade Amaru, downgrading is not supported",
			ErrSchemaNewer, unknown[len(unknown)-1], orNone(st.Latest))
	}
	for v := range dir {
		if !recorded[v] {
			st.Pending = append(st.Pending, v)
		}
	}
	sort.Strings(st.Pending)
	// A pending version older than an applied one means a file was added out of order.
	if len(st.Pending) > 0 && st.Current != "" && st.Pending[0] < st.Current {
		return st, fmt.Errorf("%w: version %s is older than the applied %s", ErrHistoryChanged, st.Pending[0], st.Current)
	}
	return st, nil
}

func orNone(v string) string {
	if v == "" {
		return "none"
	}
	return v
}
