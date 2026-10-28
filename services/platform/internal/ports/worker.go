package ports

import (
	"context"
	"errors"
	"time"
)

// ErrLinkClosed is returned by Send and Receive once the link is closed. Test with errors.Is.
var ErrLinkClosed = errors.New("worker link closed")

// This file is the Worker interface of the Platform (ADR-0013, ADR-0029, FEAT-ENV-001): the only
// way the coordinator and a Worker talk, whether the Worker is the embedded one in the Platform
// process or a remote one behind a WebSocket. The messages mirror packages/contracts
// proto/amaru/worker/v1/worker.proto field for field (the `proto` struct tags name the proto
// fields); a test fails when a mirrored message drifts from the proto file. Go structs are used
// instead of generated Protobuf types so that the core takes no Protobuf dependency; the WebSocket
// adapter, when it exists, maps between the two.
//
// Implemented messages: Offer, Assignment, AcceptAttempt, RejectAttempt, LeaseRenew, LeaseRevoked,
// Result, ResultAck. Not implemented yet: handshake and enrollment, LeaseRelease, StepEvents,
// Cancel, SecretDelivery, Drain, ProtocolError, outputs and usage in Result, the envelope's
// message and ack ids (an in-process link does not lose messages).

// Link is one end of a bidirectional, ordered message channel. Send and Receive may be called from
// different goroutines. Receive blocks until a message arrives, the context ends, or the peer
// closes the link (ErrLinkClosed).
type Link[Out, In any] interface {
	Send(ctx context.Context, m Out) error
	Receive(ctx context.Context) (In, error)
	// Close ends the link for both ends; a blocked Receive returns ErrLinkClosed.
	Close() error
}

// PlatformLink is the Platform's end of a Worker connection; WorkerLink is the Worker's end.
type (
	PlatformLink = Link[PlatformMessage, WorkerMessage]
	WorkerLink   = Link[WorkerMessage, PlatformMessage]
)

// WorkerMessage is a message from a Worker to the Platform.
type WorkerMessage interface{ workerMessage() }

// PlatformMessage is a message from the Platform to a Worker.
type PlatformMessage interface{ platformMessage() }

// Offer says how many Steps the Worker can take right now. It is the absolute number of free
// slots at the moment of sending, not an increment: a later Offer replaces the earlier one.
type Offer struct {
	FreeSlots uint32 `proto:"free_slots"`
}

// Assignment hands one attempt of a Step to the Worker.
type Assignment struct {
	ExecutionID string `proto:"execution_id"`
	StepID      string `proto:"step_id"`
	// AttemptID is unique per attempt; it scopes results and events.
	AttemptID  string `proto:"attempt_id"`
	Capability string `proto:"capability"`
	InputsJSON []byte `proto:"inputs_json"`
	// Deadline is the zero time until per-Step timeouts exist (FEAT-WFL-004).
	Deadline time.Time `proto:"deadline"`
	Lease    Lease     `proto:"lease"`
	// IdempotencyKey is stable across attempts of the same Step, so a re-run side effect is deduplicated.
	IdempotencyKey       string `proto:"idempotency_key"`
	CapabilityIdempotent bool   `proto:"capability_idempotent"`
}

// Lease is the Platform's grant of a Step to a Worker. LeaseID equals the AttemptID: one Lease
// per attempt.
type Lease struct {
	LeaseID      string    `proto:"lease_id"`
	FencingToken uint64    `proto:"fencing_token"`
	ExpiresAt    time.Time `proto:"expires_at"`
}

type AcceptAttempt struct {
	AttemptID    string `proto:"attempt_id"`
	FencingToken uint64 `proto:"fencing_token"`
}

type RejectReason int

const (
	RejectNoCapacity            RejectReason = 1
	RejectCapabilityUnavailable RejectReason = 2
	RejectRevisionUnavailable   RejectReason = 3
	RejectDraining              RejectReason = 4
)

// RejectAttempt refuses an attempt before it starts. The Step goes back to queued.
type RejectAttempt struct {
	AttemptID string       `proto:"attempt_id"`
	Reason    RejectReason `proto:"reason"`
	Detail    string       `proto:"detail"`
}

// LeaseRenew is the heartbeat: it extends the Lease of the attempt.
type LeaseRenew struct {
	AttemptID    string `proto:"attempt_id"`
	FencingToken uint64 `proto:"fencing_token"`
}

// LeaseRevoked tells the Worker it no longer holds the attempt.
type LeaseRevoked struct {
	AttemptID string `proto:"attempt_id"`
	Reason    string `proto:"reason"`
}

type ResultOutcome int

const (
	OutcomeSucceeded ResultOutcome = 1
	OutcomeFailed    ResultOutcome = 2
	OutcomeCancelled ResultOutcome = 3
)

// Result ends an attempt.
type Result struct {
	AttemptID    string        `proto:"attempt_id"`
	FencingToken uint64        `proto:"fencing_token"`
	Outcome      ResultOutcome `proto:"outcome"`
	ErrorCode    string        `proto:"error_code"`
	ErrorDetail  string        `proto:"error_detail"`
}

type ResultDisposition int

const (
	ResultAccepted      ResultDisposition = 1
	ResultDuplicate     ResultDisposition = 2 // the attempt was already completed: ignored
	ResultRejectedStale ResultDisposition = 3 // the Worker no longer holds the Lease
)

// ResultAck answers a Result.
type ResultAck struct {
	AttemptID   string            `proto:"attempt_id"`
	Disposition ResultDisposition `proto:"disposition"`
}

func (Offer) workerMessage()         {}
func (AcceptAttempt) workerMessage() {}
func (RejectAttempt) workerMessage() {}
func (LeaseRenew) workerMessage()    {}
func (Result) workerMessage()        {}

func (Assignment) platformMessage()   {}
func (LeaseRevoked) platformMessage() {}
func (ResultAck) platformMessage()    {}
