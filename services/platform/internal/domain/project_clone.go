package domain

// ProjectClone is the Instance-local selected clone, separate from shared Project identity.
// Adapters validate and canonicalize Root on the Instance host; it grants no authority.
type ProjectClone struct {
	ProjectKey ProjectKey
	Root       string
}
