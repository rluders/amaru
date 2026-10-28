package fsblob

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
	"github.com/rluders/amaru/services/platform/internal/ports/porttest"
)

func TestBlobStoreConformance(t *testing.T) {
	porttest.BlobStore(t, func(t *testing.T) ports.BlobStore {
		s, err := New(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		return s
	})
}

func TestBlobLayoutAndDamage(t *testing.T) {
	root := t.TempDir()
	s, _ := New(root)
	hash, _, err := s.Put(t.Context(), strings.NewReader("content"), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := strings.TrimPrefix(hash, domain.ContentHashPrefix)
	p := filepath.Join(root, "sha256", h[:2], h[2:])
	info, err := os.Stat(p)
	if err != nil || info.Mode().Perm() != 0o400 {
		t.Fatalf("blob %v %v: want a read-only file at the content address", info, err)
	}
	if left, _ := os.ReadDir(filepath.Join(root, "tmp")); len(left) != 0 {
		t.Errorf("temporary files left behind: %v", left)
	}
	// A blob changed on disk is reported at the end of the stream, not served as the content.
	if err := os.Chmod(p, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := s.Open(t.Context(), hash)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if _, err := io.ReadAll(r); !errors.Is(err, domain.ErrHashMismatch) {
		t.Fatalf("damaged blob: got %v, want hash mismatch", err)
	}
}

func TestTooLargeLeavesNoTemporaryFile(t *testing.T) {
	root := t.TempDir()
	s, _ := New(root)
	if _, _, err := s.Put(t.Context(), strings.NewReader("0123456789"), 5); !errors.Is(err, domain.ErrTooLarge) {
		t.Fatal(err)
	}
	if left, _ := os.ReadDir(filepath.Join(root, "tmp")); len(left) != 0 {
		t.Errorf("temporary files left behind: %v", left)
	}
}
