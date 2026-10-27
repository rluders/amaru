package migrations

import (
	"errors"
	"strings"
	"testing"
)

func TestPlan(t *testing.T) {
	dir := map[string]string{v1: "h1", v2: "h2"}
	cases := []struct {
		name    string
		dir     map[string]string
		applied []Applied
		want    Status
		err     error
	}{
		{"fresh database applies everything in order", dir, nil,
			Status{Latest: v2, Pending: []string{v1, v2}}, nil},
		{"partly migrated applies the rest", dir, []Applied{{v1, "h1"}},
			Status{Current: v1, Latest: v2, Applied: []string{v1}, Pending: []string{v2}}, nil},
		{"up to date has nothing pending", dir, []Applied{{v2, "h2"}, {v1, "h1"}},
			Status{Current: v2, Latest: v2, Applied: []string{v1, v2}}, nil},
		{"empty directory and fresh database", nil, nil, Status{}, nil},
		{"a recorded revision without a stored hash is not compared", dir, []Applied{{v1, ""}, {v2, ""}},
			Status{Current: v2, Latest: v2, Applied: []string{v1, v2}}, nil},
		{"database newer than the build", map[string]string{v1: "h1"}, []Applied{{v1, "h1"}, {v2, "h2"}},
			Status{Current: v2, Latest: v1, Applied: []string{v1, v2}}, ErrSchemaNewer},
		{"database migrated, build knows no migration", nil, []Applied{{v1, "h1"}},
			Status{Current: v1, Applied: []string{v1}}, ErrSchemaNewer},
		{"applied file was edited", dir, []Applied{{v1, "other"}},
			Status{Current: v1, Latest: v2}, ErrHistoryChanged},
		{"a file was added before the applied one", map[string]string{v1: "h1", v2: "h2"}, []Applied{{v2, "h2"}},
			Status{Current: v2, Latest: v2, Applied: []string{v2}, Pending: []string{v1}}, ErrHistoryChanged},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := plan(c.dir, c.applied)
			if !errors.Is(err, c.err) {
				t.Fatalf("err = %v, want %v", err, c.err)
			}
			if c.err != nil {
				return
			}
			if got.Current != c.want.Current || got.Latest != c.want.Latest ||
				strings.Join(got.Applied, ",") != strings.Join(c.want.Applied, ",") ||
				strings.Join(got.Pending, ",") != strings.Join(c.want.Pending, ",") {
				t.Fatalf("status = %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestDowngradeIsRefusedWithAMessage(t *testing.T) {
	_, err := plan(map[string]string{v1: "h1"}, []Applied{{v1, "h1"}, {v2, "h2"}})
	if err == nil {
		t.Fatal("no error")
	}
	for _, want := range []string{v2, v1, "upgrade Amaru", "downgrading is not supported"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message %q does not mention %q", err, want)
		}
	}
}

func TestUpToDate(t *testing.T) {
	if !(Status{}).UpToDate() || (Status{Pending: []string{v1}}).UpToDate() {
		t.Fatal("UpToDate wrong")
	}
}
