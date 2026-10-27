package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func okSpec() CapabilitySpec {
	return CapabilitySpec{ID: "x.y", Title: "t",
		Inputs:  []PortSpec{{Name: "in", Kind: PortFile, MediaTypes: []string{"image/*"}, Required: true}},
		Outputs: []PortSpec{{Name: "out", Kind: PortFile}}}
}

func TestCapabilitySpecValidate(t *testing.T) {
	if err := okSpec().Validate(); err != nil {
		t.Fatal(err)
	}
	long := strings.Repeat("a", MaxTitle+1)
	manyPorts := func() []PortSpec {
		ps := make([]PortSpec, MaxPorts+1)
		for i := range ps {
			ps[i] = PortSpec{Name: "p" + strings.Repeat("a", i), Kind: PortText}
		}
		return ps
	}
	cases := map[string]func(*CapabilitySpec){
		"id without dot":      func(s *CapabilitySpec) { s.ID = "code" },
		"id uppercase":        func(s *CapabilitySpec) { s.ID = "Code.test" },
		"empty id":            func(s *CapabilitySpec) { s.ID = "" },
		"blank title":         func(s *CapabilitySpec) { s.Title = "  " },
		"long title":          func(s *CapabilitySpec) { s.Title = long },
		"long description":    func(s *CapabilitySpec) { s.Description = strings.Repeat("d", MaxDescription+1) },
		"nil inputs":          func(s *CapabilitySpec) { s.Inputs = nil },
		"nil outputs":         func(s *CapabilitySpec) { s.Outputs = nil },
		"too many inputs":     func(s *CapabilitySpec) { s.Inputs = manyPorts() },
		"bad port name":       func(s *CapabilitySpec) { s.Inputs[0].Name = "Bad-Name" },
		"duplicate port":      func(s *CapabilitySpec) { s.Inputs = append(s.Inputs, s.Inputs[0]) },
		"unknown kind":        func(s *CapabilitySpec) { s.Inputs[0].Kind = "blob" },
		"text output":         func(s *CapabilitySpec) { s.Outputs[0].Kind = PortText },
		"bad media type":      func(s *CapabilitySpec) { s.Inputs[0].MediaTypes = []string{"image"} },
		"enum without values": func(s *CapabilitySpec) { s.Inputs[0].Kind, s.Inputs[0].MediaTypes = PortEnum, nil },
		"too many enum values": func(s *CapabilitySpec) {
			s.Inputs[0].Kind, s.Inputs[0].Enum = PortEnum, make([]string, MaxEnumValues+1)
		},
		"too many media types":  func(s *CapabilitySpec) { s.Inputs[0].MediaTypes = make([]string, MaxMediaTypes+1) },
		"long port description": func(s *CapabilitySpec) { s.Inputs[0].Description = strings.Repeat("d", MaxDescription+1) },
	}
	for name, mutate := range cases {
		s := okSpec()
		mutate(&s)
		if err := s.Validate(); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: want ErrInvalid, got %v", name, err)
		}
	}
}

func TestBuiltinCatalogIsValid(t *testing.T) {
	c := Builtin() // panics if catalog.json is invalid
	got := []Capability{}
	for _, s := range c.List() {
		if err := s.Validate(); err != nil {
			t.Errorf("%s: %v", s.ID, err)
		}
		got = append(got, s.ID)
	}
	want := []Capability{"code.test", "mcp.call", "asset.upload", "doc.convert", "image.convert", "media.transcode"}
	if len(got) != len(want) {
		t.Fatalf("catalog ids %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("catalog ids %v, want %v", got, want)
		}
	}
	s, ok := c.Spec("doc.convert")
	if !ok {
		t.Fatal("doc.convert missing")
	}
	if p, ok := s.Input("format"); !ok || p.Kind != PortEnum || !p.Required {
		t.Errorf("doc.convert format: %+v", p)
	}
	if _, ok := s.Output("document"); !ok {
		t.Error("doc.convert output role document missing")
	}
	if _, ok := c.Spec("nope.nope"); ok {
		t.Error("unknown id must not resolve")
	}
}

// FEAT-CAP-001: specs are data, and adding a Capability changes no core interface.
func TestAddingACapabilityNeedsOnlyData(t *testing.T) {
	c := Builtin()
	before := len(c.List())
	extra := `[{"id":"lint.run","title":"Run a linter","inputs":[{"name":"path","kind":"text"}],"outputs":[{"name":"report","kind":"file","mediaTypes":["application/json"]}]}]`
	more, err := ParseCatalog(strings.NewReader(extra))
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range more.List() {
		if err := c.Add(s); err != nil {
			t.Fatal(err)
		}
	}
	got, ok := c.Spec("lint.run")
	if !ok || len(c.List()) != before+1 {
		t.Fatalf("lint.run not added: %v %d", ok, len(c.List()))
	}
	if _, ok := got.Output("report"); !ok {
		t.Error("added spec lost its output")
	}
	if len(Builtin().List()) != before {
		t.Error("Add on one catalog leaked into another")
	}
}

func TestCatalogRejectsDuplicatesAndInvalid(t *testing.T) {
	c := Builtin()
	dup := okSpec()
	dup.ID = "code.test"
	if err := c.Add(dup); !errors.Is(err, ErrInvalid) {
		t.Errorf("duplicate id: %v", err)
	}
	bad := okSpec()
	bad.Title = ""
	if _, err := NewCatalog(okSpec(), bad); !errors.Is(err, ErrInvalid) {
		t.Errorf("invalid spec: %v", err)
	}
	spec, _ := c.Spec("code.test")
	if spec.Title == "" || len(c.List()) != 6 {
		t.Error("catalog changed by rejected adds")
	}
}

func TestCatalogReturnsCopies(t *testing.T) {
	c := Builtin()
	s, _ := c.Spec("doc.convert")
	s.Inputs[1].Enum[0] = "tampered"
	s.Inputs[0].MediaTypes[0] = "x/y"
	again, _ := c.Spec("doc.convert")
	if again.Inputs[1].Enum[0] != "html" || again.Inputs[0].MediaTypes[0] != "text/*" {
		t.Error("mutating a returned spec changed the catalog")
	}
	l := c.List()
	l[0].Title = "tampered"
	if c.List()[0].Title == "tampered" {
		t.Error("mutating List changed the catalog")
	}
}

func TestParseCatalogIsStrict(t *testing.T) {
	for name, in := range map[string]string{
		"unknown field": `[{"id":"a.b","title":"t","inputs":[],"outputs":[],"vendor":"x"}]`,
		"trailing":      `[] []`,
		"not an array":  `{}`,
		"missing ports": `[{"id":"a.b","title":"t"}]`,
		"empty":         ``,
	} {
		if _, err := ParseCatalog(strings.NewReader(in)); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: want ErrInvalid, got %v", name, err)
		}
	}
	if c, err := ParseCatalog(bytes.NewReader([]byte(`[]`))); err != nil || len(c.List()) != 0 {
		t.Errorf("empty array: %v", err)
	}
}

func TestMediaTypesCompatible(t *testing.T) {
	for _, c := range []struct {
		produced, accepted []string
		want               bool
	}{
		{nil, []string{"image/png"}, true},
		{[]string{"image/png"}, nil, true},
		{[]string{"image/png"}, []string{"image/*"}, true},
		{[]string{"image/*"}, []string{"image/png"}, true},
		{[]string{"image/png"}, []string{"video/*"}, false},
		{[]string{"text/plain", "image/png"}, []string{"audio/*", "image/png"}, true},
		{[]string{"imagex/png"}, []string{"image/*"}, false},
	} {
		if got := MediaTypesCompatible(c.produced, c.accepted); got != c.want {
			t.Errorf("%v vs %v: got %v want %v", c.produced, c.accepted, got, c.want)
		}
	}
}

// The domain types must agree with the shared contract fixtures (packages/contracts).
func TestContractFixturesAgree(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..", "packages", "contracts", "fixtures", "api", "v1")
	read := func(rel string, v any) {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.DisallowUnknownFields()
		if err := dec.Decode(v); err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
	}
	var enumNoValues CapabilitySpec
	read("CapabilitySpec/invalid-enum-without-values.json", &enumNoValues)
	if err := enumNoValues.Validate(); !errors.Is(err, ErrInvalid) {
		t.Errorf("enum without values: %v", err)
	}
	var info struct {
		CapabilitySpec
		Builtin   bool            `json:"builtin"`
		Providers json.RawMessage `json:"providers"`
	}
	read("CapabilityInfo/invalid-text-output.json", &info)
	if err := info.CapabilitySpec.Validate(); !errors.Is(err, ErrInvalid) {
		t.Errorf("text output: %v", err)
	}
	var list []struct {
		CapabilitySpec
		Builtin   bool            `json:"builtin"`
		Providers json.RawMessage `json:"providers"`
	}
	read("CapabilityList/valid.json", &list)
	if len(list) == 0 {
		t.Fatal("empty fixture list")
	}
	for _, e := range list {
		if err := e.CapabilitySpec.Validate(); err != nil {
			t.Errorf("%s: %v", e.ID, err)
		}
	}
}
