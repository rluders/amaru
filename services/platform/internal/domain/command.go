package domain

import (
	"errors"
	"time"
)

// ErrDuplicateCommand is returned (wrapped) by a store write that carries a CommandRecord whose
// (Name, Key) is already stored. Nothing else in the write was applied. Test with errors.Is.
var ErrDuplicateCommand = errors.New("command already recorded")

// ErrKeyReused is returned (wrapped) when an idempotency key is presented again with a different
// request: the key names one request, so the caller made a mistake. Test with errors.Is.
var ErrKeyReused = errors.New("idempotency key reused for a different request")

// CommandRecord is the outcome of one mutating command, stored in the same write as the change
// it made (FEAT-EXE-005). A second request with the same Name and Key does not run again: it
// gets Result back. Fingerprint identifies the request, so a reused key is told from a retry.
type CommandRecord struct {
	Name        string // the command, for example "execution.submit"
	Key         string // the caller's idempotency key; unique per Name
	Fingerprint string // hash of the request that ran
	Result      []byte // the original result, encoded by the command
	At          time.Time
}

// Valid reports whether the record can be stored.
func (r CommandRecord) Valid() error {
	if r.Name == "" || r.Key == "" || r.Fingerprint == "" {
		return errors.Join(ErrInvalid, errors.New("command record needs a name, a key and a fingerprint"))
	}
	return nil
}
