package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"

	ep "github.com/fergusstrange/embedded-postgres/v2"
)

// cmdBundle plays the installer build: it fetches the pinned PostgreSQL
// distribution once (build time, network), copies it to <out>/pg, builds the
// supervisor from the pinned module into <out>/supervisor and writes a
// checksum manifest. At runtime nothing is downloaded.
func cmdBundle(args []string) error {
	fl := flag.NewFlagSet("bundle", flag.ExitOnError)
	out := fl.String("out", "", "bundle directory to create")
	ver := fl.String("pg", string(ep.V18), "PostgreSQL distribution version")
	fl.Parse(args)
	if *out == "" {
		return fmt.Errorf("-out required")
	}
	ctx := context.Background()
	tmp, err := os.MkdirTemp("", "spike-pgcache-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	inst, err := ep.DownloadProvider{CacheDir: tmp}.Acquire(ctx, ep.BinaryRequest{Version: ep.PostgresVersion(*ver), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH})
	if err != nil {
		return fmt.Errorf("fetch distribution: %w", err)
	}
	defer inst.Release()
	if err := copyTree(inst.Dir, filepath.Join(*out, "pg")); err != nil {
		return err
	}
	sup := filepath.Join(*out, "supervisor")
	if runtime.GOOS == "windows" {
		sup += ".exe"
	}
	build := exec.Command("go", "build", "-trimpath", "-o", sup, "github.com/fergusstrange/embedded-postgres/v2/cmd/embedded-postgres")
	build.Stdout, build.Stderr = os.Stdout, os.Stderr
	if err := build.Run(); err != nil {
		return fmt.Errorf("build supervisor: %w", err)
	}
	return writeManifest(*out)
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		t := filepath.Join(dst, rel)
		switch {
		case d.IsDir():
			return os.MkdirAll(t, 0o755)
		case d.Type()&fs.ModeSymlink != 0:
			l, err := os.Readlink(p)
			if err != nil {
				return err
			}
			return os.Symlink(l, t)
		default:
			info, err := d.Info()
			if err != nil {
				return err
			}
			in, err := os.Open(p)
			if err != nil {
				return err
			}
			defer in.Close()
			o, err := os.OpenFile(t, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
			if err != nil {
				return err
			}
			if _, err := io.Copy(o, in); err != nil {
				o.Close()
				return err
			}
			return o.Close()
		}
	})
}

func writeManifest(dir string) error {
	var lines []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Type()&fs.ModeSymlink != 0 || filepath.Base(p) == "MANIFEST.sha256" {
			return err
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		h := sha256.New()
		if _, err := io.Copy(h, f); err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		lines = append(lines, hex.EncodeToString(h.Sum(nil))+"  "+filepath.ToSlash(rel)+"\n")
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(lines)
	f, err := os.Create(filepath.Join(dir, "MANIFEST.sha256"))
	if err != nil {
		return err
	}
	defer f.Close()
	for _, l := range lines {
		f.WriteString(l)
	}
	return nil
}

// dirSize returns the total size of regular files under dir.
func dirSize(dir string) int64 {
	var n int64
	filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			if i, e := d.Info(); e == nil {
				n += i.Size()
			}
		}
		return nil
	})
	return n
}
