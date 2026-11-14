package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"time"
)

// Usage measurements mirror the existing Worker protocol, including presence.
// No value is inferred when a measurement or report is missing (ADR-0033).
type UsageSource int

const (
	UsageReported  UsageSource = 1
	UsageEstimated UsageSource = 2
	UsageUnknown   UsageSource = 3
)

type IntMeasurement struct {
	Source UsageSource `proto:"source"`
	Value  *uint64     `proto:"value"`
}
type DoubleMeasurement struct {
	Source UsageSource `proto:"source"`
	Value  *float64    `proto:"value"`
}
type Cost struct {
	Source            UsageSource `proto:"source"`
	Amount            string      `proto:"amount"`
	Currency          string      `proto:"currency"`
	PriceTableVersion string      `proto:"price_table_version"`
}
type UsageReport struct {
	Provider         string             `proto:"provider"`
	Model            string             `proto:"model"`
	InputTokens      *IntMeasurement    `proto:"input_tokens"`
	OutputTokens     *IntMeasurement    `proto:"output_tokens"`
	CacheReadTokens  *IntMeasurement    `proto:"cache_read_tokens"`
	CacheWriteTokens *IntMeasurement    `proto:"cache_write_tokens"`
	Credits          *DoubleMeasurement `proto:"credits"`
	WallDurationMS   *IntMeasurement    `proto:"wall_duration_ms"`
	Cost             *Cost              `proto:"cost"`
}

// ResultCapture is an internal durable receipt, not a public contract. Attempt +
// ordinal identifies each report from the Worker result producer. Fingerprint
// covers the entire incoming result; no raw error detail is retained here.
// Publication references are pending UsageRecorded intents, not emitted Events.
type ResultCapture struct {
	ExecutionID   string
	StepID        string
	AttemptID     string
	FenceToken    uint64
	ProjectID     string
	EnvironmentID string
	CapturedAt    time.Time
	Fingerprint   string
	Reports       []UsageReport
}

const MaxUsageReports = 64
const MaxUsageBytes = 65536

var decimalUsage = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?$`)
var currencyUsage = regexp.MustCompile(`^[A-Z]{3}$`)

func measurementValid(source UsageSource, present bool) bool {
	return source == UsageUnknown && !present || (source == UsageReported || source == UsageEstimated) && present
}
func ValidateUsage(reports []UsageReport) error {
	if len(reports) > MaxUsageReports {
		return invalid("too many usage reports")
	}
	for _, r := range reports {
		if len(r.Provider) > 256 || len(r.Model) > 256 {
			return invalid("usage label too long")
		}
		for _, m := range []*IntMeasurement{r.InputTokens, r.OutputTokens, r.CacheReadTokens, r.CacheWriteTokens, r.WallDurationMS} {
			if m != nil && !measurementValid(m.Source, m.Value != nil) {
				return invalid("invalid usage measurement presence/source")
			}
		}
		if m := r.Credits; m != nil {
			if !measurementValid(m.Source, m.Value != nil) || m.Value != nil && (math.IsNaN(*m.Value) || math.IsInf(*m.Value, 0) || *m.Value < 0) {
				return invalid("invalid usage credits")
			}
		}
		if c := r.Cost; c != nil {
			if !measurementValid(c.Source, c.Amount != "" && c.Currency != "") || c.Source == UsageUnknown && (c.Amount != "" || c.Currency != "") {
				return invalid("invalid usage cost presence/source")
			}
			if c.Source != UsageUnknown && (!decimalUsage.MatchString(c.Amount) || !currencyUsage.MatchString(c.Currency)) {
				return invalid("invalid usage decimal/currency")
			}
			if c.Source == UsageEstimated && c.PriceTableVersion == "" {
				return invalid("estimated cost needs price table version")
			}
		}
	}
	raw, err := json.Marshal(reports)
	if err != nil || len(raw) > MaxUsageBytes {
		return invalid("usage payload exceeds bound or cannot encode")
	}
	return nil
}
func (c ResultCapture) Validate(e Execution, steps []StepExecution) error {
	if len(steps) != 1 || c.ExecutionID != e.ID || c.ProjectID != e.ProjectID || c.EnvironmentID != e.EnvironmentID || c.CapturedAt.IsZero() || len(c.Fingerprint) != 64 {
		return invalid("invalid usage capture attribution")
	}
	if _, err := hex.DecodeString(c.Fingerprint); err != nil {
		return invalid("invalid result fingerprint")
	}
	for _, st := range steps {
		if st.ID == c.StepID && st.ExecutionID == c.ExecutionID && st.AttemptID == c.AttemptID && c.AttemptID != "" && st.FenceToken == c.FenceToken && (st.State == StepSucceeded || st.State == StepFailed || st.State == StepCancelled || st.State == StepTimedOut) {
			return ValidateUsage(c.Reports)
		}
	}
	return invalid("usage capture requires matching terminal step")
}

// UsageID is a stable internal identity, independent of clock and mutable labels.
func (c ResultCapture) UsageID(ordinal int) string {
	raw, _ := json.Marshal([]any{c.ExecutionID, c.StepID, c.AttemptID, "worker_result", ordinal})
	sum := sha256.Sum256(raw)
	return fmt.Sprintf("usage:%x", sum)
}
