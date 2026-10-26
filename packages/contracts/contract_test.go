package contracts_test

// Structural checks on the contract files. They need only the standard
// library and the YAML decoder: every schema parses, has a unique $id that
// matches its path, and every $ref (in schemas and in the OpenAPI document)
// points at a file that exists. Validating payloads against the schemas is the
// fixture suite (API-002), not this file.

import (
	"encoding/json"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const idBase = "https://amaru.invalid/contracts/v1/schemas/"

func loadSchemas(t *testing.T) map[string]map[string]any {
	t.Helper()
	out := map[string]map[string]any{}
	err := filepath.WalkDir("schemas", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".json") {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		var doc map[string]any
		if err := json.Unmarshal(b, &doc); err != nil {
			t.Errorf("%s: %v", p, err)
			return nil
		}
		out[filepath.ToSlash(p)] = doc
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Fatal("no schemas found")
	}
	return out
}

func refs(v any, f func(string)) {
	switch x := v.(type) {
	case map[string]any:
		for k, c := range x {
			if s, ok := c.(string); ok && k == "$ref" {
				f(s)
			} else {
				refs(c, f)
			}
		}
	case []any:
		for _, c := range x {
			refs(c, f)
		}
	}
}

func TestSchemasHaveUniqueIDsMatchingTheirPath(t *testing.T) {
	seen := map[string]string{}
	for p, doc := range loadSchemas(t) {
		id, _ := doc["$id"].(string)
		want := idBase + strings.TrimSuffix(strings.TrimPrefix(p, "schemas/"), ".json") + ".json"
		if id != want {
			t.Errorf("%s: $id %q, want %q", p, id, want)
		}
		if prev, dup := seen[id]; dup {
			t.Errorf("%s and %s share $id %q", p, prev, id)
		}
		seen[id] = p
		if doc["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
			t.Errorf("%s: $schema is %v, want Draft 2020-12", p, doc["$schema"])
		}
	}
}

func TestSchemaRefsResolveToFiles(t *testing.T) {
	schemas := loadSchemas(t)
	for p, doc := range schemas {
		refs(doc, func(ref string) {
			if strings.HasPrefix(ref, "#") {
				return
			}
			file, _, _ := strings.Cut(ref, "#")
			target := path.Join(path.Dir(p), file)
			if _, ok := schemas[target]; !ok {
				t.Errorf("%s: $ref %q points at %q, which does not exist", p, ref, target)
			}
		})
	}
}

func TestOpenAPIRefsResolveAndOperationsAreUnique(t *testing.T) {
	b, err := os.ReadFile("openapi/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["openapi"] != "3.1.0" {
		t.Errorf("openapi = %v, want 3.1.0", doc["openapi"])
	}
	refs(doc, func(ref string) {
		if strings.HasPrefix(ref, "#") {
			return
		}
		file, _, _ := strings.Cut(ref, "#")
		if _, err := os.Stat(filepath.Join("openapi", filepath.FromSlash(file))); err != nil {
			t.Errorf("$ref %q: %v", ref, err)
		}
	})
	ids := map[string]string{}
	for p, item := range doc["paths"].(map[string]any) {
		for m, op := range item.(map[string]any) {
			o, ok := op.(map[string]any)
			if !ok {
				continue
			}
			id, _ := o["operationId"].(string)
			if id == "" {
				t.Errorf("%s %s has no operationId", strings.ToUpper(m), p)
				continue
			}
			if prev, dup := ids[id]; dup {
				t.Errorf("operationId %q on %s %s and %s", id, m, p, prev)
			}
			ids[id] = m + " " + p
		}
	}
	for _, s := range doc["servers"].([]any) {
		u, _ := url.Parse(strings.NewReplacer("{port}", "0").Replace(s.(map[string]any)["url"].(string)))
		if !strings.HasPrefix(u.Path, "/v1") {
			t.Errorf("server URL %v is not under /v1", u)
		}
	}
}

// ADR-0047: no prototype namespace or product name remains in the contract
// files. PROVENANCE.md and the baseline image are the only exceptions.
func TestNoPrototypeNamesInContractFiles(t *testing.T) {
	banned := []string{"weave", "orkestra", "obsidian", "/v0"}
	for _, root := range []string{"schemas", "openapi", "proto"} {
		_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			b, _ := os.ReadFile(p)
			low := strings.ToLower(string(b))
			for _, w := range banned {
				if strings.Contains(low, w) {
					t.Errorf("%s contains %q", p, w)
				}
			}
			return nil
		})
	}
}
