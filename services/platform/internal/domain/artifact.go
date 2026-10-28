package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// Errors of the Artifact path. ErrOutsideOutput is returned (wrapped) for an output that is not
// inside the output location of its Step, ErrHashMismatch for a hash a Provider claimed that the
// content does not have, and ErrTooLarge for content over the size limit.
var (
	ErrTooLarge      = errors.New("content is too large")
	ErrOutsideOutput = errors.New("output is outside the output location")
	ErrHashMismatch  = errors.New("claimed hash does not match the content")
)

// Limits on an Artifact record, so an untrusted client cannot store unbounded fields.
const (
	MaxArtifactIDLen  = 200
	MaxOutputPathLen  = 1024
	MaxMediaTypeLen   = 255
	ContentHashPrefix = "sha256:"
)

var (
	contentHashPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	mediaPattern       = regexp.MustCompile(`^[a-z]+/[a-z0-9][a-z0-9.+-]*$`)
)

// ContentHash returns the hash of content in the form every Amaru record uses: "sha256:" and the
// lowercase hex SHA-256. Only the Platform computes it; a hash a Provider reports is a claim to
// compare, never a value to store (FEAT-ART-001).
func ContentHash(content []byte) string {
	sum := sha256.Sum256(content)
	return ContentHashPrefix + hex.EncodeToString(sum[:])
}

// ValidContentHash reports whether h has the form ContentHash returns.
func ValidContentHash(h string) bool { return contentHashPattern.MatchString(h) }

// OutputPath validates the path of an output as a Provider reports it and returns its clean form.
// The path is relative to the output location of the Step and uses forward slashes. It may not be
// empty, absolute, contain a ".." element (even one that would stay inside), a backslash or a
// NUL, or name the location itself. The check is lexical; whoever opens the file must also refuse
// a symbolic link that leaves the location (internal/artifact does, through os.Root).
func OutputPath(p string) (string, error) {
	switch {
	case p == "" || len(p) > MaxOutputPathLen || !utf8.ValidString(p):
		return "", outside("output path must be 1 to %d characters of valid UTF-8", MaxOutputPathLen)
	case strings.ContainsAny(p, "\\\x00"):
		return "", outside("output path %q has a backslash or a NUL", p)
	case strings.HasPrefix(p, "/"):
		return "", outside("output path %q is absolute", p)
	}
	for _, el := range strings.Split(p, "/") {
		if el == ".." {
			return "", outside("output path %q leaves the output location", p)
		}
	}
	clean := path.Clean(p)
	if clean == "." {
		return "", outside("output path %q names no file inside the output location", p)
	}
	return clean, nil
}

func outside(format string, a ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrOutsideOutput}, a...)...)
}

// ArtifactVersion is one immutable version of an Artifact: content addressed by its hash, with the
// Step attempt that produced it. An Artifact is the stable id; its history is its versions in
// order (ADR-0017). Nothing here is ever edited.
type ArtifactVersion struct {
	// ArtifactID is the stable id of the Artifact, chosen by the caller (for example the Project
	// and the output name). Version starts at 1 and is assigned by the store.
	ArtifactID string `json:"artifactId"`
	Version    int64  `json:"version"`
	// Hash and Size are computed by the Platform from the content it received.
	Hash      string `json:"hash"`
	Size      int64  `json:"size"`
	MediaType string `json:"mediaType"`
	// Path is where the output was in the output location, for the Evidence reader.
	Path        string    `json:"path"`
	ExecutionID string    `json:"executionId"`
	StepID      string    `json:"stepId"`
	AttemptID   string    `json:"attemptId"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Validate checks the shape of a version, including one read back from the store.
func (v ArtifactVersion) Validate() error {
	switch {
	case v.ArtifactID == "" || len(v.ArtifactID) > MaxArtifactIDLen:
		return invalid("artifact id must be 1 to %d characters", MaxArtifactIDLen)
	case !ValidContentHash(v.Hash):
		return invalid("artifact %s: hash %q is not sha256:<hex>", v.ArtifactID, v.Hash)
	case v.Size < 0:
		return invalid("artifact %s: size is negative", v.ArtifactID)
	case len(v.MediaType) > MaxMediaTypeLen || (v.MediaType != "" && !mediaPattern.MatchString(v.MediaType)):
		return invalid("artifact %s: media type %q is not type/subtype", v.ArtifactID, v.MediaType)
	case v.ExecutionID == "" || v.StepID == "" || v.AttemptID == "":
		return invalid("artifact %s: the producing execution, step and attempt are required", v.ArtifactID)
	case v.CreatedAt.IsZero():
		return invalid("artifact %s: createdAt is required", v.ArtifactID)
	}
	if _, err := OutputPath(v.Path); err != nil {
		return invalid("artifact %s: %v", v.ArtifactID, err)
	}
	return nil
}
