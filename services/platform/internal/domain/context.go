package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Limits on an assembled context, so an untrusted client cannot make the Platform build or store
// an unbounded document. The per-Step token budget is FEAT-SKL-004, not this.
const (
	MaxContextBytes     = 4 << 20
	MaxContextIntent    = 64 << 10
	MaxContextKnowledge = 64
)

// ContextSkill is a resolved Skill ready to be injected: the audit record and the body whose hash
// it names.
type ContextSkill struct {
	Use  SkillUse
	Body string
}

// KnowledgeItem is a piece of Project documentation included in a context. Knowledge stays in the
// Project (DOMAIN_MODEL rule 3): the context records its path and the hash of the text the
// Platform computed, never a hash the caller supplied.
type KnowledgeItem struct {
	Path string
	Text string
}

// ContextInput is everything a context is built from. The same input gives the same context,
// byte for byte; Skills and Knowledge are included in the order given, because injection order is
// part of what an Agent is told (FEAT-SKL-002).
type ContextInput struct {
	// Agent is the snapshot of the Agent the Step runs, or nil for a Step with no Agent.
	Agent *ResolvedAgent
	// Intent is the task: for example the Work Item's intent.
	Intent    string
	Skills    []ContextSkill
	Knowledge []KnowledgeItem
}

// ContextItem is the record of one included item: what it is, what it was, and how large.
type ContextItem struct {
	Kind    string `json:"kind"` // intent, agent, skill or knowledge
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	Hash    string `json:"hash"` // sha256:<hex> of the text included
	Bytes   int    `json:"bytes"`
}

// Kinds of ContextItem.
const (
	ContextIntent    = "intent"
	ContextAgent     = "agent"
	ContextSkillItem = "skill"
	ContextKnowledge = "knowledge"
)

// AssembledContext is the result: the exact document an Agent is given, its hash and the list of
// what went into it.
type AssembledContext struct {
	// Document is the context text. Hash is "sha256:" and the hex SHA-256 of Document's bytes.
	Document string        `json:"document"`
	Hash     string        `json:"hash"`
	Items    []ContextItem `json:"items"`
}

// ContextFormat names the layout of Document. A change to the layout is a new version, so a stored
// context is always read with the layout it was written with.
const ContextFormat = "amaru-context/1"

// AssembleContext builds the context for the input. It is pure and deterministic: it reads no
// clock, map order or environment, normalizes line endings, and gives every item a header that
// states its exact byte length, so a body that looks like a header cannot be mistaken for one. It
// refuses (ErrInvalid) a Skill whose body is not the one its hash names, a duplicate Skill or
// Knowledge path, text that is not UTF-8, and a document over MaxContextBytes.
func AssembleContext(in ContextInput) (AssembledContext, error) {
	if len(in.Intent) > MaxContextIntent {
		return AssembledContext{}, invalid("context: intent is longer than %d bytes", MaxContextIntent)
	}
	if len(in.Skills) > MaxStepSkills {
		return AssembledContext{}, invalid("context: at most %d skills", MaxStepSkills)
	}
	if len(in.Knowledge) > MaxContextKnowledge {
		return AssembledContext{}, invalid("context: at most %d knowledge items", MaxContextKnowledge)
	}
	var doc strings.Builder
	var items []ContextItem
	section := func(kind, name, version, text string) {
		text = normalizeText(text)
		sum := sha256.Sum256([]byte(text))
		hash := sha256Prefix + hex.EncodeToString(sum[:])
		header := fmt.Sprintf("--- %s name=%q", kind, name)
		if version != "" {
			header += fmt.Sprintf(" version=%q", version)
		}
		header += fmt.Sprintf(" sha256=%s bytes=%d ---\n", hash[len(sha256Prefix):], len(text))
		doc.WriteString(header)
		doc.WriteString(text)
		doc.WriteString("\n")
		items = append(items, ContextItem{Kind: kind, Name: name, Version: version, Hash: hash, Bytes: len(text)})
	}
	doc.WriteString("# " + ContextFormat + "\n")
	if err := checkText("intent", in.Intent); err != nil {
		return AssembledContext{}, err
	}
	if in.Intent != "" {
		section(ContextIntent, "intent", "", in.Intent)
	}
	if a := in.Agent; a != nil {
		if err := checkText("agent instructions", a.Instructions); err != nil {
			return AssembledContext{}, err
		}
		section(ContextAgent, a.ID, strconv.Itoa(a.Revision), a.Instructions)
	}
	seen := map[string]bool{}
	for _, s := range in.Skills {
		if err := s.Use.validate(); err != nil {
			return AssembledContext{}, err
		}
		if err := checkText("skill "+s.Use.Name, s.Body); err != nil {
			return AssembledContext{}, err
		}
		if HashBody(s.Body) != s.Use.Hash {
			return AssembledContext{}, invalid("context: skill %s %s: the body is not the content its hash names", s.Use.Name, s.Use.Version)
		}
		if seen["skill:"+s.Use.Name] {
			return AssembledContext{}, invalid("context: skill %s is listed twice", s.Use.Name)
		}
		seen["skill:"+s.Use.Name] = true
		section(ContextSkillItem, s.Use.Name, s.Use.Version, s.Body)
	}
	for _, k := range in.Knowledge {
		if k.Path == "" || len(k.Path) > maxKnowledgePath || strings.ContainsAny(k.Path, "\x00\\") || strings.HasPrefix(k.Path, "/") || hasDotDot(k.Path) {
			return AssembledContext{}, invalid("context: knowledge path %q must be a relative path inside the Project", k.Path)
		}
		if err := checkText("knowledge "+k.Path, k.Text); err != nil {
			return AssembledContext{}, err
		}
		if seen["knowledge:"+k.Path] {
			return AssembledContext{}, invalid("context: knowledge %s is listed twice", k.Path)
		}
		seen["knowledge:"+k.Path] = true
		section(ContextKnowledge, k.Path, "", k.Text)
	}
	if doc.Len() > MaxContextBytes {
		return AssembledContext{}, invalid("context: the document is %d bytes, over the limit of %d", doc.Len(), MaxContextBytes)
	}
	sum := sha256.Sum256([]byte(doc.String()))
	return AssembledContext{Document: doc.String(), Hash: sha256Prefix + hex.EncodeToString(sum[:]), Items: items}, nil
}

const (
	sha256Prefix     = "sha256:"
	maxKnowledgePath = 1024
)

func hasDotDot(p string) bool {
	for _, el := range strings.Split(p, "/") {
		if el == ".." {
			return true
		}
	}
	return false
}

func checkText(what, s string) error {
	if !utf8.ValidString(s) || strings.ContainsRune(s, 0) {
		return invalid("context: %s is not valid text", what)
	}
	return nil
}

// normalizeText makes line endings the same on every platform and drops trailing newlines, so a
// file saved with CRLF or with an extra blank line at the end gives the same context.
func normalizeText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.TrimRight(s, "\n")
}

// StepContext is the context recorded for a Step: the Evidence of what the Agent was told. It is
// written once and never changes (FEAT-SKL-002, FEAT-ART-002).
type StepContext struct {
	ExecutionID string        `json:"executionId"`
	StepID      string        `json:"stepId"`
	Format      string        `json:"format"`
	Hash        string        `json:"hash"`
	Document    string        `json:"document"`
	Items       []ContextItem `json:"items"`
	CreatedAt   time.Time     `json:"createdAt"`
}

// Validate checks that the record holds the document its hash names.
func (c StepContext) Validate() error {
	sum := sha256.Sum256([]byte(c.Document))
	switch {
	case c.ExecutionID == "" || c.StepID == "":
		return invalid("step context: execution and step are required")
	case c.Format != ContextFormat:
		return invalid("step context %s: format %q is not %s", c.StepID, c.Format, ContextFormat)
	case c.Document == "" || len(c.Document) > MaxContextBytes:
		return invalid("step context %s: the document must be 1 to %d bytes", c.StepID, MaxContextBytes)
	case c.Hash != sha256Prefix+hex.EncodeToString(sum[:]):
		return invalid("step context %s: the hash is not the hash of the document", c.StepID)
	case c.CreatedAt.IsZero():
		return invalid("step context %s: createdAt is required", c.StepID)
	}
	return nil
}
