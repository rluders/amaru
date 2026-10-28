package fakes

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"sync"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// BlobStore is an in-memory ports.BlobStore.
type BlobStore struct {
	mu    sync.Mutex
	blobs map[string][]byte
}

var _ ports.BlobStore = (*BlobStore)(nil)

func NewBlobStore() *BlobStore { return &BlobStore{blobs: map[string][]byte{}} }

func (s *BlobStore) Put(_ context.Context, r io.Reader, max int64) (string, int64, error) {
	if max <= 0 {
		max = ports.DefaultMaxBlob
	}
	b, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return "", 0, fmt.Errorf("fakes: read content: %w", err)
	}
	if int64(len(b)) > max {
		return "", 0, fmt.Errorf("%w: more than %d bytes", domain.ErrTooLarge, max)
	}
	h := domain.ContentHash(b)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.blobs[h] = b
	return h, int64(len(b)), nil
}

func (s *BlobStore) Open(_ context.Context, hash string) (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.blobs[hash]
	if !ok {
		return nil, fmt.Errorf("%w: blob %s", domain.ErrNotFound, hash)
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

// ArtifactIndex is an in-memory ports.ArtifactIndex.
type ArtifactIndex struct {
	mu       sync.Mutex
	versions map[string][]domain.ArtifactVersion
}

var _ ports.ArtifactIndex = (*ArtifactIndex)(nil)

func NewArtifactIndex() *ArtifactIndex {
	return &ArtifactIndex{versions: map[string][]domain.ArtifactVersion{}}
}

func (s *ArtifactIndex) Record(_ context.Context, v domain.ArtifactVersion) (domain.ArtifactVersion, error) {
	if err := v.Validate(); err != nil {
		return domain.ArtifactVersion{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, old := range s.versions[v.ArtifactID] {
		if old.ExecutionID == v.ExecutionID && old.StepID == v.StepID && old.AttemptID == v.AttemptID && old.Path == v.Path {
			return old, nil
		}
	}
	v.Version = int64(len(s.versions[v.ArtifactID])) + 1
	s.versions[v.ArtifactID] = append(s.versions[v.ArtifactID], v)
	return v, nil
}

func (s *ArtifactIndex) Versions(_ context.Context, id string) ([]domain.ArtifactVersion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]domain.ArtifactVersion{}, s.versions[id]...), nil
}

func (s *ArtifactIndex) ByExecution(_ context.Context, exec string) ([]domain.ArtifactVersion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []domain.ArtifactVersion{}
	for _, vs := range s.versions {
		for _, v := range vs {
			if v.ExecutionID == exec {
				out = append(out, v)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].StepID != out[j].StepID {
			return out[i].StepID < out[j].StepID
		}
		if out[i].ArtifactID != out[j].ArtifactID {
			return out[i].ArtifactID < out[j].ArtifactID
		}
		return out[i].Version < out[j].Version
	})
	return out, nil
}
