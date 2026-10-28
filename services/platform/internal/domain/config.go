package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
)

// ConfigScope is where a configuration layer comes from (ADR-0039). The order is the resolution
// order: a later scope wins over an earlier one, except where an earlier scope locked the key or
// the key can only be tightened.
type ConfigScope string

const (
	// ConfigDeployment is the operator's: environment and the Platform config file.
	ConfigDeployment ConfigScope = "deployment"
	// ConfigOrganization is stored in the Platform.
	ConfigOrganization ConfigScope = "organization"
	// ConfigProject is the Project's committed configuration.
	ConfigProject ConfigScope = "project"
	// ConfigUser is a person's UI preferences. They never affect execution.
	ConfigUser ConfigScope = "user"
)

var configScopes = []ConfigScope{ConfigDeployment, ConfigOrganization, ConfigProject, ConfigUser}

// rank is the position in the resolution order, or -1 for an unknown scope.
func (s ConfigScope) rank() int { return slices.Index(configScopes, s) }

// mayLock reports whether a layer of this scope may lock keys: only the scopes above the Project.
func (s ConfigScope) mayLock() bool { return s == ConfigDeployment || s == ConfigOrganization }

// ConfigKind is the type of a configuration value.
type ConfigKind string

const (
	ConfigBool   ConfigKind = "bool"
	ConfigInt    ConfigKind = "int"
	ConfigString ConfigKind = "string"
)

// ConfigValue is one typed value. Only the field named by Kind is meaningful.
type ConfigValue struct {
	Kind ConfigKind `json:"kind"`
	Bool bool       `json:"bool,omitempty"`
	Int  int64      `json:"int,omitempty"`
	Str  string     `json:"string,omitempty"`
}

// BoolValue, IntValue and StringValue build a ConfigValue.
func BoolValue(b bool) ConfigValue     { return ConfigValue{Kind: ConfigBool, Bool: b} }
func IntValue(i int64) ConfigValue     { return ConfigValue{Kind: ConfigInt, Int: i} }
func StringValue(s string) ConfigValue { return ConfigValue{Kind: ConfigString, Str: s} }

// Tightening says which value of a security-relevant key is the tighter one.
type Tightening string

const (
	// TightenToFalse and TightenToTrue: for a bool, the named value is the tighter one.
	TightenToFalse Tightening = "false"
	TightenToTrue  Tightening = "true"
	// TightenLower and TightenHigher: for an int, the smaller or larger number is the tighter one.
	TightenLower  Tightening = "lower"
	TightenHigher Tightening = "higher"
)

// ConfigKey declares one key the Platform knows.
type ConfigKey struct {
	Name string     `json:"name"`
	Kind ConfigKind `json:"kind"`
	// Tighten marks a security-relevant key (Policy, egress, retention): a scope above may set it
	// and a scope below can only set a value that is as tight or tighter. Empty means an ordinary
	// key.
	Tighten Tightening `json:"tighten,omitempty"`
	// UserSetting marks a key a person may set as a preference. A user layer cannot set any
	// other key, so a preference never changes execution.
	UserSetting bool `json:"userSetting,omitempty"`
}

// ConfigSchema is the set of known keys, by name.
type ConfigSchema map[string]ConfigKey

// Validate checks that every key is well formed: a name, a known kind, a tightening that fits
// the kind, and no user setting that is also security-relevant.
func (s ConfigSchema) Validate() error {
	for name, k := range s {
		switch {
		case name == "" || k.Name != name:
			return invalid("config schema: key %q is stored under another name %q", k.Name, name)
		case k.Kind != ConfigBool && k.Kind != ConfigInt && k.Kind != ConfigString:
			return invalid("config schema: key %s has kind %q", name, k.Kind)
		case k.Tighten != "" && !k.Tighten.fits(k.Kind):
			return invalid("config schema: key %s of kind %s cannot tighten %q", name, k.Kind, k.Tighten)
		case k.Tighten != "" && k.UserSetting:
			return invalid("config schema: key %s is security-relevant and cannot be a user setting", name)
		}
	}
	return nil
}

func (t Tightening) fits(k ConfigKind) bool {
	switch k {
	case ConfigBool:
		return t == TightenToFalse || t == TightenToTrue
	case ConfigInt:
		return t == TightenLower || t == TightenHigher
	}
	return false
}

// looser reports whether next is a looser value than prev for a key that tightens.
func (k ConfigKey) looser(prev, next ConfigValue) bool {
	switch k.Tighten {
	case TightenToFalse:
		return next.Bool && !prev.Bool
	case TightenToTrue:
		return !next.Bool && prev.Bool
	case TightenLower:
		return next.Int > prev.Int
	case TightenHigher:
		return next.Int < prev.Int
	}
	return false
}

// ConfigLayer is the configuration one scope supplies. Locks are honoured for the Deployment
// and Organization scopes only.
type ConfigLayer struct {
	Scope  ConfigScope            `json:"scope"`
	Values map[string]ConfigValue `json:"values,omitempty"`
	Locks  []string               `json:"locks,omitempty"`
}

// Reasons a layer's entry did not take effect.
const (
	ConfigRefusedUnknownKey  = "unknown_key"
	ConfigRefusedWrongKind   = "wrong_kind"
	ConfigRefusedLocked      = "locked"
	ConfigRefusedLoosens     = "loosens"
	ConfigRefusedNotUser     = "not_a_user_setting"
	ConfigRefusedLockNotAble = "scope_may_not_lock"
)

// ConfigRefusal records an entry a layer supplied and the resolver did not apply. A bad entry
// from one scope never changes what another scope set, and never stops the others applying.
type ConfigRefusal struct {
	Scope  ConfigScope `json:"scope"`
	Key    string      `json:"key"`
	Reason string      `json:"reason"`
}

// EffectiveValue is the value a key resolved to and where it came from.
type EffectiveValue struct {
	Value ConfigValue `json:"value"`
	Scope ConfigScope `json:"scope"`
	// Locked is the scope that locked the key, or empty.
	Locked ConfigScope `json:"locked,omitempty"`
}

// EffectiveConfig is the result of resolving the layers. It has no clock: the same layers always
// give the same Digest, so the digest can be recorded as Evidence of what a run was configured
// with.
type EffectiveConfig struct {
	Values  map[string]EffectiveValue `json:"values"`
	Refused []ConfigRefusal           `json:"refused"`
	// Digest is "sha256:" and the hash of the JSON of Values and Refused.
	Digest string `json:"digest"`
}

// ResolveConfig applies the layers in the order of ADR-0039 and returns the effective values. The
// layers may be given in any order and each scope may appear once; a layer with an unknown or
// repeated scope is a program error and returns ErrInvalid. Entries a scope may not set are
// refused one by one and listed in Refused; they do not fail the resolution.
//
// Rules, per layer from the lowest scope up:
//   - an unknown key, or a value of the wrong kind, is refused;
//   - a key an earlier scope locked is refused, and so is a security-relevant key set to a looser
//     value than an earlier scope set;
//   - the user scope may set user-setting keys only;
//   - the Deployment and Organization scopes may lock keys, which keeps their value (or the lack
//     of one) against every scope after them.
func ResolveConfig(schema ConfigSchema, layers ...ConfigLayer) (EffectiveConfig, error) {
	if err := schema.Validate(); err != nil {
		return EffectiveConfig{}, err
	}
	seen := map[ConfigScope]bool{}
	for _, l := range layers {
		if l.Scope.rank() < 0 {
			return EffectiveConfig{}, invalid("config: unknown scope %q", l.Scope)
		}
		if seen[l.Scope] {
			return EffectiveConfig{}, invalid("config: scope %s given twice", l.Scope)
		}
		seen[l.Scope] = true
	}
	ordered := slices.Clone(layers)
	slices.SortFunc(ordered, func(a, b ConfigLayer) int { return a.Scope.rank() - b.Scope.rank() })

	out := EffectiveConfig{Values: map[string]EffectiveValue{}, Refused: []ConfigRefusal{}}
	locked := map[string]ConfigScope{}
	for _, l := range ordered {
		keys := make([]string, 0, len(l.Values))
		for name := range l.Values {
			keys = append(keys, name)
		}
		slices.Sort(keys)
		for _, name := range keys {
			v := l.Values[name]
			k, known := schema[name]
			reason := ""
			switch {
			case !known:
				reason = ConfigRefusedUnknownKey
			case v.Kind != k.Kind:
				reason = ConfigRefusedWrongKind
			case locked[name] != "":
				reason = ConfigRefusedLocked
			case l.Scope == ConfigUser && !k.UserSetting:
				reason = ConfigRefusedNotUser
			case k.Tighten != "":
				if prev, ok := out.Values[name]; ok && k.looser(prev.Value, v) {
					reason = ConfigRefusedLoosens
				}
			}
			if reason != "" {
				out.Refused = append(out.Refused, ConfigRefusal{Scope: l.Scope, Key: name, Reason: reason})
				continue
			}
			out.Values[name] = EffectiveValue{Value: v, Scope: l.Scope}
		}
		for _, name := range slices.Sorted(slices.Values(l.Locks)) {
			if _, known := schema[name]; !known {
				out.Refused = append(out.Refused, ConfigRefusal{Scope: l.Scope, Key: name, Reason: ConfigRefusedUnknownKey})
				continue
			}
			if !l.Scope.mayLock() {
				out.Refused = append(out.Refused, ConfigRefusal{Scope: l.Scope, Key: name, Reason: ConfigRefusedLockNotAble})
				continue
			}
			if locked[name] != "" {
				continue // the earlier lock stands
			}
			locked[name] = l.Scope
			if ev, ok := out.Values[name]; ok {
				ev.Locked = l.Scope
				out.Values[name] = ev
			}
		}
	}
	raw, err := json.Marshal(struct {
		Values  map[string]EffectiveValue `json:"values"`
		Refused []ConfigRefusal           `json:"refused"`
	}{out.Values, out.Refused})
	if err != nil {
		return EffectiveConfig{}, fmt.Errorf("config: encode: %w", err)
	}
	sum := sha256.Sum256(raw)
	out.Digest = "sha256:" + hex.EncodeToString(sum[:])
	return out, nil
}
