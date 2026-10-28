// Package artifact records the outputs of a Step as Artifact versions (FEAT-ART-001, ADR-0017).
// The Platform reads each output from the Step's output location, computes its hash and size
// itself, stores the content in the blob store and records a version. A hash a Provider reports
// is only compared; it is never stored. An output outside the location is rejected, whether it
// says so with an absolute path, a ".." element or a symbolic link that leads out.
package artifact

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// Ingester stores outputs. Its fields are required except MaxSize.
type Ingester struct {
	Blobs ports.BlobStore
	Index ports.ArtifactIndex
	Now   func() time.Time
	// MaxSize bounds one output in bytes; zero means ports.DefaultMaxBlob.
	MaxSize int64
}

// Source names the Step attempt that produced the outputs.
type Source struct {
	ExecutionID, StepID, AttemptID string
}

// Output is one output a Provider reports.
type Output struct {
	// ArtifactID is the stable id of the Artifact this output is a version of.
	ArtifactID string
	// Path is relative to the output location and uses forward slashes.
	Path      string
	MediaType string
	// ClaimedHash is the hash the Provider says the content has, or empty. When set and different
	// from the Platform's hash the output is rejected with domain.ErrHashMismatch.
	ClaimedHash string
}

// Ingest records outputs from location, a directory on the Platform's filesystem that holds the
// Step's outputs, and returns the versions in the order of outs. It works in two passes: every
// output is read, hashed and stored first, and a version is recorded only if all of them passed,
// so one bad output records none. A blob stored by a pass that then failed stays unreferenced
// until blob garbage collection (not implemented). Ingesting the same attempt again records no
// new version.
func (i *Ingester) Ingest(ctx context.Context, src Source, location string, outs []Output) ([]domain.ArtifactVersion, error) {
	if src.ExecutionID == "" || src.StepID == "" || src.AttemptID == "" {
		return nil, fmt.Errorf("%w: the producing execution, step and attempt are required", domain.ErrInvalid)
	}
	paths := make([]string, len(outs))
	for n, o := range outs {
		p, err := domain.OutputPath(o.Path)
		if err != nil {
			return nil, fmt.Errorf("output %d: %w", n, err)
		}
		paths[n] = p
	}
	// os.Root refuses a path that leaves the directory, by ".." or by a symbolic link, so the
	// lexical check above is not the only line of defense.
	root, err := os.OpenRoot(location)
	if err != nil {
		return nil, fmt.Errorf("open output location: %w", err)
	}
	defer root.Close()
	versions := make([]domain.ArtifactVersion, len(outs))
	for n, o := range outs {
		hash, size, err := i.store(ctx, root, paths[n])
		if err != nil {
			return nil, fmt.Errorf("output %q: %w", o.Path, err)
		}
		if o.ClaimedHash != "" && o.ClaimedHash != hash {
			return nil, fmt.Errorf("output %q: %w: the provider claimed %s, the content is %s", o.Path, domain.ErrHashMismatch, o.ClaimedHash, hash)
		}
		versions[n] = domain.ArtifactVersion{
			ArtifactID: o.ArtifactID, Hash: hash, Size: size, MediaType: o.MediaType, Path: paths[n],
			ExecutionID: src.ExecutionID, StepID: src.StepID, AttemptID: src.AttemptID, CreatedAt: i.Now().UTC(),
		}
		if err := versions[n].Validate(); err != nil {
			return nil, err
		}
	}
	for n := range versions {
		v, err := i.Index.Record(ctx, versions[n])
		if err != nil {
			return nil, fmt.Errorf("record output %q: %w", outs[n].Path, err)
		}
		versions[n] = v
	}
	return versions, nil
}

func (i *Ingester) store(ctx context.Context, root *os.Root, path string) (string, int64, error) {
	f, err := root.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", 0, fmt.Errorf("%w: no such file", domain.ErrNotFound)
		}
		if errors.Is(err, os.ErrPermission) {
			return "", 0, fmt.Errorf("%w: %v", domain.ErrNotPermitted, err)
		}
		// os.Root reports a path that leaves the root, and a symbolic link out of it, as an error
		// that is not ErrNotExist.
		return "", 0, fmt.Errorf("%w: %v", domain.ErrOutsideOutput, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", 0, fmt.Errorf("stat output: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", 0, fmt.Errorf("%w: not a regular file", domain.ErrInvalid)
	}
	return i.Blobs.Put(ctx, f, i.MaxSize)
}
