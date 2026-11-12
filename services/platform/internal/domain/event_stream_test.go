package domain

import (
	"encoding/json"
	"testing"
)

func TestReplayResync(t *testing.T) {
	for _, tc := range []struct {
		name                              string
		after, last, unavailable, expired int64
		reason                            string
	}{
		{"empty", 0, 0, 0, 0, ""},
		{"current", 8, 8, 1, 3, ""},
		{"retained boundary", 3, 8, 1, 3, ""},
		{"missed prefix", 2, 8, 1, 3, "cursor_too_old"},
		{"new client reloads", 0, 8, 0, 3, "cursor_too_old"},
		{"unknown history", 0, 8, 1, 3, "history_unavailable"},
		{"future cursor", 9, 8, 1, 3, "cursor_ahead"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := ReplayResync(tc.after, tc.last, tc.unavailable, tc.expired)
			if tc.reason == "" {
				if r != nil {
					t.Fatalf("unexpected resync: %+v", r)
				}
				return
			}
			if r == nil || r.Type != "resync_required" || r.Reason != tc.reason || r.ResumeFrom != tc.last {
				t.Fatalf("resync: %+v", r)
			}
		})
	}
	encoded, err := json.Marshal(ReplayResync(2, 8, 0, 3))
	if err != nil || string(encoded) != `{"type":"resync_required","reason":"cursor_too_old","resumeFrom":8}` {
		t.Fatalf("contract shape: %s %v", encoded, err)
	}
}
