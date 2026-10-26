package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/rluders/amaru/spikes/wp-m1-02/internal/bus"
)

// cmdNATS measures the embedded JetStream footprint and the file-handling
// behaviour that matters on Windows (A9: footprint, file locking): start, publish,
// graceful stop, delete the whole store directory, start again from scratch.
func cmdNATS(args []string) error {
	ctx := context.Background()
	dir, err := os.MkdirTemp("", "spike-nats-")
	if err != nil {
		return err
	}
	store := filepath.Join(dir, "jetstream")
	exe, _ := os.Executable()
	rec := map[string]any{"goos": runtime.GOOS, "goarch": runtime.GOARCH, "spike_binary_bytes": fileSize(exe)}

	var before runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	t0 := time.Now()
	e, err := bus.Start(ctx, store, 2*time.Minute)
	if err != nil {
		return err
	}
	rec["start_ms"] = time.Since(t0).Milliseconds()
	payload := make([]byte, 1024)
	for i := 0; i < 10000; i++ {
		if _, err := e.JS.PublishMsg(ctx, &nats.Msg{Subject: bus.SubjectBase + ".p", Data: payload}, jetstream.WithMsgID(fmt.Sprint("m", i))); err != nil {
			return err
		}
	}
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	rss, _ := rssKB([]int{os.Getpid()})
	rec["go_heap_delta_kb"] = (int64(after.HeapAlloc) - int64(before.HeapAlloc)) / 1024
	rec["process_rss_kb_with_10k_msgs"] = rss
	rec["store_bytes_10k_msgs_1KiB"] = dirSize(store)
	t1 := time.Now()
	e.Stop()
	rec["stop_ms"] = time.Since(t1).Milliseconds()

	// restart reads the same store
	e, err = bus.Start(ctx, store, 2*time.Minute)
	if err != nil {
		rec["restart_error"] = err.Error()
	} else {
		s, _ := e.JS.Stream(ctx, bus.StreamName)
		info, _ := s.Info(ctx)
		rec["msgs_after_restart"] = info.State.Msgs
		e.Stop()
	}
	// file-locking check: after a graceful stop the whole store must be deletable
	rmErr := os.RemoveAll(store)
	rec["remove_store_after_stop_error"] = errStr(rmErr)
	rec["store_removed"] = !exists(store)
	if rmErr == nil {
		e, err = bus.Start(ctx, store, 2*time.Minute)
		rec["fresh_start_after_wipe_error"] = errStr(err)
		if err == nil {
			e.Stop()
		}
	}
	os.RemoveAll(dir)
	b, _ := json.MarshalIndent(rec, "", "  ")
	fmt.Println(string(b))
	if rmErr != nil {
		return fmt.Errorf("store not removable after stop: %w", rmErr)
	}
	return nil
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }
