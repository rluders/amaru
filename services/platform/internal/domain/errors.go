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
