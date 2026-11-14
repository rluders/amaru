package domain

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

// WRK-003/F-7: a stale form must not undo a newer Worktree permission
// decision. These are domain editors, not authenticated HTTP clients.
func TestAgentStaleEditPreservesWorktreeDecision(t *testing.T) {
	for _, writable := range []bool{false, true} {
		name := "revoke"
		if writable {
			name = "enable"
		}
		for _, mode := range []string{"patch", "replacement"} {
			t.Run(name+"/"+mode, func(t *testing.T) {
				initial := okAgent()
				initial.WorktreeWrite = !writable
				if initial.WorktreeWrite {
					initial.WorktreeCommands = []string{"go test"}
				}
				h, err := NewAgentHistory(initial, t0)
				if err != nil {
					t.Fatal(err)
				}
				editor := h.Head()
				latest := h.Head().Definition
				latest.WorktreeWrite, latest.WorktreeCommands = writable, nil
				if writable {
					latest.WorktreeCommands = []string{"go vet"}
				}
				committed, changed, err := h.Replace(editor.Revision, latest, t0.Add(time.Minute))
				if err != nil || !changed {
					t.Fatalf("permission edit: changed=%v err=%v", changed, err)
				}
				before := h.Versions()
				role := "Reviewer"
				stale := editor.Definition
				stale.Role = role
				var result AgentVersion
				if mode == "replacement" {
					result, changed, err = h.Replace(editor.Revision, stale, t0.Add(2*time.Minute))
				} else {
					result, changed, err = h.Update(editor.Revision, AgentPatch{Role: &role, WorktreeWrite: &stale.WorktreeWrite, WorktreeCommands: &stale.WorktreeCommands}, t0.Add(2*time.Minute))
				}
				if !errors.Is(err, ErrConflict) || changed || !reflect.DeepEqual(result, AgentVersion{}) {
					t.Fatalf("stale edit: result=%+v changed=%v err=%v", result, changed, err)
				}
				if !reflect.DeepEqual(h.Versions(), before) {
					t.Fatal("conflict changed recorded history")
				}
				if editor.Definition.Role != initial.Role || editor.Definition.WorktreeWrite != initial.WorktreeWrite || !reflect.DeepEqual(editor.Definition.WorktreeCommands, initial.WorktreeCommands) {
					t.Fatal("another editor mutated the open edit")
				}
				// Refetch and reconcile only the intended Role change. Updating
				// the revision on an obsolete full form would lose that intent.
				refreshed := h.Head()
				saved, changed, err := h.Update(refreshed.Revision, AgentPatch{Role: &role}, t0.Add(3*time.Minute))
				if err != nil || !changed || saved.Revision != committed.Revision+1 {
					t.Fatalf("reconcile: %+v %v %v", saved, changed, err)
				}
				if saved.Definition.Role != role || saved.Definition.WorktreeWrite != writable || !reflect.DeepEqual(saved.Definition.WorktreeCommands, latest.WorktreeCommands) {
					t.Fatalf("reconciled edit lost intent: %+v", saved.Definition)
				}
				if !reflect.DeepEqual(h.Versions()[:len(before)], before) {
					t.Fatal("reconcile rewrote prior revisions")
				}
				before = h.Versions()
				if _, changed, err = h.Replace(saved.Revision, saved.Definition, t0.Add(4*time.Minute)); err != nil || changed {
					t.Fatalf("duplicate content: %v %v", changed, err)
				}
				if _, changed, err = h.Replace(editor.Revision, saved.Definition, t0.Add(5*time.Minute)); !errors.Is(err, ErrConflict) || changed {
					t.Fatalf("stale matching content: %v %v", changed, err)
				}
				if !reflect.DeepEqual(h.Versions(), before) {
					t.Fatal("duplicate or stale content changed history")
				}
			})
		}
	}
}

func TestAgentInvalidReplacementPreservesOpenEdit(t *testing.T) {
	h, err := NewAgentHistory(okAgent(), t0)
	if err != nil {
		t.Fatal(err)
	}
	open := h.Head()
	before := h.Versions()
	invalid := open.Definition
	invalid.WorktreeWrite, invalid.WorktreeCommands = false, []string{"go test"}
	if _, changed, err := h.Replace(open.Revision, invalid, t0.Add(time.Minute)); !errors.Is(err, ErrInvalid) || changed {
		t.Fatalf("invalid replacement: %v %v", changed, err)
	}
	if !reflect.DeepEqual(h.Versions(), before) {
		t.Fatal("invalid replacement changed history")
	}
	role := "Reviewer"
	if _, changed, err := h.Update(open.Revision, AgentPatch{Role: &role}, t0.Add(2*time.Minute)); err != nil || !changed {
		t.Fatalf("valid open edit: %v %v", changed, err)
	}
}
