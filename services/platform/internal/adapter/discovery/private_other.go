//go:build !linux

package discovery

import (
	"crypto/rand"
	"errors"
	"io"
	"os"
)

var randomSource io.Reader = rand.Reader

func private(os.FileInfo, bool) bool { return false }
func openPrivateDirectory(string, bool) (*os.Root, error) {
	return nil, errors.New("private discovery is not implemented on this OS")
}
func lockExclusive(*os.File) error { return errUnsafe }
