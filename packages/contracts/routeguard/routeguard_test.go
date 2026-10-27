package routeguard

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

const contractPath = "../openapi/openapi.yaml"

type recorder struct{ errs []string }

func (r *recorder) Helper() {}
func (r *recorder) Errorf(format string, args ...any) {
	r.errs = append(r.errs, strings.TrimSpace(fmt.Sprintf(format, args...)))
}

func openContract(t *testing.T) *os.File {
	t.Helper()
	f, err := os.Open(contractPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func contractRoutes(t *testing.T) []Route {
	t.Helper()
	rs, err := FromOpenAPI(openContract(t))
	if err != nil {
		t.Fatal(err)
	}
	return rs
}

func patterns(rs []Route) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.Method + " " + r.Path
	}
	return out
}

func TestContractRoutesAreUnderV1AndIncludeTheDeliveryLoop(t *testing.T) {
	rs := contractRoutes(t)
	if len(rs) == 0 {
		t.Fatal("no routes")
	}
	have := set(rs)
	for _, r := range rs {
		if !strings.HasPrefix(r.Path, "/v1/") {
			t.Errorf("%s is not under /v1", r)
		}
	}
	if _, ok := have["GET /v1/projects/{}/work-packages/{}/delivery-loop"]; !ok {
		t.Error("the delivery-loop route is missing from the contract")
	}
	for k := range have {
		if strings.Contains(k, "autoloop") {
			t.Errorf("prototype route name survived: %s", k)
		}
	}
}

func TestExactMatchPasses(t *testing.T) {
	var rec recorder
	AssertMatch(&rec, openContract(t), patterns(contractRoutes(t)))
	if len(rec.errs) != 0 {
		t.Fatalf("unexpected failure: %v", rec.errs)
	}
}

// The prototype registered GET /work-packages/{id}/autoloop without declaring
// it. The guard must fail on that shape of drift.
func TestRegisteredButUndeclaredRouteFails(t *testing.T) {
	var rec recorder
	reg := append(patterns(contractRoutes(t)), "GET /v1/projects/{projectId}/work-packages/{id}/autoloop")
	AssertMatch(&rec, openContract(t), reg)
	if len(rec.errs) != 1 || !strings.Contains(rec.errs[0], "registered but not in the contract: GET /v1/projects/{}/work-packages/{}/autoloop") {
		t.Fatalf("drift not reported as expected: %v", rec.errs)
	}
}

func TestDeclaredButUnregisteredRouteFails(t *testing.T) {
	var rec recorder
	reg := patterns(contractRoutes(t))
	reg = reg[1:]
	AssertMatch(&rec, openContract(t), reg)
	if len(rec.errs) != 1 || !strings.Contains(rec.errs[0], "in the contract but not registered") {
		t.Fatalf("missing route not reported: %v", rec.errs)
	}
}

func TestMethodMattersAndParameterNamesDoNot(t *testing.T) {
	c := []Route{{"GET", "/v1/a/{}"}}
	if rep := Compare(c, mustPatterns(t, "GET /v1/a/{anything}")); !rep.OK() {
		t.Fatalf("parameter names must not matter: %s", rep)
	}
	if rep := Compare(c, mustPatterns(t, "POST /v1/a/{id}")); rep.OK() {
		t.Fatal("a different method must be drift")
	}
}

func TestAllowUnservedMustExistInContract(t *testing.T) {
	reg := patterns(contractRoutes(t))
	first := contractRoutes(t)[0]

	var rec recorder
	AssertMatch(&rec, openContract(t), reg[1:], first)
	if len(rec.errs) != 0 {
		t.Fatalf("an allowed unserved route must pass: %v", rec.errs)
	}

	rec = recorder{}
	AssertMatch(&rec, openContract(t), reg, Route{"GET", "/v1/not/in/contract"})
	if len(rec.errs) != 1 || !strings.Contains(rec.errs[0], "not in the contract") {
		t.Fatalf("a stale allowance must fail: %v", rec.errs)
	}
}

func TestPatternsWithoutAMethodOrWithWildcardsAreRejected(t *testing.T) {
	for _, p := range []string{"/v1/a", "get /v1/a", "GET /v1/a/{rest...}", "GET /v1/{$}", "GET v1/a"} {
		if _, err := FromPatterns([]string{p}); err == nil {
			t.Errorf("%q must be rejected", p)
		}
	}
}

func TestOpenAPIWithoutPathsIsRejected(t *testing.T) {
	if _, err := FromOpenAPI(strings.NewReader("openapi: 3.1.0\n")); err == nil {
		t.Fatal("a document without paths must be rejected")
	}
}

func TestServerBasePath(t *testing.T) {
	for in, want := range map[string]string{
		"http://127.0.0.1:{port}/v1": "/v1",
		"https://example.test/":      "",
		"https://example.test":       "",
		"/api/v2/":                   "/api/v2",
	} {
		if got := serverBasePath(in); got != want {
			t.Errorf("serverBasePath(%q) = %q, want %q", in, got, want)
		}
	}
}

func mustPatterns(t *testing.T, p ...string) []Route {
	t.Helper()
	rs, err := FromPatterns(p)
	if err != nil {
		t.Fatal(err)
	}
	return rs
}
