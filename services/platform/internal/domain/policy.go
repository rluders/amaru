package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Decision is the outcome of a Policy evaluation (ADR-0037).
type Decision string

const (
	DecisionAllow           Decision = "allow"
	DecisionRequireApproval Decision = "require_approval"
	DecisionDeny            Decision = "deny"
)

// PrincipalKind is who or what a request is attributed to (POLICY_AND_APPROVAL_SHAPES §2).
type PrincipalKind string

const (
	PrincipalHuman   PrincipalKind = "human"
	PrincipalAgent   PrincipalKind = "agent"
	PrincipalTrigger PrincipalKind = "trigger" // a webhook, schedule or delivery loop
	PrincipalPlugin  PrincipalKind = "plugin"
)

// Principal is the requester of an action. Only a human holds a Role and may decide an Approval.
// An Agent or Trigger names the human it acts for in Sponsor (ADR-0048 item 7).
type Principal struct {
	Kind    PrincipalKind `json:"kind"`
	ID      string        `json:"id"`
	Sponsor string        `json:"sponsor,omitempty"`
	Role    Role          `json:"role,omitempty"`
}

// Triggers: how the request was started. Every kind goes through the same evaluation.
const (
	TriggerManual       = "manual"
	TriggerWebhook      = "webhook"
	TriggerSchedule     = "schedule"
	TriggerDeliveryLoop = "delivery_loop"
)

// PolicyScope is where the request happens. The Organization is implicit.
type PolicyScope struct {
	ProjectID string `json:"projectId,omitempty"`
}

// PolicyRequest asks whether one action may happen. Evaluate is a pure function of it, the Grants
// and the rules: the caller loads those and records the result.
type PolicyRequest struct {
	Principal  Principal   `json:"principal"`
	Subject    Subject     `json:"subject,omitempty"`
	Capability Capability  `json:"capability"`
	Resource   Resource    `json:"resource,omitempty"`
	Scope      PolicyScope `json:"scope"`
	Trigger    string      `json:"trigger"`
	// DefinitionHash is the hash of the definition the Subject runs under; a Grant bound to a
	// different hash does not match.
	DefinitionHash string `json:"definitionHash,omitempty"`
}

// Validate checks the request is well formed. An invalid request is a caller bug, not a
// refusal, so Evaluate returns an error instead of a decision.
func (r PolicyRequest) Validate() error {
	switch {
	case !r.Capability.Valid():
		return invalid("policy request: capability %q is not a dotted lowercase identifier", r.Capability)
	case r.Principal.ID == "":
		return invalid("policy request: principal id is required")
	case r.Subject != (Subject{}) && (!r.Subject.Kind.Valid() || r.Subject.ID == ""):
		return invalid("policy request: subject needs a valid kind and an id")
	}
	switch r.Principal.Kind {
	case PrincipalHuman:
		if r.Principal.Role != "" && !r.Principal.Role.Valid() {
			return invalid("policy request: role %q is unknown", r.Principal.Role)
		}
	case PrincipalAgent, PrincipalTrigger, PrincipalPlugin:
		if r.Principal.Sponsor == "" {
			return invalid("policy request: a %s principal needs a human sponsor", r.Principal.Kind)
		}
	default:
		return invalid("policy request: principal kind %q is unknown", r.Principal.Kind)
	}
	switch r.Trigger {
	case TriggerManual, TriggerWebhook, TriggerSchedule, TriggerDeliveryLoop:
	default:
		return invalid("policy request: trigger %q is unknown", r.Trigger)
	}
	return nil
}

// RuleEffect is what a matching rule does.
type RuleEffect string

const (
	// EffectDeny wins over every other rule and every Grant.
	EffectDeny RuleEffect = "deny"
	// EffectRequireApproval marks an action approvable: without a Grant it needs an Approval.
	EffectRequireApproval RuleEffect = "require_approval"
	// EffectAllow allows an action without a Grant. Only the built-in defaults and an
	// Organization rule may use it; a Project rule may not (it may only tighten).
	EffectAllow RuleEffect = "allow"
)

// RuleSource says where a rule comes from.
type RuleSource string

const (
	SourceDefault      RuleSource = "default"
	SourceOrganization RuleSource = "organization"
	SourceProject      RuleSource = "project"
)

// PolicyRule is one rule from the built-in defaults or from configuration (ADR-0039).
type PolicyRule struct {
	ID     string     `json:"id"`
	Source RuleSource `json:"source"`
	Effect RuleEffect `json:"effect"`
	// Capabilities are exact ids, "prefix.*" or "*". A rule names at least one.
	Capabilities []string `json:"capabilities"`
	// ResourceType, when set, limits the rule to requests whose resource has that Type and a
	// Value that matches ResourceMatch ("*" at the end is a prefix; empty matches any value).
	ResourceType  string `json:"resourceType,omitempty"`
	ResourceMatch string `json:"resourceMatch,omitempty"`
	Message       string `json:"message,omitempty"`
}

// Validate checks a rule loaded from configuration. A Project rule may not allow.
func (r PolicyRule) Validate() error {
	switch {
	case r.ID == "":
		return invalid("policy rule id is required")
	case len(r.Capabilities) == 0:
		return invalid("policy rule %s: name at least one capability", r.ID)
	case r.Effect != EffectDeny && r.Effect != EffectRequireApproval && r.Effect != EffectAllow:
		return invalid("policy rule %s: effect %q is unknown", r.ID, r.Effect)
	case r.Source != SourceDefault && r.Source != SourceOrganization && r.Source != SourceProject:
		return invalid("policy rule %s: source %q is unknown", r.ID, r.Source)
	case r.Source == SourceProject && r.Effect == EffectAllow:
		return invalid("policy rule %s: a project may add approval requirements or denials, never an allow", r.ID)
	case r.ResourceType == "" && r.ResourceMatch != "":
		return invalid("policy rule %s: a resource pattern needs a resource type", r.ID)
	}
	return nil
}

func (r PolicyRule) matches(req PolicyRequest) bool {
	hit := false
	for _, c := range r.Capabilities {
		if matchPattern(c, string(req.Capability)) {
			hit = true
			break
		}
	}
	if !hit {
		return false
	}
	if r.ResourceType == "" {
		return true
	}
	if r.ResourceType != req.Resource.Type {
		return false
	}
	return r.ResourceMatch == "" || matchPattern(r.ResourceMatch, req.Resource.Value)
}

// Reason codes of a Policy result. Clients tell a refusal from an uncovered request by the code
// (ADR-0048 items 5 and 6).
const (
	ReasonExplicitDeny     = "policy.explicit_deny"
	ReasonUncovered        = "policy.uncovered"
	ReasonGrantMatched     = "grant.matched"
	ReasonRuleRequiresAppr = "rule.require_approval"
	ReasonRuleAllow        = "rule.allow"
)

// PolicyReason is one reason in a result. Message never carries a secret value.
type PolicyReason struct {
	Code    string `json:"code"`
	RuleID  string `json:"ruleId,omitempty"`
	Source  string `json:"source"` // default, organization, project, grant or request
	Message string `json:"message"`
}

// PolicyResult is the outcome of Evaluate: one decision and at least one reason.
type PolicyResult struct {
	Decision       Decision       `json:"decision"`
	Reasons        []PolicyReason `json:"reasons"`
	MatchedGrantID string         `json:"matchedGrantId,omitempty"`
	EvaluatedAt    time.Time      `json:"evaluatedAt"`
}

// Explain renders the result as one actionable sentence led by the first reason's code, for the
// reason a Step is refused or kept waiting.
func (r PolicyResult) Explain(req PolicyRequest) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s (%s): %s", r.Reasons[0].Code, req.Capability, r.Reasons[0].Message)
	for _, rs := range r.Reasons[1:] {
		fmt.Fprintf(&b, "; %s", rs.Message)
	}
	return b.String()
}

// Evaluate decides one request (ADR-0037). The order is fixed:
//
//  1. a matching deny rule wins and no Grant is looked at;
//  2. otherwise an active Grant that covers the request allows it;
//  3. otherwise a matching require_approval rule makes it need an Approval, which also beats an
//     allow rule, because a rule may tighten what another allows;
//  4. otherwise a matching allow rule allows it;
//  5. otherwise it is denied as uncovered.
//
// It is pure and total: the same input gives the same output, every valid request gets one of the
// three decisions, and adding a deny rule never turns a deny or approval into an allow. Reasons
// come in rule order, then Grant id order. It returns an error only for an invalid request or
// rule; a request that no one covers is a deny, not an error.
func Evaluate(now time.Time, req PolicyRequest, grants []Grant, rules []PolicyRule) (PolicyResult, error) {
	if err := req.Validate(); err != nil {
		return PolicyResult{}, err
	}
	for _, r := range rules {
		if err := r.Validate(); err != nil {
			return PolicyResult{}, err
		}
	}
	res := PolicyResult{EvaluatedAt: now}
	var approve, allow []PolicyRule
	for _, r := range rules {
		if !r.matches(req) {
			continue
		}
		switch r.Effect {
		case EffectDeny:
			res.Reasons = append(res.Reasons, ruleReason(ReasonExplicitDeny, r, "denied by rule "+r.ID))
		case EffectRequireApproval:
			approve = append(approve, r)
		case EffectAllow:
			allow = append(allow, r)
		}
	}
	if len(res.Reasons) > 0 {
		res.Decision = DecisionDeny
		return res, nil
	}
	if g, ok := firstCoveringGrant(now, req, grants); ok {
		res.Decision, res.MatchedGrantID = DecisionAllow, g.ID
		res.Reasons = []PolicyReason{{Code: ReasonGrantMatched, Source: "grant", Message: "allowed by grant " + g.ID}}
		return res, nil
	}
	if len(approve) > 0 {
		res.Decision = DecisionRequireApproval
		for _, r := range approve {
			res.Reasons = append(res.Reasons, ruleReason(ReasonRuleRequiresAppr, r, "needs an approval by rule "+r.ID))
		}
		return res, nil
	}
	if len(allow) > 0 {
		res.Decision = DecisionAllow
		for _, r := range allow {
			res.Reasons = append(res.Reasons, ruleReason(ReasonRuleAllow, r, "allowed by rule "+r.ID))
		}
		return res, nil
	}
	res.Decision = DecisionDeny
	res.Reasons = []PolicyReason{{
		Code: ReasonUncovered, Source: "request",
		Message: fmt.Sprintf("no rule or grant covers %s; an Owner or Admin can grant it to %s", req.Capability, subjectLabel(req.Subject)),
	}}
	return res, nil
}

func ruleReason(code string, r PolicyRule, fallback string) PolicyReason {
	msg := r.Message
	if msg == "" {
		msg = fallback
	}
	return PolicyReason{Code: code, RuleID: r.ID, Source: string(r.Source), Message: msg}
}

func subjectLabel(s Subject) string {
	if s == (Subject{}) {
		return "this request (it names no agent, workflow or plugin)"
	}
	return string(s.Kind) + " " + s.ID
}

// firstCoveringGrant returns the lowest-id active Grant that covers the request, so the choice
// does not depend on the order the store returned them.
func firstCoveringGrant(now time.Time, req PolicyRequest, grants []Grant) (Grant, bool) {
	var hits []Grant
	for _, g := range grants {
		if g.Standing(now, req.DefinitionHash) == GrantActive && g.covers(req) {
			hits = append(hits, g)
		}
	}
	if len(hits) == 0 {
		return Grant{}, false
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].ID < hits[j].ID })
	return hits[0], true
}

// DefaultRules are the built-in rules every Platform starts from (POLICY_AND_APPROVAL_SHAPES §5).
// They allow only what runs inside the Step's own worktree without reaching out, and deny the
// reserved merge Capability. Anything else, such as `mcp.call` or `asset.upload`, is uncovered and
// needs a Grant. The consequential actions of rule 12 are added by FEAT-POL-002.
func DefaultRules() []PolicyRule {
	return []PolicyRule{
		{ID: "default.local.allow", Source: SourceDefault, Effect: EffectAllow,
			Capabilities: []string{"agent.run", "check.run", "git.commit", "code.test", "doc.convert", "image.convert", "media.transcode"},
			Message:      "runs in the Step's own worktree"},
		{ID: "default.scm.pr.merge.deny", Source: SourceDefault, Effect: EffectDeny,
			Capabilities: []string{"scm.pr.merge"},
			Message:      "the Platform never merges in the MVP"},
	}
}
