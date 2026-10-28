// Package fsblob is the filesystem ports.BlobStore: a content-addressed directory under the
// data directory (ADR-0017, the compact-install default). A blob lives at
// <root>/sha256/<first two hex>/<rest>, is written to a temporary file in the same directory
// tree and renamed into place, and is never rewritten.
package fsblob

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// Store is the filesystem blob store under root.
type Store struct{ root string }

var _ ports.BlobStore = (*Store)(nil)

// New returns a store that keeps blobs under root, creating it if needed.
func New(root string) (*Store, error) {
	if root == "" {
		return nil, errors.New("fsblob: root is required")
	}
	if err := os.MkdirAll(filepath.Join(root, "tmp"), 0o700); err != nil {
		return nil, fmt.Errorf("fsblob: create root: %w", err)
	}
	return &Store{root: root}, nil
}

func (s *Store) path(hash string) (string, error) {
	if !domain.ValidContentHash(hash) {
		return "", fmt.Errorf("%w: %q is not sha256:<hex>", domain.ErrInvalid, hash)
	}
	h := hash[len(domain.ContentHashPrefix):]
	return filepath.Join(s.root, "sha256", h[:2], h[2:]), nil
}

func (s *Store) Put(ctx context.Context, r io.Reader, max int64) (string, int64, error) {
	if max <= 0 {
		max = ports.DefaultMaxBlob
	}
	tmp, err := os.CreateTemp(filepath.Join(s.root, "tmp"), "put-*")
	if err != nil {
		return "", 0, fmt.Errorf("fsblob: create temp file: %w", err)
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	// The hash is computed over the bytes written, so it is the Platform's own.
	sum := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmp, sum), io.LimitReader(&ctxReader{ctx, r}, max+1))
	if err != nil {
		return "", 0, fmt.Errorf("fsblob: write content: %w", err)
	}
	if n > max {
		return "", 0, fmt.Errorf("%w: more than %d bytes", domain.ErrTooLarge, max)
	}
	hash := domain.ContentHashPrefix + hex.EncodeToString(sum.Sum(nil))
	dst, _ := s.path(hash)
	if _, err := os.Stat(dst); err == nil {
		return hash, n, nil // already stored: the same content is kept once
	}
	if err := tmp.Sync(); err != nil {
		return "", 0, fmt.Errorf("fsblob: sync: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return "", 0, fmt.Errorf("fsblob: create directory: %w", err)
	}
	if err := os.Chmod(tmp.Name(), 0o400); err != nil {
		return "", 0, fmt.Errorf("fsblob: make read-only: %w", err)
	}
	if err := os.Rename(tmp.Name(), dst); err != nil {
		return "", 0, fmt.Errorf("fsblob: store blob: %w", err)
	}
	return hash, n, nil
}

func (s *Store) Open(_ context.Context, hash string) (io.ReadCloser, error) {
	p, err := s.path(hash)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: blob %s", domain.ErrNotFound, hash)
	}
	if err != nil {
		return nil, fmt.Errorf("fsblob: open blob: %w", err)
	}
	return &verifying{f: f, sum: sha256.New(), want: hash}, nil
}

// verifying hashes what it reads and reports a damaged blob at the end of the stream, so a reader
// that reads to EOF never takes corrupt bytes as the content with that hash.
type verifying struct {
	f   *os.File
	sum interface {
		io.Writer
		Sum([]byte) []byte
	}
	want string
}

func (v *verifying) Read(p []byte) (int, error) {
	n, err := v.f.Read(p)
	v.sum.Write(p[:n])
	if err == io.EOF && domain.ContentHashPrefix+hex.EncodeToString(v.sum.Sum(nil)) != v.want {
		return n, fmt.Errorf("%w: blob %s", domain.ErrHashMismatch, v.want)
	}
	return n, err
}

func (v *verifying) Close() error { return v.f.Close() }

// ctxReader stops a copy when the context ends.
type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (c *ctxReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}
