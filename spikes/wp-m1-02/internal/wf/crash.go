package wf

import (
	"os"
	"strconv"
	"strings"
	"sync"
)

// Crasher is the crash-injection port. Hit is called at every named boundary.
type Crasher interface{ Hit(point string) }

// NoCrash never crashes.
type NoCrash struct{}

func (NoCrash) Hit(string) {}

// EnvCrash terminates the process like kill -9 (no deferred functions, no
// graceful close of PostgreSQL or NATS) at the nth hit of a named point.
// Spec: "<point>", "<point>#<n>", and a ":hang" suffix to block the calling
// goroutine forever instead (a hung Worker, not a dead process).
type EnvCrash struct {
	point string
	nth   int
	seen  int
	hang  bool
	mu    sync.Mutex
}

func ParseCrash(spec string) *EnvCrash {
	c := &EnvCrash{nth: 1}
	if spec == "" {
		return c
	}
	if strings.HasSuffix(spec, ":hang") {
		c.hang = true
		spec = strings.TrimSuffix(spec, ":hang")
	}
	p, n, ok := strings.Cut(spec, "#")
	c.point = p
	if ok {
		c.nth, _ = strconv.Atoi(n)
	}
	return c
}

func (c *EnvCrash) Hit(point string) {
	if c.point != point {
		return
	}
	c.mu.Lock()
	c.seen++
	hit := c.seen == c.nth
	c.mu.Unlock()
	if hit {
		if c.hang {
			os.Stderr.WriteString("HANG-INJECTED " + point + "\n")
			select {}
		}
		os.Stderr.WriteString("CRASH-INJECTED " + point + "\n")
		os.Exit(137)
	}
}
