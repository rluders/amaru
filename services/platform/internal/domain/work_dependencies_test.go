package domain

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// depItem returns a todo item numbered n (1..9) in the test package, depending on deps.
func depItem(n int, deps ...int) WorkItem {
	w := validItem()
	w.ID = fmt.Sprintf("01K1%022d", n)
	w.Key = WorkKey(fmt.Sprintf("PROJ-%d", n))
	for _, d := range deps {
		w.DependsOn = append(w.DependsOn, fmt.Sprintf("01K1%022d", d))
	}
	return w
}

func withStatus(w WorkItem, s WorkItemStatus) WorkItem {
	w.Status = s
	return w
}

func TestDependsOnValidate(t *testing.T) {
	self := depItem(1)
	self.DependsOn = []string{self.ID}
	tooMany := depItem(1)
	for n := 0; n <= MaxDependsOn; n++ {
		tooMany.DependsOn = append(tooMany.DependsOn, fmt.Sprintf("01K3%022d", n))
	}
	cases := map[string]struct {
		w  WorkItem
		ok bool
	}{
		"none":            {depItem(1), true},
		"two":             {depItem(1, 2, 3), true},
		"self":            {self, false},
		"duplicate":       {depItem(1, 2, 2), false},
		"legacy id":       {func() WorkItem { w := depItem(1); w.DependsOn = []string{"WI-002"}; return w }(), false},
		"too many":        {tooMany, false},
		"empty id":        {func() WorkItem { w := depItem(1); w.DependsOn = []string{""}; return w }(), false},
		"exactly the max": {func() WorkItem { w := tooMany; w.DependsOn = w.DependsOn[:MaxDependsOn]; return w }(), true},
	}
	for name, c := range cases {
		err := c.w.Validate()
		if c.ok && err != nil {
			t.Errorf("%s: %v", name, err)
		}
		if !c.ok && !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: want ErrInvalid, got %v", name, err)
		}
	}
}

func TestDependencyCycle(t *testing.T) {
	cases := map[string]struct {
		items []WorkItem
		cycle []int // item numbers on the cycle, nil when acyclic
	}{
		"empty":          {nil, nil},
		"no edges":       {[]WorkItem{depItem(1), depItem(2)}, nil},
		"chain":          {[]WorkItem{depItem(1, 2), depItem(2, 3), depItem(3)}, nil},
		"diamond":        {[]WorkItem{depItem(1, 2, 3), depItem(2, 4), depItem(3, 4), depItem(4)}, nil},
		"self loop":      {[]WorkItem{func() WorkItem { w := depItem(1); w.DependsOn = []string{w.ID}; return w }()}, []int{1}},
		"two cycle":      {[]WorkItem{depItem(1, 2), depItem(2, 1)}, []int{1, 2}},
		"three cycle":    {[]WorkItem{depItem(1, 2), depItem(2, 3), depItem(3, 1)}, []int{1, 2, 3}},
		"cycle off tail": {[]WorkItem{depItem(1, 2), depItem(2, 3), depItem(3, 4), depItem(4, 2)}, []int{2, 3, 4}},
		"unknown id":     {[]WorkItem{depItem(1, 9)}, nil},
	}
	for name, c := range cases {
		got := DependencyCycle(c.items)
		var want []string
		for _, n := range c.cycle {
			want = append(want, depItem(n).ID)
		}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s: cycle %v, want %v", name, got, want)
		}
	}
}

func TestValidateDependencies(t *testing.T) {
	if err := ValidateDependencies([]WorkItem{depItem(1, 2), depItem(2)}); err != nil {
		t.Fatalf("acyclic graph: %v", err)
	}
	err := ValidateDependencies([]WorkItem{depItem(1, 2), depItem(2, 3), depItem(3, 1)})
	if !errors.Is(err, ErrDependencyCycle) || !errors.Is(err, ErrInvalid) {
		t.Fatalf("cycle: want ErrDependencyCycle and ErrInvalid, got %v", err)
	}
	if !strings.Contains(err.Error(), "PROJ-1 -> PROJ-2 -> PROJ-3") {
		t.Errorf("cycle error should name the items by key: %v", err)
	}
	if err := ValidateDependencies([]WorkItem{depItem(1, 9)}); !errors.Is(err, ErrInvalid) || errors.Is(err, ErrDependencyCycle) {
		t.Errorf("missing dependency: want ErrInvalid only, got %v", err)
	}
}

func TestSetDependsOn(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	a, b, c := depItem(1), depItem(2, 3), depItem(3)
	items := []WorkItem{a, b, c}

	if err := a.SetDependsOn(now, []string{b.ID}, items); err != nil {
		t.Fatalf("valid edit: %v", err)
	}
	if len(a.DependsOn) != 1 || a.DependsOn[0] != b.ID || !a.UpdatedAt.Equal(now) {
		t.Errorf("edit not applied: %+v", a)
	}

	// c -> a would close a -> b -> c -> a. The stale copy of c in items is replaced by c itself.
	items = []WorkItem{a, b, c}
	before := c
	err := c.SetDependsOn(now, []string{a.ID}, items)
	if !errors.Is(err, ErrDependencyCycle) {
		t.Fatalf("cycle: want ErrDependencyCycle, got %v", err)
	}
	if len(c.DependsOn) != len(before.DependsOn) || !c.UpdatedAt.Equal(before.UpdatedAt) {
		t.Errorf("rejected edit changed the item: %+v", c)
	}

	// Rejected for the other reasons too, each leaving the item unchanged.
	for name, deps := range map[string][]string{
		"unknown":   {depItem(9).ID},
		"self":      {c.ID},
		"duplicate": {a.ID, a.ID},
	} {
		d := c
		if err := d.SetDependsOn(now, deps, items); !errors.Is(err, ErrInvalid) || len(d.DependsOn) != 0 {
			t.Errorf("%s: err %v, deps %v", name, err, d.DependsOn)
		}
	}

	// Clearing is allowed.
	if err := b.SetDependsOn(now, nil, items); err != nil || len(b.DependsOn) != 0 {
		t.Errorf("clear: %v %v", err, b.DependsOn)
	}
}

func TestDependenciesMayCrossWorkPackages(t *testing.T) {
	other := depItem(2)
	other.PackageID = "01K2000000000000000000000" + "2"
	a := depItem(1)
	if err := a.SetDependsOn(time.Now(), []string{other.ID}, []WorkItem{a, other}); err != nil {
		t.Errorf("cross-package dependency within a Project must be allowed: %v", err)
	}
}

func TestStartWaitsForDependencies(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	human := Actor{Kind: ActorHuman}
	loop := Actor{Kind: ActorDeliveryLoop, Authority: "pol-1"}

	for _, st := range []WorkItemStatus{WITodo, WIInProgress, WIBlocked} {
		dep := withStatus(depItem(2), st)
		w := depItem(1, 2)
		err := w.SetStatus(now, WIInProgress, human, "", []WorkItem{w, dep})
		wantErr := st != WIDone
		if wantErr != errors.Is(err, ErrDependenciesNotDone) {
			t.Errorf("dependency %s: err %v", st, err)
		}
		if wantErr && (w.Status != WITodo || !strings.Contains(err.Error(), "PROJ-2 ("+string(st)+")")) {
			t.Errorf("dependency %s: item moved or error does not name it: %v (status %s)", st, err, w.Status)
		}
	}

	t.Run("done dependency lets it start", func(t *testing.T) {
		w := depItem(1, 2)
		if err := w.SetStatus(now, WIInProgress, human, "", []WorkItem{withStatus(depItem(2), WIDone)}); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("the delivery loop is bound too", func(t *testing.T) {
		w := depItem(1, 2)
		if err := w.SetStatus(now, WIInProgress, loop, "", []WorkItem{depItem(2)}); !errors.Is(err, ErrDependenciesNotDone) {
			t.Errorf("got %v", err)
		}
	})
	t.Run("a missing dependency counts as not done", func(t *testing.T) {
		w := depItem(1, 2)
		err := w.SetStatus(now, WIInProgress, human, "", nil)
		if !errors.Is(err, ErrDependenciesNotDone) || !strings.Contains(err.Error(), "not found") {
			t.Errorf("got %v", err)
		}
	})
	t.Run("blocked item resuming is checked too", func(t *testing.T) {
		w := withStatus(depItem(1, 2), WIBlocked)
		w.BlockedReason = "x"
		if err := w.SetStatus(now, WIInProgress, human, "", []WorkItem{depItem(2)}); !errors.Is(err, ErrDependenciesNotDone) {
			t.Errorf("got %v", err)
		}
	})
	t.Run("every unmet dependency is named, done ones are not", func(t *testing.T) {
		w := depItem(1, 2, 3, 4)
		err := w.SetStatus(now, WIInProgress, human, "", []WorkItem{
			depItem(2), withStatus(depItem(3), WIDone), withStatus(depItem(4), WIBlocked)})
		if err == nil || !strings.Contains(err.Error(), "PROJ-2 (todo)") || !strings.Contains(err.Error(), "PROJ-4 (blocked)") || strings.Contains(err.Error(), "PROJ-3") {
			t.Errorf("got %v", err)
		}
	})
	t.Run("permission is checked before dependencies", func(t *testing.T) {
		w := depItem(1, 2)
		if err := w.SetStatus(now, WIInProgress, Actor{Kind: "agent"}, "", []WorkItem{depItem(2)}); !errors.Is(err, ErrNotPermitted) {
			t.Errorf("got %v", err)
		}
	})
	t.Run("an item without dependencies is unaffected", func(t *testing.T) {
		w := depItem(1)
		if err := w.SetStatus(now, WIInProgress, human, "", nil); err != nil {
			t.Fatal(err)
		}
	})
}

func TestBlockedStateExplainsTheDependency(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	human := Actor{Kind: ActorHuman}
	items := []WorkItem{depItem(2), withStatus(depItem(3), WIInProgress), withStatus(depItem(4), WIDone)}

	w := depItem(1, 2, 3, 4)
	if err := w.SetStatus(now, WIBlocked, human, "", items); err != nil {
		t.Fatal(err)
	}
	if w.BlockedReason != "waiting on PROJ-2 (todo), PROJ-3 (in_progress)" {
		t.Errorf("reason %q", w.BlockedReason)
	}
	if err := w.Validate(); err != nil {
		t.Errorf("explained block must validate: %v", err)
	}

	// A reason the human gave is kept, not overwritten.
	h := depItem(1, 2)
	if err := h.SetStatus(now, WIBlocked, human, "needs design first", items); err != nil || h.BlockedReason != "needs design first" {
		t.Errorf("explicit reason: %v %q", err, h.BlockedReason)
	}
	// Blocking with nothing to explain stays reasonless, as before.
	n := depItem(1)
	if err := n.SetStatus(now, WIBlocked, human, "", items); err != nil || n.BlockedReason != "" {
		t.Errorf("no deps: %v %q", err, n.BlockedReason)
	}
	// Once the dependency is done the item can start and the reason is cleared.
	done := []WorkItem{withStatus(depItem(2), WIDone), withStatus(depItem(3), WIDone), withStatus(depItem(4), WIDone)}
	if err := w.SetStatus(now, WIInProgress, human, "", done); err != nil || w.BlockedReason != "" {
		t.Errorf("resume: %v %q", err, w.BlockedReason)
	}
}

func TestUnmetDependenciesOrder(t *testing.T) {
	w := depItem(1, 4, 2, 3)
	got := w.UnmetDependencies([]WorkItem{depItem(2), depItem(3), depItem(4)})
	if len(got) != 3 || got[0].Key != "PROJ-4" || got[1].Key != "PROJ-2" || got[2].Key != "PROJ-3" {
		t.Errorf("want DependsOn order, got %+v", got)
	}
}
