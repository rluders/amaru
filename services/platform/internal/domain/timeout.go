package domain

import (
	"fmt"
	"time"
)

// CodeTimeout is the error code of a Step that ran past its wall-clock timeout (FEAT-WFL-004).
// The Step ends in StepTimedOut (ADR-0044), is never re-run automatically, and a person resumes
// or cancels it.
const CodeTimeout = "timeout"

// Timeouts are the wall-clock limits of one Step, from RESILIENCE.md: a default per kind of Step
// and the largest value an override may take (the Organization maximum). Approval waits have no
// timeout and are not covered here.
type Timeouts struct {
	Agent, Tool, Max time.Duration
}

// DefaultTimeouts are the documented defaults: agent Steps 60 minutes, tool and check Steps 30
// minutes, Organization maximum 8 hours.
var DefaultTimeouts = Timeouts{Agent: 60 * time.Minute, Tool: 30 * time.Minute, Max: 8 * time.Hour}

// Effective returns the timeout a Step runs under: the override in seconds when it is set, else
// the default for its kind. A Step always has one.
func (t Timeouts) Effective(agent bool, overrideSeconds int) time.Duration {
	switch {
	case overrideSeconds > 0:
		return time.Duration(overrideSeconds) * time.Second
	case agent:
		return t.Agent
	}
	return t.Tool
}

// Check refuses an override that is negative or above the Organization maximum. Zero means "use
// the default" and passes.
func (t Timeouts) Check(overrideSeconds int) error {
	switch {
	case overrideSeconds < 0:
		return invalid("timeout must not be negative")
	case time.Duration(overrideSeconds)*time.Second > t.Max:
		return invalid("timeout of %ds is above the organization maximum of %s", overrideSeconds, t.Max)
	}
	return nil
}

// Validate checks the limits themselves: every one positive, the defaults within the maximum.
func (t Timeouts) Validate() error {
	if t.Agent <= 0 || t.Tool <= 0 || t.Max <= 0 {
		return fmt.Errorf("%w: timeouts must be positive", ErrInvalid)
	}
	if t.Agent > t.Max || t.Tool > t.Max {
		return fmt.Errorf("%w: a default timeout is above the maximum %s", ErrInvalid, t.Max)
	}
	return nil
}
