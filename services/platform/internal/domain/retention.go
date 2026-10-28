package domain

import (
	"fmt"
	"time"
)

// DataClass is a kind of stored data that has its own retention (DATA_ARCHITECTURE §5).
type DataClass string

const (
	// ClassAudit is Events, Approvals, Grants and other audit records.
	ClassAudit DataClass = "audit"
	// ClassLogs is provider stdout/stderr, transcripts and check output.
	ClassLogs DataClass = "logs"
	// ClassIntermediateArtifact is an Artifact no Approval or accepted Work Item references.
	ClassIntermediateArtifact DataClass = "intermediate_artifact"
	// ClassReferencedArtifact is an Artifact an Approval or accepted Work Item references. It is
	// kept while referenced; that rule is fixed and has no setting.
	ClassReferencedArtifact DataClass = "referenced_artifact"
)

// Retention is how long a class is kept after it was created. Indefinite ignores Keep.
type Retention struct {
	Indefinite bool          `json:"indefinite,omitempty"`
	Keep       time.Duration `json:"keep,omitempty"`
}

// Days is a Retention of n days.
func Days(n int) Retention { return Retention{Keep: time.Duration(n) * 24 * time.Hour} }

// Forever is the Retention that never expires.
var Forever = Retention{Indefinite: true}

// atLeast reports whether r keeps data at least as long as o does.
func (r Retention) atLeast(o Retention) bool {
	switch {
	case r.Indefinite:
		return true
	case o.Indefinite:
		return false
	}
	return r.Keep >= o.Keep
}

// RetentionPolicy is the retention per data class an Organization set.
type RetentionPolicy map[DataClass]Retention

// DefaultRetention is the proposed deployment-wide default of DATA_ARCHITECTURE §5. The values
// are proposed and not yet validated with operators.
func DefaultRetention() RetentionPolicy {
	return RetentionPolicy{
		ClassAudit:                Forever,
		ClassLogs:                 Days(30),
		ClassIntermediateArtifact: Days(90),
		ClassReferencedArtifact:   Forever,
	}
}

// NewRetentionPolicy applies the Organization's settings over the defaults and validates the
// result. It refuses, with ErrInvalid: an unknown class; any setting for referenced artifacts
// (fixed); a finite setting that is not positive; and an audit retention shorter than the
// retention of the Artifacts it justifies (DATA_ARCHITECTURE §5).
func NewRetentionPolicy(set map[DataClass]Retention) (RetentionPolicy, error) {
	p := DefaultRetention()
	for class, r := range set {
		if _, known := p[class]; !known {
			return nil, invalid("retention: unknown data class %q", class)
		}
		if class == ClassReferencedArtifact {
			return nil, invalid("retention: %q is kept while referenced and has no setting", class)
		}
		if !r.Indefinite && r.Keep <= 0 {
			return nil, invalid("retention: %q must keep for a positive time or be indefinite", class)
		}
		p[class] = r
	}
	if !p[ClassAudit].atLeast(p[ClassIntermediateArtifact]) {
		return nil, invalid("retention: audit records are kept at least as long as the artifacts they justify")
	}
	return p, nil
}

// Expired reports whether data of the class, created at createdAt, may be deleted at now. Data
// that something still references is never expired: an Artifact an Approval references is kept
// while referenced, whatever its class setting.
func (p RetentionPolicy) Expired(class DataClass, createdAt, now time.Time, referenced bool) (bool, error) {
	r, ok := p[class]
	if !ok {
		return false, fmt.Errorf("%w: retention: unknown data class %q", ErrInvalid, class)
	}
	if referenced || class == ClassReferencedArtifact || r.Indefinite {
		return false, nil
	}
	return !now.Before(createdAt.Add(r.Keep)), nil
}
