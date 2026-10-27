package domain

import "errors"

// ErrInvalidTransition is returned (wrapped) for any state change the lifecycle does not
// document. Test with errors.Is.
var ErrInvalidTransition = errors.New("invalid state transition")

// ErrReasonRequired is returned when a transition that must explain itself has no reason.
var ErrReasonRequired = errors.New("transition requires a reason")

// ErrErrorRequired is returned when a failure is recorded without an ExecError.
var ErrErrorRequired = errors.New("failure requires an error")

// ExecError describes why an Execution or Step failed. Code is a stable, provider-neutral
// token; Message is free text and never carries credentials.
type ExecError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ErrInvalid is returned (wrapped) when a definition fails validation. Test with errors.Is.
var ErrInvalid = errors.New("invalid definition")

// ErrConflict is returned (wrapped) when an edit was made against a revision that is no longer
// the latest: someone else changed the definition first. Test with errors.Is.
var ErrConflict = errors.New("stale revision")

// ErrDeleted is returned (wrapped) when a deleted definition is edited or deleted again.
var ErrDeleted = errors.New("definition is deleted")

// ErrNotPermitted is returned (wrapped) when the actor may not make the change, for example an
// Agent or a Step setting a Work Item status. Test with errors.Is.
var ErrNotPermitted = errors.New("actor is not permitted")

// ErrDependenciesNotDone is returned (wrapped) when a Work Item is started while a dependency is
// not done. The message names the unmet dependencies. Test with errors.Is.
var ErrDependenciesNotDone = errors.New("dependencies are not done")

// ErrDependencyCycle is returned (wrapped, and also matching ErrInvalid) when a dependency edit
// would make a Work Item depend on itself through others. Test with errors.Is.
var ErrDependencyCycle = errors.New("dependency cycle")

// ErrNotFound is returned (wrapped) when a stored record does not exist. Test with errors.Is.
var ErrNotFound = errors.New("not found")

// ErrExists is returned (wrapped) when a record with the same identity is already stored. Test
// with errors.Is.
var ErrExists = errors.New("already exists")
