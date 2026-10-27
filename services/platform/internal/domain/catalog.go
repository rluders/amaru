package domain

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
)

// Catalog is a set of validated CapabilitySpecs keyed by ID. The built-in set is data
// (catalog.json); further Capabilities are added with Add, from a plugin or Provider, without
// any change to this type or to its callers.
type Catalog struct {
	specs map[Capability]CapabilitySpec
	order []Capability
}

// NewCatalog returns a Catalog holding specs. It fails on an invalid spec or a duplicate ID.
func NewCatalog(specs ...CapabilitySpec) (*Catalog, error) {
	c := &Catalog{specs: map[Capability]CapabilitySpec{}}
	for _, s := range specs {
		if err := c.Add(s); err != nil {
			return nil, err
		}
	}
	return c, nil
}

// Add validates spec and adds it. An existing ID is never replaced.
func (c *Catalog) Add(spec CapabilitySpec) error {
	if err := spec.Validate(); err != nil {
		return err
	}
	if _, dup := c.specs[spec.ID]; dup {
		return invalid("capability %s is already in the catalog", spec.ID)
	}
	c.specs[spec.ID] = spec.clone()
	c.order = append(c.order, spec.ID)
	return nil
}

// Spec returns a copy of the spec of id.
func (c *Catalog) Spec(id Capability) (CapabilitySpec, bool) {
	s, ok := c.specs[id]
	return s.clone(), ok
}

// List returns copies of all specs in insertion order.
func (c *Catalog) List() []CapabilitySpec {
	out := make([]CapabilitySpec, 0, len(c.order))
	for _, id := range c.order {
		out = append(out, c.specs[id].clone())
	}
	return out
}

// ParseCatalog reads a JSON array of CapabilitySpecs. Unknown fields and trailing content are
// errors, so a typo in a data file cannot silently drop a constraint.
func ParseCatalog(r io.Reader) (*Catalog, error) {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	var specs []CapabilitySpec
	if err := dec.Decode(&specs); err != nil {
		return nil, invalid("catalog: %v", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, invalid("catalog: trailing content after the array")
	}
	return NewCatalog(specs...)
}

//go:embed catalog.json
var builtinJSON []byte

// Builtin returns a new Catalog of the built-in Capabilities. Each call returns an independent
// value, so adding to one never affects another.
func Builtin() *Catalog {
	c, err := ParseCatalog(bytes.NewReader(builtinJSON))
	if err != nil {
		panic(fmt.Sprintf("domain: built-in catalog.json is invalid: %v", err)) // guarded by TestBuiltinCatalogIsValid
	}
	return c
}
