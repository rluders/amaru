//go:build linux

package discovery

import (
	"crypto/rand"
	"errors"
	"io"
	"os"
	"syscall"
)

var randomSource io.Reader = rand.Reader

func private(st os.FileInfo, dir bool) bool {
	if st.Mode()&os.ModeSymlink != 0 || st.IsDir() != dir {
		return false
	}
	if !dir && !st.Mode().IsRegular() {
		return false
	}
	want := os.FileMode(0600)
	if dir {
		want = 0700
	}
	owner, ok := st.Sys().(*syscall.Stat_t)
	return ok && owner.Uid == uint32(os.Geteuid()) && st.Mode().Perm() == want && st.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) == 0
}
func openPrivateDirectory(dir string, create bool) (*os.Root, error) {
	if create {
		if err := os.Mkdir(dir, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return nil, errUnsafe
		}
	}
	st, err := os.Lstat(dir)
	if err != nil || !private(st, true) {
		return nil, errUnsafe
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, errUnsafe
	}
	actual, err := root.Stat(".")
	if err != nil || !private(actual, true) || !os.SameFile(st, actual) {
		root.Close()
		return nil, errUnsafe
	}
	return root, nil
}
func lockExclusive(f *os.File) error {
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return ErrRunning
		}
		return errUnsafe
	}
	return nil
}
