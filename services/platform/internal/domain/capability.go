package domain

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Limits mirror packages/contracts (CapabilitySpec, PortSpec).
const (
	MaxTitle       = 200
	MaxDescription = 1000
	MaxPorts       = 20
	MaxMediaTypes  = 20
	MaxEnumValues  = 50
	MaxPortName    = 120
)

var (
	capabilityPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)
	portNamePattern   = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	mediaTypePattern  = regexp.MustCompile(`^[a-z]+/([a-z0-9][a-z0-9.+-]*|\*)$`)
)

// Capability identifies a vendor-neutral operation, e.g. "code.test". It is data, not an enum:
// a new Capability is a new CapabilitySpec, never a new type or interface.
type Capability string

// Valid reports whether c is a dotted lowercase identifier.
func (c Capability) Valid() bool { return capabilityPattern.MatchString(string(c)) }

// PortKind is the kind of value a port carries.
type PortKind string

const (
	PortText   PortKind = "text"
	PortNumber PortKind = "number"
	PortBool   PortKind = "bool"
	PortEnum   PortKind = "enum"
	PortFile   PortKind = "file" // a project-relative path
)

func (k PortKind) Valid() bool {
	switch k {
	case PortText, PortNumber, PortBool, PortEnum, PortFile:
		return true
	}
	return false
}

// PortSpec describes one named input or output of a Capability. Output ports are always files;
// their Name is the output role (e.g. "report").
type PortSpec struct {
	Name        string   `json:"name"`
	Kind        PortKind `json:"kind"`
	MediaTypes  []string `json:"mediaTypes,omitempty"` // file ports; empty = any; "model/*" wildcards allowed
	Required    bool     `json:"required,omitempty"`
	Enum        []string `json:"enum,omitempty"` // enum ports
	Description string   `json:"description,omitempty"`
}

// CapabilitySpec is the vendor-neutral contract of a Capability: the named inputs it takes and
// the output roles it produces. Providers implement it; Workflows reference only the ID.
type CapabilitySpec struct {
	ID          Capability `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	Inputs      []PortSpec `json:"inputs"`
	Outputs     []PortSpec `json:"outputs"`
}

func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, a...))
}

// Validate checks a spec, which may come from an untrusted external Provider or plugin.
func (s CapabilitySpec) Validate() error {
	if !s.ID.Valid() {
		return invalid("capability id %q is not a dotted lowercase identifier", s.ID)
	}
	if strings.TrimSpace(s.Title) == "" || utf8.RuneCountInString(s.Title) > MaxTitle {
		return invalid("%s: title is required and at most %d characters", s.ID, MaxTitle)
	}
	if utf8.RuneCountInString(s.Description) > MaxDescription {
		return invalid("%s: description is longer than %d characters", s.ID, MaxDescription)
	}
	if s.Inputs == nil || s.Outputs == nil {
		return invalid("%s: inputs and outputs are required (use an empty list for none)", s.ID)
	}
	if err := validatePorts(s.ID, "input", s.Inputs, false); err != nil {
		return err
	}
	return validatePorts(s.ID, "output", s.Outputs, true)
}

func validatePorts(id Capability, what string, ports []PortSpec, outputs bool) error {
	if len(ports) > MaxPorts {
		return invalid("%s: more than %d %ss", id, MaxPorts, what)
	}
	seen := map[string]bool{}
	for _, p := range ports {
		if !portNamePattern.MatchString(p.Name) || len(p.Name) > MaxPortName {
			return invalid("%s: %s name %q is not a lowercase identifier", id, what, p.Name)
		}
		if seen[p.Name] {
			return invalid("%s: duplicate %s %q", id, what, p.Name)
		}
		seen[p.Name] = true
		if !p.Kind.Valid() {
			return invalid("%s: %s %q has unknown kind %q", id, what, p.Name, p.Kind)
		}
		if outputs && p.Kind != PortFile {
			return invalid("%s: output %q must be a file", id, p.Name)
		}
		if len(p.MediaTypes) > MaxMediaTypes || len(p.Enum) > MaxEnumValues || utf8.RuneCountInString(p.Description) > MaxDescription {
			return invalid("%s: %s %q exceeds a size limit", id, what, p.Name)
		}
		for _, m := range p.MediaTypes {
			if !mediaTypePattern.MatchString(m) {
				return invalid("%s: %s %q has invalid media type %q", id, what, p.Name, m)
			}
		}
		if p.Kind == PortEnum && len(p.Enum) == 0 {
			return invalid("%s: enum %s %q needs values", id, what, p.Name)
		}
	}
	return nil
}

// Input returns the named input port.
func (s CapabilitySpec) Input(name string) (PortSpec, bool) { return findPort(s.Inputs, name) }

// Output returns the named output port (role).
func (s CapabilitySpec) Output(name string) (PortSpec, bool) { return findPort(s.Outputs, name) }

func findPort(ps []PortSpec, name string) (PortSpec, bool) {
	for _, p := range ps {
		if p.Name == name {
			return p, true
		}
	}
	return PortSpec{}, false
}

// clone returns a deep copy, so callers cannot mutate a Catalog through a returned spec.
func (s CapabilitySpec) clone() CapabilitySpec {
	s.Inputs = clonePorts(s.Inputs)
	s.Outputs = clonePorts(s.Outputs)
	return s
}

func clonePorts(ps []PortSpec) []PortSpec {
	out := make([]PortSpec, len(ps))
	for i, p := range ps {
		p.MediaTypes = append([]string(nil), p.MediaTypes...)
		p.Enum = append([]string(nil), p.Enum...)
		out[i] = p
	}
	return out
}

// MediaTypesCompatible reports whether a file of one of `produced` types may feed an input that
// accepts `accepted`. Empty on either side means "any".
func MediaTypesCompatible(produced, accepted []string) bool {
	if len(produced) == 0 || len(accepted) == 0 {
		return true
	}
	for _, p := range produced {
		for _, a := range accepted {
			if mediaMatch(p, a) || mediaMatch(a, p) {
				return true
			}
		}
	}
	return false
}

func mediaMatch(pattern, mt string) bool {
	if pattern == mt {
		return true
	}
	if base, ok := strings.CutSuffix(pattern, "/*"); ok {
		return strings.HasPrefix(mt, base+"/")
	}
	return false
}
