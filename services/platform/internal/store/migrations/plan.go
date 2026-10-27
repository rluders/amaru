package migrations

import (
	"errors"
	"fmt"
	"sort"
)

var (
	// ErrSchemaNewer: the database records a version this build does not know.
	ErrSchemaNewer = errors.New("migrations: database schema is newer than this build")
	// ErrOutOfOrder: a migration file is older than one already applied.
	ErrOutOfOrder = errors.New("migrations: a migration older than an applied one is pending")
)

// Status is where a database stands relative to a directory.
type Status struct {
	Current string   // newest applied version, "" for a fresh database
	Latest  string   // newest version in the directory, "" when it has none
	Applied []string // applied versions, ascending
	Pending []string // versions to apply, ascending
}

// UpToDate reports whether nothing is pending.
func (s Status) UpToDate() bool { return len(s.Pending) == 0 }

// plan compares the directory's versions with the recorded ones. It is pure. Versions are
// fixed-width timestamps, so string order is chronological order.
func plan(dir, applied []string) (Status, error) {
	var st Status
	known := map[string]bool{}
	for _, v := range dir {
		known[v] = true
		if v > st.Latest {
			st.Latest = v
		}
	}
	recorded := map[string]bool{}
	var unknown []string
	for _, v := range applied {
		recorded[v] = true
		st.Applied = append(st.Applied, v)
		if v > st.Current {
			st.Current = v
		}
		if !known[v] {
			unknown = append(unknown, v)
		}
	}
	sort.Strings(st.Applied)
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return st, fmt.Errorf("%w: it records version %s but the newest this build knows is %s; upgrade Amaru, downgrading is not supported",
			ErrSchemaNewer, unknown[len(unknown)-1], orNone(st.Latest))
	}
	for _, v := range dir {
		if !recorded[v] {
			st.Pending = append(st.Pending, v)
		}
	}
	sort.Strings(st.Pending)
	if len(st.Pending) > 0 && st.Current != "" && st.Pending[0] < st.Current {
		return st, fmt.Errorf("%w: version %s is older than the applied %s", ErrOutOfOrder, st.Pending[0], st.Current)
	}
	return st, nil
}

func orNone(v string) string {
	if v == "" {
		return "none"
	}
	return v
}
