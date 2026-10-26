// Package bus embeds nats-server with JetStream in the calling process
// (ADR-0016): no listener, in-process connection, store under a data directory.
package bus

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	StreamName   = "ASSIGN"
	SubjectBase  = "amaru.org1.assign"
	SubjectMatch = SubjectBase + ".>"
	Consumer     = "workers"
)

// Embedded is a running in-process JetStream server plus its client.
type Embedded struct {
	Srv *server.Server
	NC  *nats.Conn
	JS  jetstream.JetStream
}

// Start boots the server. storeDir persists across Start/Stop and process
// restarts. dupWindow is the JetStream duplicate-tracking window of the stream.
func Start(ctx context.Context, storeDir string, dupWindow time.Duration) (*Embedded, error) {
	s, err := server.NewServer(&server.Options{
		ServerName: "amaru-embedded",
		DontListen: true,
		JetStream:  true,
		StoreDir:   storeDir,
		NoLog:      true,
		NoSigs:     true,
	})
	if err != nil {
		return nil, err
	}
	go s.Start()
	if !s.ReadyForConnections(10 * time.Second) {
		s.Shutdown()
		return nil, fmt.Errorf("embedded nats not ready")
	}
	nc, err := nats.Connect("", nats.InProcessServer(s), nats.NoReconnect())
	if err != nil {
		s.Shutdown()
		return nil, err
	}
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		s.Shutdown()
		return nil, err
	}
	e := &Embedded{Srv: s, NC: nc, JS: js}
	if _, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:       StreamName,
		Subjects:   []string{SubjectMatch},
		Retention:  jetstream.WorkQueuePolicy,
		Storage:    jetstream.FileStorage,
		Duplicates: dupWindow,
	}); err != nil {
		e.Stop()
		return nil, err
	}
	return e, nil
}

// Consumer returns the durable pull consumer of the work queue.
func (e *Embedded) Consumer(ctx context.Context, ackWait time.Duration, maxDeliver int) (jetstream.Consumer, error) {
	return e.JS.CreateOrUpdateConsumer(ctx, StreamName, jetstream.ConsumerConfig{
		Durable:       Consumer,
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       ackWait,
		MaxDeliver:    maxDeliver,
		FilterSubject: SubjectMatch,
	})
}

// Stop shuts the server down and waits for it, as a graceful Platform stop does.
func (e *Embedded) Stop() {
	e.NC.Close()
	e.Srv.Shutdown()
	e.Srv.WaitForShutdown()
}
