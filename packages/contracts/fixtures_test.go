package contracts_test

// API-002: the shared fixture corpus (fixtures/manifest.json) validated
// against the schemas with a Draft 2020-12 validator. The TypeScript suite in
// test/ reads the same manifest; `make contracts-parity` compares the
// per-fixture outcomes of both.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

// Worker fixtures are protobuf JSON (the canonical JSON mapping of the
// message) validated against the message descriptors built from proto/ by
// `buf build`, so they follow the sources and not the committed baseline.
const protoPrefix = "proto:"

func workerMessage(t *testing.T, name string) protoreflect.MessageDescriptor {
	t.Helper()
	out := filepath.Join(t.TempDir(), "worker.binpb")
	if b, err := exec.Command("buf", "build", "-o", out).CombinedOutput(); err != nil {
		t.Fatalf("buf build: %v\n%s", err, b)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var set descriptorpb.FileDescriptorSet
	if err := proto.Unmarshal(raw, &set); err != nil {
		t.Fatal(err)
	}
	files, err := protodesc.NewFiles(&set)
	if err != nil {
		t.Fatal(err)
	}
	d, err := files.FindDescriptorByName(protoreflect.FullName(name))
	if err != nil {
		t.Fatal(err)
	}
	return d.(protoreflect.MessageDescriptor)
}

type manifest struct {
	Version int     `json:"version"`
	Entries []entry `json:"entries"`
}

type entry struct {
	File   string `json:"file"`
	Schema string `json:"schema"`
	Kind   string `json:"kind"`
	Expect string `json:"expect"`
	Error  string `json:"error"`
	Note   string `json:"note"`
	Source string `json:"source"`
}

var kindOutcome = map[string]string{
	"valid":          "valid",
	"forward-compat": "valid",
	"invalid":        "invalid",
	"malformed":      "malformed",
}

func loadManifest(t *testing.T) manifest {
	t.Helper()
	b, err := os.ReadFile("fixtures/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var m manifest
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		t.Fatalf("fixtures/manifest.json: %v", err)
	}
	if m.Version != 1 || len(m.Entries) == 0 {
		t.Fatalf("fixtures/manifest.json: version %d, %d entries", m.Version, len(m.Entries))
	}
	return m
}

// parse reads exactly one JSON document. Anything else is malformed.
func parse(b []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("data after the JSON document")
	}
	return v, nil
}

func compiler(t *testing.T) *jsonschema.Compiler {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	for p, doc := range loadSchemas(t) {
		id, _ := doc["$id"].(string)
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		v, err := parse(b)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		if err := c.AddResource(id, v); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
	}
	return c
}

// Error ids are "keyword@/instance/path". The two validators word composition
// failures differently (allOf, if/then, not, oneOf, anyOf), so those all
// normalize to the keyword "condition"; "$ref" and schema wrappers carry no
// information and are skipped. unevaluatedProperties is reported by this
// library as a false schema on the offending property, so it is mapped back
// to the object that has the property.
var conditionKinds = map[string]bool{"allOf": true, "anyOf": true, "oneOf": true, "not": true, "if": true, "then": true, "else": true}

// errorIDs lists every error id in a validation error tree.
func errorIDs(e *jsonschema.ValidationError, out map[string]bool) {
	name := reflect.TypeOf(e.ErrorKind).Elem().Name()
	kw := strings.ToLower(name[:1]) + name[1:]
	loc := e.InstanceLocation
	switch {
	case kw == "schema" || kw == "reference":
		kw = ""
	case kw == "falseSchema" && len(loc) > 0:
		kw, loc = "unevaluatedProperties", loc[:len(loc)-1]
	case conditionKinds[kw]:
		kw = "condition"
	}
	if kw != "" {
		out[kw+"@/"+strings.Join(loc, "/")] = true
	}
	for _, c := range e.Causes {
		errorIDs(c, out)
	}
}

// outcome validates one fixture. For an invalid one it also returns the error ids.
func outcome(c *jsonschema.Compiler, w protoreflect.MessageDescriptor, e entry) (string, map[string]bool, error) {
	b, err := os.ReadFile(filepath.Join("fixtures", filepath.FromSlash(e.File)))
	if err != nil {
		return "", nil, err
	}
	v, err := parse(b)
	if err != nil {
		return "malformed", nil, nil
	}
	if strings.HasPrefix(e.Schema, protoPrefix) {
		// The error id of a decode failure is "decode@/": the two protobuf
		// libraries do not share error vocabulary.
		if err := protojson.Unmarshal(b, dynamicpb.NewMessage(w)); err != nil {
			return "invalid", map[string]bool{"decode@/": true}, nil
		}
		return "valid", nil, nil
	}
	sch, err := c.Compile(idBase + e.Schema)
	if err != nil {
		return "", nil, fmt.Errorf("compile %s: %w", e.Schema, err)
	}
	err = sch.Validate(v)
	if err == nil {
		return "valid", nil, nil
	}
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return "", nil, err
	}
	ids := map[string]bool{}
	errorIDs(ve, ids)
	return "invalid", ids, nil
}

func TestFixturesMatchTheirDeclaredOutcome(t *testing.T) {
	c := compiler(t)
	w := workerMessage(t, "amaru.worker.v1.Envelope")
	got := map[string]string{}
	for _, e := range loadManifest(t).Entries {
		t.Run(e.File, func(t *testing.T) {
			out, ids, err := outcome(c, w, e)
			if err != nil {
				t.Fatal(err)
			}
			got[e.File] = out
			if out != e.Expect {
				t.Errorf("outcome %s, manifest expects %s", out, e.Expect)
			}
			if e.Expect == "invalid" && out == "invalid" && !ids[e.Error] {
				t.Errorf("error id %q not reported; got %v", e.Error, sortedKeys(ids))
			}
		})
	}
	if p := os.Getenv("CONTRACTS_OUTCOMES_OUT"); p != "" {
		writeOutcomes(t, p, got)
	}
}

func writeOutcomes(t *testing.T, path string, got map[string]string) {
	t.Helper()
	files := make([]string, 0, len(got))
	for f := range got {
		files = append(files, f)
	}
	sort.Strings(files)
	var b strings.Builder
	for _, f := range files {
		fmt.Fprintf(&b, "%s\t%s\n", f, got[f])
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// The manifest and the directory must describe the same set of files, and
// every entry must be well formed.
func TestManifestIsCompleteAndConsistent(t *testing.T) {
	m := loadManifest(t)
	schemas := loadSchemas(t)
	listed := map[string]bool{}
	kinds := map[string]int{}
	for _, e := range m.Entries {
		if listed[e.File] {
			t.Errorf("%s is listed twice", e.File)
		}
		listed[e.File] = true
		kinds[e.Kind]++
		if want, ok := kindOutcome[e.Kind]; !ok || e.Expect != want {
			t.Errorf("%s: kind %q with expect %q", e.File, e.Kind, e.Expect)
		}
		if _, ok := schemas["schemas/"+e.Schema]; !ok && e.Schema != protoPrefix+"amaru.worker.v1.Envelope" {
			t.Errorf("%s: schema %q does not exist", e.File, e.Schema)
		}
		if (e.Expect == "invalid") != (e.Error != "") {
			t.Errorf("%s: an error id is required for invalid fixtures and only for them", e.File)
		}
		if e.Error != "" && !regexp.MustCompile(`^[A-Za-z]+@/[^ ]*$`).MatchString(e.Error) {
			t.Errorf("%s: error id %q is not keyword@/instance/path", e.File, e.Error)
		}
		if e.Kind != "valid" && e.Kind != "invalid" && e.Kind != "malformed" && e.Note == "" {
			t.Errorf("%s: a %s fixture needs a note", e.File, e.Kind)
		}
	}
	for _, k := range []string{"valid", "invalid", "forward-compat", "malformed"} {
		if kinds[k] == 0 {
			t.Errorf("the corpus has no %s fixture", k)
		}
	}
	err := filepath.WalkDir("fixtures", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Base(p) == "manifest.json" {
			return err
		}
		rel := filepath.ToSlash(strings.TrimPrefix(p, "fixtures"+string(filepath.Separator)))
		if !listed[rel] {
			t.Errorf("%s is not in the manifest", rel)
		}
		delete(listed, rel)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for f := range listed {
		t.Errorf("%s is in the manifest but not on disk", f)
	}
	lower := map[string]string{}
	for _, e := range m.Entries {
		if prev, dup := lower[strings.ToLower(e.File)]; dup && prev != e.File {
			t.Errorf("%s and %s differ only by case (breaks case-insensitive checkouts)", e.File, prev)
		}
		lower[strings.ToLower(e.File)] = e.File
	}
}

// Every shape the contract carries for these areas has at least one valid and
// one invalid fixture, so a schema cannot be added without a test.
func TestEveryAreaIsCovered(t *testing.T) {
	m := loadManifest(t)
	have := map[string]map[string]bool{}
	for _, e := range m.Entries {
		if have[e.Schema] == nil {
			have[e.Schema] = map[string]bool{}
		}
		have[e.Schema][kindOutcome[e.Kind]] = true
	}
	for _, s := range []string{
		"Approval.json", "DecideApproval.json", "Principal.json", "PolicyRequest.json", "PolicyDecision.json", "Grant.json",
		"UsageReport.json", "UsageRecord.json", "StepExecution.json",
		"events/Event.json", "events/EventList.json", "events/Frame.json", "events/ResyncRequired.json",
		"events/ApprovalEvent.json", "events/PolicyEvaluated.json", "events/GrantChanged.json", "events/UsageRecorded.json", "events/ResultRejectedStale.json",
		"provider/Message.json", "provider/Request.json", protoPrefix + "amaru.worker.v1.Envelope",
	} {
		if !have[s]["valid"] || !have[s]["invalid"] {
			t.Errorf("%s needs at least one valid and one invalid fixture (has %v)", s, have[s])
		}
	}
}

// Fixtures are shared by every consumer, so they must carry no secret values
// and none of the prototype's names.
func TestFixturesHaveNoSecretsOrPrototypeNames(t *testing.T) {
	secret := regexp.MustCompile(`(?i)-----BEGIN [A-Z ]*PRIVATE KEY-----|ghp_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}|sk-[A-Za-z0-9]{20,}|AKIA[0-9A-Z]{16}|xox[abpr]-[A-Za-z0-9-]{10,}|eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.|(password|passwd|secret|token|apikey|api_key)["']?\s*[:=]\s*["'][^"'\s]{8,}`)
	banned := []string{"weave", "orkestra", "obsidian", "/v0"}
	_ = filepath.WalkDir("fixtures", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, _ := os.ReadFile(p)
		if m := secret.Find(b); m != nil {
			t.Errorf("%s looks like it contains a secret value", p)
		}
		low := strings.ToLower(string(b) + p)
		for _, w := range banned {
			if strings.Contains(low, w) {
				t.Errorf("%s contains %q", p, w)
			}
		}
		return nil
	})
}
