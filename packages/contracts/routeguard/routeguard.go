// Package routeguard compares the routes a server registers with the routes
// the OpenAPI contract declares, so a route can never exist without being in
// the contract (FEAT-API-001) and the contract can never promise a route that
// is not served.
//
// It is a test utility: the Platform (M3) feeds it the patterns its router
// registered and calls AssertMatch from a test. It starts no server and does
// not know any router; it only reads the contract and compares two lists.
package routeguard

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Route is one HTTP method on one path. Path is absolute, including the
// contract's server base path (for example /v1/projects/{}), with every path
// parameter normalized to {} so that parameter names do not matter.
type Route struct {
	Method string
	Path   string
}

func (r Route) String() string { return r.Method + " " + r.Path }

var methods = map[string]bool{"get": true, "put": true, "post": true, "delete": true, "options": true, "head": true, "patch": true, "trace": true}

// FromOpenAPI reads an OpenAPI 3.x document and returns its routes, sorted.
// The base path is the path of the first server URL (http://host:{port}/v1
// gives /v1). A document without servers has an empty base path.
func FromOpenAPI(r io.Reader) ([]Route, error) {
	var doc struct {
		Servers []struct {
			URL string `yaml:"url"`
		} `yaml:"servers"`
		Paths map[string]map[string]yaml.Node `yaml:"paths"`
	}
	if err := yaml.NewDecoder(r).Decode(&doc); err != nil {
		return nil, fmt.Errorf("routeguard: decode OpenAPI: %w", err)
	}
	if len(doc.Paths) == 0 {
		return nil, fmt.Errorf("routeguard: OpenAPI document has no paths")
	}
	base := ""
	if len(doc.Servers) > 0 {
		base = serverBasePath(doc.Servers[0].URL)
	}
	var out []Route
	for p, item := range doc.Paths {
		if !strings.HasPrefix(p, "/") {
			return nil, fmt.Errorf("routeguard: path %q does not start with /", p)
		}
		for m := range item {
			if methods[strings.ToLower(m)] {
				out = append(out, Route{Method: strings.ToUpper(m), Path: normalize(base + p)})
			}
		}
	}
	sortRoutes(out)
	return out, nil
}

// FromPatterns parses registered routes written as Go 1.22 ServeMux patterns,
// "METHOD /path/{param}". A pattern without a method is an error: a route that
// answers every method cannot be compared with the contract. A "{$}" or
// "{name...}" wildcard is an error for the same reason.
func FromPatterns(patterns []string) ([]Route, error) {
	var out []Route
	for _, s := range patterns {
		method, path, ok := strings.Cut(strings.TrimSpace(s), " ")
		path = strings.TrimSpace(path)
		if !ok || method != strings.ToUpper(method) || !strings.HasPrefix(path, "/") {
			return nil, fmt.Errorf("routeguard: pattern %q is not \"METHOD /path\"", s)
		}
		if strings.Contains(path, "...}") || strings.Contains(path, "{$}") {
			return nil, fmt.Errorf("routeguard: pattern %q uses a wildcard that the contract cannot declare", s)
		}
		out = append(out, Route{Method: method, Path: normalize(path)})
	}
	sortRoutes(out)
	return out, nil
}

// Report is the difference between the contract and the registered routes.
type Report struct {
	// Undocumented are registered routes that the contract does not declare
	// (the prototype's /work-packages/{id}/autoloop was one).
	Undocumented []Route
	// Unserved are contract routes that no registered route answers.
	Unserved []Route
}

// OK reports whether the two sets are equal.
func (r Report) OK() bool { return len(r.Undocumented) == 0 && len(r.Unserved) == 0 }

func (r Report) String() string {
	var b strings.Builder
	for _, x := range r.Undocumented {
		fmt.Fprintf(&b, "registered but not in the contract: %s\n", x)
	}
	for _, x := range r.Unserved {
		fmt.Fprintf(&b, "in the contract but not registered: %s\n", x)
	}
	return b.String()
}

// Compare returns the two-way difference between contract and registered.
func Compare(contract, registered []Route) Report {
	c := set(contract)
	g := set(registered)
	var rep Report
	for k, r := range g {
		if _, ok := c[k]; !ok {
			rep.Undocumented = append(rep.Undocumented, r)
		}
	}
	for k, r := range c {
		if _, ok := g[k]; !ok {
			rep.Unserved = append(rep.Unserved, r)
		}
	}
	sortRoutes(rep.Undocumented)
	sortRoutes(rep.Unserved)
	return rep
}

// TB is the part of testing.TB that AssertMatch uses.
type TB interface {
	Helper()
	Errorf(format string, args ...any)
}

// AssertMatch fails the test when the registered patterns and the contract
// differ in either direction. allowUnserved lists contract routes that are
// deliberately not registered yet (for example a route of a later milestone);
// each entry must still exist in the contract, so the list cannot rot.
func AssertMatch(t TB, contract io.Reader, registered []string, allowUnserved ...Route) {
	t.Helper()
	cr, err := FromOpenAPI(contract)
	if err != nil {
		t.Errorf("%v", err)
		return
	}
	rr, err := FromPatterns(registered)
	if err != nil {
		t.Errorf("%v", err)
		return
	}
	rep := Compare(cr, rr)
	allowed := set(normalized(allowUnserved))
	known := set(cr)
	for k, r := range allowed {
		if _, ok := known[k]; !ok {
			t.Errorf("routeguard: allowed-unserved route %s is not in the contract", r)
		}
	}
	var unserved []Route
	for _, r := range rep.Unserved {
		if _, ok := allowed[key(r)]; !ok {
			unserved = append(unserved, r)
		}
	}
	rep.Unserved = unserved
	if !rep.OK() {
		t.Errorf("route drift between server and contract:\n%s", rep)
	}
}

func serverBasePath(url string) string {
	rest := url
	if i := strings.Index(rest, "://"); i >= 0 {
		rest = rest[i+3:]
	}
	i := strings.Index(rest, "/")
	if i < 0 {
		return ""
	}
	return strings.TrimRight(rest[i:], "/")
}

// normalize replaces every {name} segment by {} and drops a trailing slash.
func normalize(p string) string {
	segs := strings.Split(p, "/")
	for i, s := range segs {
		if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") {
			segs[i] = "{}"
		}
	}
	out := strings.Join(segs, "/")
	if len(out) > 1 {
		out = strings.TrimRight(out, "/")
	}
	return out
}

func normalized(rs []Route) []Route {
	out := make([]Route, len(rs))
	for i, r := range rs {
		out[i] = Route{Method: strings.ToUpper(r.Method), Path: normalize(r.Path)}
	}
	return out
}

func key(r Route) string { return r.Method + " " + r.Path }

func set(rs []Route) map[string]Route {
	m := make(map[string]Route, len(rs))
	for _, r := range rs {
		m[key(r)] = r
	}
	return m
}

func sortRoutes(rs []Route) {
	sort.Slice(rs, func(i, j int) bool {
		if rs[i].Path != rs[j].Path {
			return rs[i].Path < rs[j].Path
		}
		return rs[i].Method < rs[j].Method
	})
}
