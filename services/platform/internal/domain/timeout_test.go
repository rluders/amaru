package domain

import (
	"errors"
	"testing"
	"time"
)

func TestTimeoutsEffective(t *testing.T) {
	d := DefaultTimeouts
	if d.Agent != 60*time.Minute || d.Tool != 30*time.Minute || d.Max != 8*time.Hour {
		t.Fatalf("defaults %+v differ from RESILIENCE.md", d)
	}
	for _, tc := range []struct {
		agent    bool
		override int
		want     time.Duration
	}{
		{true, 0, 60 * time.Minute}, {false, 0, 30 * time.Minute},
		{true, 90, 90 * time.Second}, {false, 3600, time.Hour},
	} {
		if got := d.Effective(tc.agent, tc.override); got != tc.want {
			t.Errorf("Effective(%v, %d) = %s, want %s", tc.agent, tc.override, got, tc.want)
		}
	}
}

func TestTimeoutsCheck(t *testing.T) {
	d := DefaultTimeouts
	for _, ok := range []int{0, 1, 8 * 3600} {
		if err := d.Check(ok); err != nil {
			t.Errorf("Check(%d): %v", ok, err)
		}
	}
	for _, bad := range []int{-1, 8*3600 + 1} {
		if err := d.Check(bad); !errors.Is(err, ErrInvalid) {
			t.Errorf("Check(%d) = %v, want ErrInvalid", bad, err)
		}
	}
	if err := (Timeouts{Agent: 2 * time.Hour, Tool: time.Hour, Max: time.Hour}).Validate(); !errors.Is(err, ErrInvalid) {
		t.Errorf("default above maximum: %v", err)
	}
	if err := (Timeouts{}).Validate(); !errors.Is(err, ErrInvalid) {
		t.Errorf("zero limits: %v", err)
	}
	if err := DefaultTimeouts.Validate(); err != nil {
		t.Error(err)
	}
}
