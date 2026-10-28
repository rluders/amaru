package ports

import (
	"context"
	"io"

	"github.com/rluders/amaru/services/platform/internal/domain"
)

// BlobStore keeps Artifact content addressed by its hash (ADR-0017). A blob is immutable: the
// same content is stored once, whoever put it.
type BlobStore interface {
	// Put reads r to the end, stores it and returns the hash and size the Platform computed from
	// the bytes it read. It reads at most max bytes and returns domain.ErrTooLarge when r has more,
	// storing nothing. A max of zero or less means DefaultMaxBlob.
	Put(ctx context.Context, r io.Reader, max int64) (hash string, size int64, err error)
	// Open returns the content with the hash, or domain.ErrNotFound. The caller closes it. It
	// returns domain.ErrHashMismatch when the stored bytes no longer have that hash.
	Open(ctx context.Context, hash string) (io.ReadCloser, error)
}

// DefaultMaxBlob bounds one blob when the caller gives no limit: 1 GiB.
const DefaultMaxBlob int64 = 1 << 30

// ArtifactIndex keeps the versions of Artifacts in the Platform store. Versions are never edited
// or deleted.
type ArtifactIndex interface {
	// Record stores v as the next version of its Artifact and returns it with Version set. The
	// same attempt recording the same Artifact again returns the version already stored, so a
	// retried Record adds nothing. v.Version is ignored. It returns domain.ErrInvalid for an
	// invalid version.
	Record(ctx context.Context, v domain.ArtifactVersion) (domain.ArtifactVersion, error)
	// Versions returns the versions of the Artifact in order, or an empty slice.
	Versions(ctx context.Context, artifactID string) ([]domain.ArtifactVersion, error)
	// ByExecution returns every version a Step of the Execution produced, ordered by Step, then
	// Artifact id. It is what the Evidence bundle reads (FEAT-ART-002).
	ByExecution(ctx context.Context, executionID string) ([]domain.ArtifactVersion, error)
}
