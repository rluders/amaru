package coordinator_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/rluders/amaru/services/platform/internal/coordinator/coordtest"
	"github.com/rluders/amaru/services/platform/internal/ports"
	"github.com/rluders/amaru/services/platform/internal/worker/inproc"
)

// serialized wraps a link so that every message crosses it as JSON bytes and is rebuilt on the
// other side. It stands in for a remote Worker's wire: it proves the coordinator depends only on
// message content, not on shared memory, ordering tricks or pointer identity. It is not a network
// and it is not the WebSocket/Protobuf transport, which does not exist yet.
type serialized[Out, In any] struct {
	ports.Link[Out, In]
	decode func(name string, raw []byte) (In, error)
}

type frame struct {
	Type string          `json:"type"`
	Body json.RawMessage `json:"body"`
}

func name(m any) string { return fmt.Sprintf("%T", m) }

func (s serialized[Out, In]) Send(ctx context.Context, m Out) error {
	raw, err := json.Marshal(m)
	if err != nil {
		return err
	}
	f, _ := json.Marshal(frame{Type: name(m), Body: raw})
	// Send the bytes through the real link as a rebuilt message of the same type.
	var rebuilt any
	if rebuilt, err = rebuild(f); err != nil {
		return err
	}
	return s.Link.Send(ctx, rebuilt.(Out))
}

func rebuild(f []byte) (any, error) {
	var fr frame
	if err := json.Unmarshal(f, &fr); err != nil {
		return nil, err
	}
	var v any
	switch fr.Type {
	case name(ports.Offer{}):
		v = new(ports.Offer)
	case name(ports.AcceptAttempt{}):
		v = new(ports.AcceptAttempt)
	case name(ports.RejectAttempt{}):
		v = new(ports.RejectAttempt)
	case name(ports.LeaseRenew{}):
		v = new(ports.LeaseRenew)
	case name(ports.Result{}):
		v = new(ports.Result)
	case name(ports.Assignment{}):
		v = new(ports.Assignment)
	case name(ports.LeaseRevoked{}):
		v = new(ports.LeaseRevoked)
	case name(ports.ResultAck{}):
		v = new(ports.ResultAck)
	case name(ports.Cancel{}):
		v = new(ports.Cancel)
	default:
		return nil, fmt.Errorf("unknown message type %s", fr.Type)
	}
	if err := json.Unmarshal(fr.Body, v); err != nil {
		return nil, err
	}
	switch p := v.(type) {
	case *ports.Offer:
		return *p, nil
	case *ports.AcceptAttempt:
		return *p, nil
	case *ports.RejectAttempt:
		return *p, nil
	case *ports.LeaseRenew:
		return *p, nil
	case *ports.Result:
		return *p, nil
	case *ports.Assignment:
		return *p, nil
	case *ports.LeaseRevoked:
		return *p, nil
	case *ports.ResultAck:
		return *p, nil
	case *ports.Cancel:
		return *p, nil
	}
	return nil, fmt.Errorf("unreachable")
}

func TestAssignmentOverSerializedLink(t *testing.T) {
	coordtest.AssignmentSuite(t, newFake, func(*testing.T) (ports.PlatformLink, ports.WorkerLink) {
		pl, wl := inproc.Pair()
		return serialized[ports.PlatformMessage, ports.WorkerMessage]{Link: pl},
			serialized[ports.WorkerMessage, ports.PlatformMessage]{Link: wl}
	})
}
