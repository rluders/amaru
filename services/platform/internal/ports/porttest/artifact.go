package porttest

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// BlobStore runs the blob contract (ADR-0017) against newStore, which returns an empty store each
// call: the hash and size are the Platform's, the same content is one blob, a limit stops a large
// input before anything is stored, and an unknown hash is not found.
func BlobStore(t *testing.T, newStore func(t *testing.T) ports.BlobStore) {
	ctx := context.Background()
	read := func(t *testing.T, s ports.BlobStore, hash string) []byte {
		t.Helper()
		r, err := s.Open(ctx, hash)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		b, err := io.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}

	t.Run("the Platform computes the hash and size of what it read", func(t *testing.T) {
		s := newStore(t)
		content := []byte("hello artifact")
		hash, size, err := s.Put(ctx, bytes.NewReader(content), 0)
		if err != nil {
			t.Fatal(err)
		}
		if hash != domain.ContentHash(content) || size != int64(len(content)) {
			t.Fatalf("hash %s size %d", hash, size)
		}
		if got := read(t, s, hash); !bytes.Equal(got, content) {
			t.Fatalf("read back %q", got)
		}
	})

	t.Run("the same content is stored once, and an empty one is allowed", func(t *testing.T) {
		s := newStore(t)
		a, _, _ := s.Put(ctx, strings.NewReader("same"), 0)
		b, _, err := s.Put(ctx, strings.NewReader("same"), 0)
		if err != nil || a != b {
			t.Fatalf("%s %s %v", a, b, err)
		}
		h, n, err := s.Put(ctx, strings.NewReader(""), 0)
		if err != nil || n != 0 || h != domain.ContentHash(nil) {
			t.Fatalf("empty: %s %d %v", h, n, err)
		}
	})

	t.Run("content over the limit is refused and not stored", func(t *testing.T) {
		s := newStore(t)
		if _, _, err := s.Put(ctx, strings.NewReader("0123456789"), 9); !errors.Is(err, domain.ErrTooLarge) {
			t.Fatalf("got %v, want too large", err)
		}
		if _, err := s.Open(ctx, domain.ContentHash([]byte("0123456789"))); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("a refused blob was stored: %v", err)
		}
		if _, _, err := s.Put(ctx, strings.NewReader("0123456789"), 10); err != nil {
			t.Fatalf("exactly at the limit: %v", err)
		}
	})

	t.Run("an unknown or malformed hash is refused", func(t *testing.T) {
		s := newStore(t)
		if _, err := s.Open(ctx, domain.ContentHash([]byte("nope"))); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("unknown: %v", err)
		}
		if _, err := s.Open(ctx, "../../etc/passwd"); err == nil {
			t.Error("a malformed hash was opened")
		}
	})
}

// ArtifactIndex runs the version contract (FEAT-ART-001) against newIndex, which returns an empty
// index each call.
func ArtifactIndex(t *testing.T, newIndex func(t *testing.T) ports.ArtifactIndex) {
	ctx := context.Background()
	at := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	ver := func(id, exec, step, attempt, content string) domain.ArtifactVersion {
		return domain.ArtifactVersion{ArtifactID: id, Hash: domain.ContentHash([]byte(content)), Size: int64(len(content)),
			MediaType: "application/json", Path: "out/" + id + ".json", ExecutionID: exec, StepID: step, AttemptID: attempt, CreatedAt: at}
	}

	t.Run("versions are numbered from 1 per Artifact and keep their producer", func(t *testing.T) {
		s := newIndex(t)
		a1, err := s.Record(ctx, ver("report", "e1", "e1.s1", "att-1", "one"))
		if err != nil || a1.Version != 1 {
			t.Fatalf("%+v %v", a1, err)
		}
		a2, _ := s.Record(ctx, ver("report", "e2", "e2.s1", "att-2", "two"))
		b1, _ := s.Record(ctx, ver("log", "e2", "e2.s1", "att-2", "log"))
		if a2.Version != 2 || b1.Version != 1 {
			t.Fatalf("report v%d, log v%d", a2.Version, b1.Version)
		}
		got, err := s.Versions(ctx, "report")
		if err != nil || len(got) != 2 || got[0].Hash != a1.Hash || got[1].ExecutionID != "e2" || got[1].AttemptID != "att-2" || !got[1].CreatedAt.Equal(at) {
			t.Fatalf("%+v %v", got, err)
		}
		if none, err := s.Versions(ctx, "none"); err != nil || len(none) != 0 {
			t.Errorf("unknown artifact: %+v %v", none, err)
		}
	})

	t.Run("recording the same attempt again returns the version already stored", func(t *testing.T) {
		s := newIndex(t)
		first, _ := s.Record(ctx, ver("report", "e1", "e1.s1", "att-1", "one"))
		again, err := s.Record(ctx, ver("report", "e1", "e1.s1", "att-1", "one"))
		if err != nil || again.Version != first.Version {
			t.Fatalf("%+v %v", again, err)
		}
		if vs, _ := s.Versions(ctx, "report"); len(vs) != 1 {
			t.Fatalf("%d versions, want 1", len(vs))
		}
	})

	t.Run("an invalid version is refused and stores nothing", func(t *testing.T) {
		s := newIndex(t)
		for name, mod := range map[string]func(*domain.ArtifactVersion){
			"hash not computed": func(v *domain.ArtifactVersion) { v.Hash = "abc" },
			"no producer":       func(v *domain.ArtifactVersion) { v.AttemptID = "" },
			"path outside":      func(v *domain.ArtifactVersion) { v.Path = "../x" },
			"bad media type":    func(v *domain.ArtifactVersion) { v.MediaType = "JSON" },
			"negative size":     func(v *domain.ArtifactVersion) { v.Size = -1 },
		} {
			v := ver("report", "e1", "e1.s1", "att-1", "one")
			mod(&v)
			if _, err := s.Record(ctx, v); !errors.Is(err, domain.ErrInvalid) {
				t.Errorf("%s: %v", name, err)
			}
		}
		if vs, _ := s.Versions(ctx, "report"); len(vs) != 0 {
			t.Fatalf("stored %+v", vs)
		}
	})

	t.Run("the versions an Execution produced are listed by Step and Artifact", func(t *testing.T) {
		s := newIndex(t)
		_, _ = s.Record(ctx, ver("b", "e1", "e1.s2", "att-2", "x"))
		_, _ = s.Record(ctx, ver("a", "e1", "e1.s2", "att-2", "x"))
		_, _ = s.Record(ctx, ver("z", "e1", "e1.s1", "att-1", "x"))
		_, _ = s.Record(ctx, ver("a", "e9", "e9.s1", "att-9", "x"))
		got, err := s.ByExecution(ctx, "e1")
		if err != nil || len(got) != 3 || got[0].ArtifactID != "z" || got[1].ArtifactID != "a" || got[2].ArtifactID != "b" {
			t.Fatalf("%+v %v", got, err)
		}
	})
}
