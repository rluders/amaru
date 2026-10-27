package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	testPkgID  = "01K2000000000000000000000" + "1"
	testItemID = "01K1000000000000000000000" + "1"
)

func validItem() WorkItem {
	return WorkItem{ID: testItemID, Key: "PROJ-1", PackageID: testPkgID, Title: "x", Status: WITodo,
		Owner: WorkOwner{Kind: OwnerHuman}}
}

func TestWorkItemValidate(t *testing.T) {
	if err := validItem().Validate(); err != nil {
		t.Fatalf("valid item rejected: %v", err)
	}
	cases := map[string]func(*WorkItem){
		"legacy id":                func(w *WorkItem) { w.ID = "WI-001" },
		"lowercase ulid":           func(w *WorkItem) { w.ID = "01k1000000000000000000000" + "1" },
		"no key":                   func(w *WorkItem) { w.Key = "" },
		"key without number":       func(w *WorkItem) { w.Key = "PROJ-" },
		"key number zero":          func(w *WorkItem) { w.Key = "PROJ-0" },
		"key leading zero":         func(w *WorkItem) { w.Key = "PROJ-01" },
		"key lowercase":            func(w *WorkItem) { w.Key = "proj-1" },
		"legacy package id":        func(w *WorkItem) { w.PackageID = "WP-001" },
		"empty title":              func(w *WorkItem) { w.Title = "  " },
		"bad status":               func(w *WorkItem) { w.Status = "running" },
		"agent owner":              func(w *WorkItem) { w.Owner.Kind = "agent" },
		"bad owner":                func(w *WorkItem) { w.Owner.Kind = "tool" },
		"reason while not blocked": func(w *WorkItem) { w.BlockedReason = "why" },
	}
	for name, mut := range cases {
		w := validItem()
		mut(&w)
		if err := w.Validate(); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: want ErrInvalid, got %v", name, err)
		}
	}
	b := validItem()
	b.Status, b.BlockedReason = WIBlocked, "waiting"
	if err := b.Validate(); err != nil {
		t.Errorf("blocked with reason: %v", err)
	}
}

func TestWorkPackageValidate(t *testing.T) {
	p := WorkPackage{ID: testPkgID, Title: "x", Status: WPPlanned}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, mut := range map[string]func(*WorkPackage){
		"bad status": func(p *WorkPackage) { p.Status = "in_progress" },
		"legacy id":  func(p *WorkPackage) { p.ID = "WP-001" },
		"no title":   func(p *WorkPackage) { p.Title = "" },
	} {
		q := p
		mut(&q)
		if !errors.Is(q.Validate(), ErrInvalid) {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestWorkKey(t *testing.T) {
	k, err := NewWorkKey("AMARU", 12)
	if err != nil || k != "AMARU-12" || k.Prefix() != "AMARU" {
		t.Fatalf("got %q %v", k, err)
	}
	for _, bad := range []struct {
		p string
		n int
	}{{"", 1}, {"a", 1}, {"A", 1}, {"AMARU", 0}, {"AMARU", -1}, {"AMARU-X", 1}, {"TOOLONGPREFIX", 1}} {
		if _, err := NewWorkKey(bad.p, bad.n); !errors.Is(err, ErrInvalid) {
			t.Errorf("%+v accepted", bad)
		}
	}
	if WorkKey("nope").Prefix() != "" {
		t.Error("invalid key has no prefix")
	}
}

// workItemEdges restates DOMAIN_MODEL §5.3 independently of the transition table: todo →
// in_progress → done, blocked reachable from todo and in_progress and back.
var workItemEdges = map[[2]WorkItemStatus]bool{
	{WITodo, WIInProgress}:    true,
	{WIInProgress, WIDone}:    true,
	{WITodo, WIBlocked}:       true,
	{WIInProgress, WIBlocked}: true,
	{WIBlocked, WITodo}:       true,
	{WIBlocked, WIInProgress}: true,
}

func TestWorkItemTransitionsMatchDomainModel(t *testing.T) {
	human := Actor{Kind: ActorHuman}
	now := time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)
	for _, from := range WorkItemStatuses {
		for _, to := range WorkItemStatuses {
			w := validItem()
			w.Status = from
			if from == WIBlocked {
				w.BlockedReason = "waiting"
			}
			err := w.SetStatus(now, to, human, "", nil)
			if want := workItemEdges[[2]WorkItemStatus{from, to}]; want != (err == nil) {
				t.Errorf("%s -> %s: documented=%v err=%v", from, to, want, err)
			}
			if err != nil && !errors.Is(err, ErrInvalidTransition) {
				t.Errorf("%s -> %s: want ErrInvalidTransition, got %v", from, to, err)
			}
			if err != nil && (w.Status != from || !w.UpdatedAt.IsZero()) {
				t.Errorf("%s -> %s: refused move changed the item", from, to)
			}
		}
	}
}

func TestSetStatusActors(t *testing.T) {
	now := time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)
	for name, by := range map[string]Actor{
		"agent":                {Kind: "agent"},
		"step":                 {Kind: "step"},
		"zero":                 {},
		"loop without policy":  {Kind: ActorDeliveryLoop},
		"human with authority": {Kind: "human_ish", Authority: "pol-1"},
	} {
		w := validItem()
		if err := w.SetStatus(now, WIInProgress, by, "", nil); !errors.Is(err, ErrNotPermitted) || w.Status != WITodo {
			t.Errorf("%s: err=%v status=%s", name, err, w.Status)
		}
	}
	w := validItem()
	if err := w.SetStatus(now, WIInProgress, Actor{Kind: ActorDeliveryLoop, Authority: "pol-1"}, "", nil); err != nil {
		t.Errorf("delivery loop with Policy authority refused: %v", err)
	}
}

func TestBlockedReasonFollowsStatus(t *testing.T) {
	now := time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)
	h := Actor{Kind: ActorHuman}
	w := validItem()
	if err := w.SetStatus(now, WIBlocked, h, "waiting for design", nil); err != nil || w.BlockedReason != "waiting for design" || !w.UpdatedAt.Equal(now) {
		t.Fatalf("block: %v %+v", err, w)
	}
	if err := w.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := w.SetStatus(now, WITodo, h, "stale reason", nil); err != nil || w.BlockedReason != "" {
		t.Fatalf("unblock must clear the reason: %v %q", err, w.BlockedReason)
	}
	long := make([]rune, MaxReason+1)
	for i := range long {
		long[i] = 'a'
	}
	if err := w.SetStatus(now, WIBlocked, h, string(long), nil); !errors.Is(err, ErrInvalid) || w.Status != WITodo {
		t.Errorf("long reason: %v", err)
	}
}

func TestDoneIsFinal(t *testing.T) {
	w := validItem()
	w.Status = WIDone
	for _, to := range WorkItemStatuses {
		if w.SetStatus(time.Now(), to, Actor{Kind: ActorHuman}, "", nil) == nil {
			t.Errorf("done -> %s allowed", to)
		}
	}
}

func TestProgressIsDerived(t *testing.T) {
	item := func(pkg string, s WorkItemStatus) WorkItem {
		w := validItem()
		w.PackageID, w.Status = pkg, s
		return w
	}
	other := "01K2000000000000000000000" + "2"
	items := []WorkItem{item(testPkgID, WIDone), item(testPkgID, WIDone), item(testPkgID, WIInProgress),
		item(testPkgID, WIBlocked), item(testPkgID, WITodo), item(other, WIDone)}
	p := ProgressOf(testPkgID, items)
	if p != (Progress{Total: 5, Todo: 1, InProgress: 1, Blocked: 1, Done: 2}) || p.Complete() {
		t.Fatalf("got %+v", p)
	}
	// Finishing the remaining items changes the answer without anything being stored.
	for i := range items {
		if items[i].PackageID == testPkgID {
			items[i].Status = WIDone
		}
	}
	if p := ProgressOf(testPkgID, items); !p.Complete() || p.Done != 5 {
		t.Errorf("got %+v", p)
	}
	if p := ProgressOf("01K2000000000000000000000"+"9", items); p.Total != 0 || p.Complete() {
		t.Errorf("empty package is not complete: %+v", p)
	}
}

// TestWorkItemContractFixturesAgree checks the Go types against the shared fixtures: every valid
// fixture decodes strictly and validates, every invalid one fails to decode or to validate.
func TestWorkItemContractFixturesAgree(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "..", "packages", "contracts", "fixtures", "api", "v1", "WorkItem")
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil || len(files) < 8 {
		t.Fatalf("fixtures not found under %s: %v (%d files)", dir, err, len(files))
	}
	for _, f := range files {
		name := filepath.Base(f)
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var w WorkItem
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.DisallowUnknownFields()
		err = dec.Decode(&w)
		if err == nil {
			err = w.Validate()
		}
		switch valid := len(name) > 5 && name[:5] == "valid"; {
		case valid && err != nil:
			t.Errorf("%s: %v", name, err)
		case !valid && err == nil:
			t.Errorf("%s: invalid fixture accepted", name)
		}
	}
}
