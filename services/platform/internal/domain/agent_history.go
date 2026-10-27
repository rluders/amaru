package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// AgentVersion is one immutable revision of an Agent definition. A Step snapshots the Agent it ran
// with (FEAT-AGT-004), and Revision and Hash let a reader tie that snapshot to this record.
type AgentVersion struct {
	Revision int    `json:"revision"` // 1, 2, 3 ... with no gaps
	Hash     string `json:"hash"`     // see HashAgent
	// Deleted marks a tombstone: the Agent was deleted at RecordedAt. Definition is the last one it had.
	Deleted    bool      `json:"deleted,omitempty"`
	RecordedAt time.Time `json:"recordedAt"`
	Definition Agent     `json:"definition"`
}

// HashAgent is the content hash of an Agent definition: SHA-256 over its canonical JSON, without
// the timestamps. Two definitions with the same hash are the same definition, so an edit is
// detected by comparing hashes and a save that changes nothing records nothing.
func HashAgent(a Agent) string {
	a = a.clone()
	a.CreatedAt, a.UpdatedAt = time.Time{}, time.Time{}
	b, err := json.Marshal(struct {
		Agent
		CreatedAt, UpdatedAt struct{} `json:"-"`
	}{Agent: a})
	if err != nil { // unreachable: Agent holds only strings, bools and lists of them
		panic(fmt.Sprintf("hash agent: %v", err))
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// AgentHistory is the append-only record of one Agent id: every revision it ever had and, once
// deleted, when. Deleting never removes or alters a revision, so a run that references one stays
// reconstructable. The id is immutable. Deleting appends a tombstone revision, and creating the Agent
// again appends a further revision to the same history, so a (id, revision) pair always names one
// definition and a Step snapshot is never ambiguous.
type AgentHistory struct {
	versions []AgentVersion
}

// NewAgentHistory records the first revision of a validated Agent. CreatedAt and UpdatedAt are set
// to now.
func NewAgentHistory(a Agent, now time.Time) (*AgentHistory, error) {
	a = a.clone()
	a.CreatedAt, a.UpdatedAt = now, now
	if err := a.Validate(); err != nil {
		return nil, err
	}
	return &AgentHistory{versions: []AgentVersion{{Revision: 1, Hash: HashAgent(a), RecordedAt: now, Definition: a}}}, nil
}

// ID is the Agent id.
func (h *AgentHistory) ID() string { return h.versions[0].Definition.ID }

// Head is the latest version.
func (h *AgentHistory) Head() AgentVersion { return h.at(len(h.versions) - 1) }

// Version returns the version with the given revision.
func (h *AgentHistory) Version(rev int) (AgentVersion, bool) {
	if rev < 1 || rev > len(h.versions) {
		return AgentVersion{}, false
	}
	return h.at(rev - 1), true
}

// Versions returns every revision, oldest first, as copies.
func (h *AgentHistory) Versions() []AgentVersion {
	out := make([]AgentVersion, len(h.versions))
	for i := range h.versions {
		out[i] = h.at(i)
	}
	return out
}

// Deleted reports whether the Agent is deleted and when.
func (h *AgentHistory) Deleted() (time.Time, bool) {
	if head := h.Head(); head.Deleted {
		return head.RecordedAt, true
	}
	return time.Time{}, false
}

func (h *AgentHistory) at(i int) AgentVersion {
	v := h.versions[i]
	v.Definition = v.Definition.clone()
	return v
}

func (h *AgentHistory) guard(expected int) error {
	if _, gone := h.Deleted(); gone {
		return fmt.Errorf("%w: agent %s", ErrDeleted, h.ID())
	}
	if head := len(h.versions); expected != head {
		return fmt.Errorf("%w: agent %s is at revision %d, edit was made against %d", ErrConflict, h.ID(), head, expected)
	}
	return nil
}

// Update applies a patch made against revision expected. It returns the new head and whether the
// edit changed anything. A stale expected revision is ErrConflict, so two editors cannot silently
// overwrite each other (the lost-update guard). A patch that leaves the content hash unchanged
// records no revision.
func (h *AgentHistory) Update(expected int, p AgentPatch, now time.Time) (AgentVersion, bool, error) {
	if err := h.guard(expected); err != nil {
		return AgentVersion{}, false, err
	}
	if p.Empty() {
		return AgentVersion{}, false, invalid("patch sets no field")
	}
	return h.Replace(expected, p.Apply(h.Head().Definition), now)
}

// Replace records next as the new definition, made against revision expected, for an edit that
// arrives as a whole definition (for example a changed file). The id, CreatedAt and UpdatedAt are
// not taken from next: the id is immutable, CreatedAt is kept and UpdatedAt becomes now.
func (h *AgentHistory) Replace(expected int, next Agent, now time.Time) (AgentVersion, bool, error) {
	if err := h.guard(expected); err != nil {
		return AgentVersion{}, false, err
	}
	head := h.Head()
	if next.ID != head.Definition.ID {
		return AgentVersion{}, false, invalid("agent id is immutable")
	}
	next = next.clone()
	next.CreatedAt, next.UpdatedAt = head.Definition.CreatedAt, now
	if err := next.Validate(); err != nil {
		return AgentVersion{}, false, err
	}
	hash := HashAgent(next)
	if hash == head.Hash {
		return head, false, nil
	}
	v := AgentVersion{Revision: len(h.versions) + 1, Hash: hash, RecordedAt: now, Definition: next}
	h.versions = append(h.versions, v)
	return h.Head(), true, nil
}

// Delete appends a tombstone revision, made against revision expected. No earlier revision is
// removed or changed; the Agent just accepts no edits until it is created again.
func (h *AgentHistory) Delete(expected int, now time.Time) error {
	if err := h.guard(expected); err != nil {
		return err
	}
	head := h.Head()
	head.Revision, head.RecordedAt, head.Deleted = len(h.versions)+1, now, true
	h.versions = append(h.versions, head)
	return nil
}

// Recreate brings a deleted Agent back under the same id, as a new revision of the same history.
// CreatedAt and UpdatedAt are set to now. The Agent must be deleted, and expected is the tombstone's
// revision.
func (h *AgentHistory) Recreate(expected int, a Agent, now time.Time) (AgentVersion, error) {
	if head := len(h.versions); expected != head {
		return AgentVersion{}, fmt.Errorf("%w: agent %s is at revision %d, edit was made against %d", ErrConflict, h.ID(), head, expected)
	}
	if _, gone := h.Deleted(); !gone {
		return AgentVersion{}, invalid("agent %s is not deleted", h.ID())
	}
	if a.ID != h.ID() {
		return AgentVersion{}, invalid("agent id is immutable")
	}
	a = a.clone()
	a.CreatedAt, a.UpdatedAt = now, now
	if err := a.Validate(); err != nil {
		return AgentVersion{}, err
	}
	h.versions = append(h.versions, AgentVersion{Revision: len(h.versions) + 1, Hash: HashAgent(a), RecordedAt: now, Definition: a})
	return h.Head(), nil
}
