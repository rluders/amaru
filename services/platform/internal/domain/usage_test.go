package domain

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"
)

func TestUsageValidation(t *testing.T) {
	zero := uint64(0)
	negative := -1.0
	nan := math.NaN()
	for _, tt := range []struct {
		name string
		r    UsageReport
		bad  bool
	}{
		{"missing is unknown", UsageReport{}, false},
		{"reported zero", UsageReport{InputTokens: &IntMeasurement{Source: UsageReported, Value: &zero}}, false},
		{"unknown absent", UsageReport{InputTokens: &IntMeasurement{Source: UsageUnknown}}, false},
		{"unknown with zero", UsageReport{InputTokens: &IntMeasurement{Source: UsageUnknown, Value: &zero}}, true},
		{"reported absent", UsageReport{InputTokens: &IntMeasurement{Source: UsageReported}}, true},
		{"unspecified", UsageReport{InputTokens: &IntMeasurement{}}, true},
		{"negative credits", UsageReport{Credits: &DoubleMeasurement{Source: UsageReported, Value: &negative}}, true},
		{"nonfinite credits", UsageReport{Credits: &DoubleMeasurement{Source: UsageReported, Value: &nan}}, true},
		{"unknown cost partial", UsageReport{Cost: &Cost{Source: UsageUnknown, Currency: "USD"}}, true},
		{"float exponent refused", UsageReport{Cost: &Cost{Source: UsageReported, Amount: "1e-3", Currency: "USD"}}, true},
		{"estimate needs version", UsageReport{Cost: &Cost{Source: UsageEstimated, Amount: "0", Currency: "USD"}}, true},
		{"label bound", UsageReport{Model: strings.Repeat("x", 257)}, true},
		{"payload bound", UsageReport{Cost: &Cost{Source: UsageReported, Amount: strings.Repeat("1", MaxUsageBytes), Currency: "USD"}}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUsage([]UsageReport{tt.r})
			if (err != nil) != tt.bad {
				t.Fatalf("err=%v bad=%v", err, tt.bad)
			}
		})
	}
	if ValidateUsage(make([]UsageReport, MaxUsageReports+1)) == nil {
		t.Fatal("unbounded count")
	}
}

func TestUsageEvidenceAttribution(t *testing.T) {
	now := time.Now().UTC()
	base := ResultCapture{ExecutionID: "e", StepID: "s", ProjectID: "p", AttemptID: "a", CapturedAt: now, Reports: []UsageReport{{}}}
	for _, field := range []string{"execution", "step", "project", "environment", "duplicate"} {
		t.Run(field, func(t *testing.T) {
			c := base
			switch field {
			case "execution":
				c.ExecutionID = "foreign"
			case "step":
				c.StepID = "foreign"
			case "project":
				c.ProjectID = "foreign"
			case "environment":
				c.EnvironmentID = "foreign"
			}
			in := ExecutionEvidenceInput{Execution: Execution{ID: "e", ProjectID: "p"}, Steps: []StepExecution{{ID: "s", ExecutionID: "e"}}, Usage: []ResultCapture{c}}
			if field == "duplicate" {
				in.Usage = append(in.Usage, c)
			}
			if _, err := BuildExecutionEvidence(in); !errors.Is(err, ErrInvalid) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}
