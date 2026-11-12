package domain

import "time"

const (
	CompactReplayWindow = 7 * 24 * time.Hour
	ServerReplayWindow  = 30 * 24 * time.Hour // also Cloud
)

// StreamEvent carries both the Project cursor and the original Execution sequence.
// It is an internal journal record; API event envelopes are composed at the API boundary.
type StreamEvent struct {
	ProjectID string
	Seq       int64
	Event     ExecutionEvent
}

// EventQuery always resumes with a Project cursor, including when filtering one Execution.
// The caller must authorize Project access before querying this internal port.
type EventQuery struct {
	ProjectID   string
	ExecutionID string
	After       int64
	Limit       int
}

func (q EventQuery) Validate() error {
	if q.ProjectID == "" || q.After < 0 || q.Limit < 1 || q.Limit > 1000 {
		return invalid("event query needs a Project, nonnegative cursor and limit 1..1000")
	}
	return nil
}

// ResyncRequired follows the existing ResyncRequired contract. ResumeFrom is a snapshot
// high-water mark: reload authoritative state before resuming there.
type ResyncRequired struct {
	Type       string `json:"type"`
	Reason     string `json:"reason"`
	ResumeFrom int64  `json:"resumeFrom"`
}

type EventPage struct {
	Events []StreamEvent
	Cursor int64
	Resync *ResyncRequired
}

// ReplayResync never silently skips an unavailable prefix of history.
func ReplayResync(after, last, unavailable, expired int64) *ResyncRequired {
	reason := ""
	switch {
	case after > last:
		reason = "cursor_ahead"
	case after < unavailable:
		reason = "history_unavailable"
	case after < expired:
		reason = "cursor_too_old"
	}
	if reason == "" {
		return nil
	}
	return &ResyncRequired{Type: "resync_required", Reason: reason, ResumeFrom: last}
}
