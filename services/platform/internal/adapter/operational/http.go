package operational

import (
	"bufio"
	"crypto/rand"
	"net"
	"net/http"
)

// HTTP must wrap auth, so rejected requests and launch exchanges are captured.
// onError is required and receives only safe logger errors; it must not log raw
// requests or retry commands. Neither request nor response credentials are read.
func (l *Logger) HTTP(instanceID string, next http.Handler, onError func(error)) http.Handler {
	if instanceID == "" || next == nil || onError == nil {
		panic("operational HTTP requires instance, handler and error callback")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := WithIdentity(r.Context(), Identity{InstanceID: instanceID, CorrelationID: rand.Text()})
		rw := &response{ResponseWriter: w, status: http.StatusOK}
		outcome := "panicked"
		defer func() {
			recovered := recover()
			if recovered != nil {
				rw.status = 0
			}
			if err := l.Write(ctx, Record{Operation: "http.request", Outcome: outcome, Status: rw.status}); err != nil {
				onError(err)
			}
			// Prevent net/http from printing a raw panic value or stack to ErrorLog.
			if recovered != nil {
				panic(http.ErrAbortHandler)
			}
		}()
		next.ServeHTTP(rw, r.WithContext(ctx))
		outcome = "succeeded"
		if rw.status >= 400 {
			outcome = "rejected"
		}
	})
}

type response struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (w *response) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *response) WriteHeader(status int) {
	if status >= 100 && status < 200 {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	if !w.wrote {
		w.status = status
		w.wrote = true
		w.ResponseWriter.WriteHeader(status)
	}
}
func (w *response) Write(p []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(p)
}

// Preserve WebSocket hijacking and report the upgrade separately from HTTP 200.
func (w *response) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, rw, err := http.NewResponseController(w.ResponseWriter).Hijack()
	if err == nil {
		w.status = http.StatusSwitchingProtocols
		w.wrote = true
	}
	return conn, rw, err
}
func (w *response) FlushError() error {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	return http.NewResponseController(w.ResponseWriter).Flush()
}
