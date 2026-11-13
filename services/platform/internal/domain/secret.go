package domain

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// SecretScope selects visibility explicitly; a name-only reference is Project-scoped.
type SecretScope string

const (
	SecretProject      SecretScope = "project"
	SecretOrganization SecretScope = "organization"
)

// SecretReference contains a name, never a value. It is internal, not a wire contract.
type SecretReference struct {
	Scope SecretScope
	Name  string
}

// SecretIdentity is an exact lookup key. An empty Project means Organization visibility.
// Resolving this identity grants no permission to obtain or deliver its value.
type SecretIdentity struct {
	Organization string
	Project      ProjectKey
	Name         string
}

// ErrSecretUnavailable hides backend diagnostics that could contain secret material.
var ErrSecretUnavailable = errors.New("secret lookup unavailable")

// Bind selects exactly one visibility; it never normalizes names or falls back.
func (r SecretReference) Bind(organization string, project ProjectKey) (SecretIdentity, error) {
	if strings.TrimSpace(organization) == "" || !project.Valid() || r.Name == "" || !utf8.ValidString(r.Name) || strings.ContainsRune(r.Name, 0) {
		return SecretIdentity{}, ErrInvalid
	}
	id := SecretIdentity{Organization: organization, Name: r.Name}
	switch r.Scope {
	case "", SecretProject:
		id.Project = project
	case SecretOrganization:
	default:
		return SecretIdentity{}, ErrInvalid
	}
	return id, nil
}
