package domain

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// WorkPackageStatus is the lifecycle label of a Work Package. People set it; progress is
// derived from the Work Items (see ProgressOf), never stored.
type WorkPackageStatus string

const (
	WPPlanned   WorkPackageStatus = "planned"
	WPActive    WorkPackageStatus = "active"
	WPCompleted WorkPackageStatus = "completed"
	WPCancelled WorkPackageStatus = "cancelled"
)

func (s WorkPackageStatus) Valid() bool {
	switch s {
	case WPPlanned, WPActive, WPCompleted, WPCancelled:
		return true
	}
	return false
}

// WorkItemStatus is set by people, or by the delivery loop as its Policy allows (DOMAIN_MODEL
// §5.3). It is deliberately not derived from Executions: a failed run does not make the work
// "failed".
type WorkItemStatus string

const (
	WITodo       WorkItemStatus = "todo"
	WIInProgress WorkItemStatus = "in_progress"
	WIBlocked    WorkItemStatus = "blocked"
	WIDone       WorkItemStatus = "done"
)

// WorkItemStatuses lists every Work Item status, in lifecycle order.
var WorkItemStatuses = []WorkItemStatus{WITodo, WIInProgress, WIBlocked, WIDone}

// workItemTransitions is the complete set of documented moves: todo → in_progress → done, with
// blocked reachable from todo and in_progress and back. Anything absent is invalid.
var workItemTransitions = map[WorkItemStatus][]WorkItemStatus{
	WITodo:       {WIInProgress, WIBlocked},
	WIInProgress: {WIDone, WIBlocked},
	WIBlocked:    {WITodo, WIInProgress},
}

func (s WorkItemStatus) Valid() bool { return slices.Contains(WorkItemStatuses, s) }

// CanTransitionTo reports whether the lifecycle documents the move s → to.
func (s WorkItemStatus) CanTransitionTo(to WorkItemStatus) bool {
	return slices.Contains(workItemTransitions[s], to)
}

// OwnerKind says who owns a Work Item. The owner is always a human; Agents and Teams are
// assignees (DOMAIN_MODEL §7), which no feature models yet.
type OwnerKind string

const OwnerHuman OwnerKind = "human"

// WorkOwner says who is responsible for a Work Item. Executors belong to Executions.
type WorkOwner struct {
	Kind  OwnerKind `json:"kind"`
	Label string    `json:"label,omitempty"`
}

func (o WorkOwner) Valid() bool { return o.Kind == OwnerHuman }

const (
	MaxSummary = 4000
	MaxIntent  = 8000
	MaxReason  = 1000
	MaxLabel   = 120
)

// ulidRe is the Crockford base32 ULID text form: the stable internal id of a Work Package or
// Work Item. It never changes and is never reused.
var ulidRe = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}$`)

// workKeyRe is the display key form `<PROJECTKEY>-<n>`. The format is proposed, not decided:
// DOMAIN_MODEL names `<PROJECTKEY>` without fixing its alphabet or length.
var workKeyRe = regexp.MustCompile(`^([A-Z][A-Z0-9]{1,9})-([1-9][0-9]{0,8})$`)

// WorkKey is the display key of a Work Item, `<PROJECTKEY>-<n>`, unique per Instance. It is for
// people to read and cite; the ULID is the identity. Allocating the next number needs storage
// and arrives with the store (WP-M3-02).
type WorkKey string

// NewWorkKey formats a display key from a project prefix and a sequence number.
func NewWorkKey(prefix string, n int) (WorkKey, error) {
	k := WorkKey(prefix + "-" + strconv.Itoa(n))
	if !workKeyRe.MatchString(string(k)) {
		return "", fmt.Errorf("%w: work key %q", ErrInvalid, k)
	}
	return k, nil
}

// Valid reports whether k has the form `<PROJECTKEY>-<n>`.
func (k WorkKey) Valid() bool { return workKeyRe.MatchString(string(k)) }

// Prefix returns the project part of the key, or "" when the key is invalid.
func (k WorkKey) Prefix() string {
	m := workKeyRe.FindStringSubmatch(string(k))
	if m == nil {
		return ""
	}
	return m[1]
}

// WorkPackage groups Work Items under a goal.
type WorkPackage struct {
	ID        string            `json:"id"`
	Title     string            `json:"title"`
	Summary   string            `json:"summary"`
	Status    WorkPackageStatus `json:"status"`
	CreatedAt time.Time         `json:"createdAt"`
	UpdatedAt time.Time         `json:"updatedAt"`
}

// MaxDependsOn bounds a Work Item's dependency list.
const MaxDependsOn = 20

// WorkItem is a human-owned unit of intended work. It does not start until every Work Item in
// DependsOn is done. The Workflow link, the external issue link and delivery-loop state are
// separate features and are not modelled here.
type WorkItem struct {
	ID            string         `json:"id"`
	Key           WorkKey        `json:"key"`
	PackageID     string         `json:"packageId"`
	Title         string         `json:"title"`
	Intent        string         `json:"intent"`
	Status        WorkItemStatus `json:"status"`
	BlockedReason string         `json:"blockedReason,omitempty"`
	Owner         WorkOwner      `json:"owner"`
	DependsOn     []string       `json:"dependsOn,omitempty"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
}

func checkTitle(title string) error {
	if strings.TrimSpace(title) == "" {
		return invalid("title is required")
	}
	if len([]rune(title)) > MaxTitle {
		return invalid("title is too long")
	}
	return nil
}

// Validate checks the invariants a stored or incoming package must hold.
func (p WorkPackage) Validate() error {
	if !ulidRe.MatchString(p.ID) {
		return invalid("package id must be a ULID")
	}
	if err := checkTitle(p.Title); err != nil {
		return err
	}
	if len([]rune(p.Summary)) > MaxSummary {
		return invalid("summary is too long")
	}
	if !p.Status.Valid() {
		return invalid("invalid package status %q", p.Status)
	}
	return nil
}

// Validate checks the invariants of a Work Item. blockedReason is only allowed while blocked.
func (w WorkItem) Validate() error {
	switch {
	case !ulidRe.MatchString(w.ID):
		return invalid("item id must be a ULID")
	case !w.Key.Valid():
		return invalid("item key must look like PROJ-12")
	case !ulidRe.MatchString(w.PackageID):
		return invalid("package id must be a ULID")
	}
	if err := checkTitle(w.Title); err != nil {
		return err
	}
	switch {
	case len([]rune(w.Intent)) > MaxIntent:
		return invalid("intent is too long")
	case !w.Status.Valid():
		return invalid("invalid item status %q", w.Status)
	case !w.Owner.Valid():
		return invalid("owner.kind must be human")
	case len([]rune(w.Owner.Label)) > MaxLabel:
		return invalid("owner.label is too long")
	case len([]rune(w.BlockedReason)) > MaxReason:
		return invalid("blockedReason is too long")
	case w.Status != WIBlocked && w.BlockedReason != "":
		return invalid("blockedReason is only allowed while status is blocked")
	}
	return checkDependsOn(w.ID, w.DependsOn)
}

// checkDependsOn checks the part of a dependency list that needs no other item: size, id form,
// distinctness and self-reference. Existence and cycles need the graph (see ValidateDependencies).
func checkDependsOn(id string, deps []string) error {
	if len(deps) > MaxDependsOn {
		return invalid("too many dependencies (max %d)", MaxDependsOn)
	}
	seen := make(map[string]bool, len(deps))
	for _, d := range deps {
		switch {
		case !ulidRe.MatchString(d):
			return invalid("dependsOn must list work item ULIDs")
		case seen[d]:
			return invalid("dependsOn lists %s twice", d)
		case d == id:
			return invalid("a work item cannot depend on itself")
		}
		seen[d] = true
	}
	return nil
}

// ActorKind says who is trying to change a Work Item status.
type ActorKind string

const (
	ActorHuman ActorKind = "human"
	// ActorDeliveryLoop is the delivery loop acting under an explicit Policy decision.
	ActorDeliveryLoop ActorKind = "delivery_loop"
)

// Actor is the identity behind a status change. A delivery-loop actor must carry the Policy
// decision that allows the move in Authority; the domain checks it is present, the Policy
// engine owns whether it is right.
type Actor struct {
	Kind      ActorKind
	Authority string
}

func (a Actor) permitted() bool {
	switch a.Kind {
	case ActorHuman:
		return true
	case ActorDeliveryLoop:
		return a.Authority != ""
	}
	return false
}

// SetStatus moves the item along a documented edge. Only a human, or the delivery loop with a
// Policy authority, may do it; an Agent, a Step or an unknown actor gets ErrNotPermitted. A
// move into blocked records reason; any other move clears it.
//
// items are the Work Items of the Project, including w's dependencies. Starting (a move into
// in_progress) is refused with ErrDependenciesNotDone, naming them, while a dependency is not
// done; a dependency missing from items counts as not done. Blocking an item that waits on
// dependencies with no reason of its own records which ones, so the block explains itself.
func (w *WorkItem) SetStatus(now time.Time, to WorkItemStatus, by Actor, reason string, items []WorkItem) error {
	if !by.permitted() {
		return fmt.Errorf("%w: %q may not set a work item status", ErrNotPermitted, by.Kind)
	}
	if !w.Status.CanTransitionTo(to) {
		return fmt.Errorf("%w: work item %s -> %s", ErrInvalidTransition, w.Status, to)
	}
	unmet := w.UnmetDependencies(items)
	switch {
	case to == WIInProgress && len(unmet) > 0:
		return fmt.Errorf("%w: %s", ErrDependenciesNotDone, describeUnmet(unmet))
	case to == WIBlocked && reason == "" && len(unmet) > 0:
		reason = describeUnmet(unmet)
	case to != WIBlocked:
		reason = ""
	}
	if len([]rune(reason)) > MaxReason {
		return invalid("blockedReason is too long")
	}
	w.Status, w.BlockedReason, w.UpdatedAt = to, reason, now
	return nil
}

// Dependency says which dependency of a Work Item is not done, and what state it is in. Key and
// Status are empty when the dependency is not among the items given.
type Dependency struct {
	ID     string
	Key    WorkKey
	Status WorkItemStatus
}

// UnmetDependencies lists the dependencies of w that are not done, in DependsOn order. A
// dependency missing from items is unmet: the check fails closed.
func (w WorkItem) UnmetDependencies(items []WorkItem) []Dependency {
	var unmet []Dependency
	for _, id := range w.DependsOn {
		i := slices.IndexFunc(items, func(it WorkItem) bool { return it.ID == id })
		switch {
		case i < 0:
			unmet = append(unmet, Dependency{ID: id})
		case items[i].Status != WIDone:
			unmet = append(unmet, Dependency{ID: id, Key: items[i].Key, Status: items[i].Status})
		}
	}
	return unmet
}

func describeUnmet(unmet []Dependency) string {
	parts := make([]string, len(unmet))
	for i, d := range unmet {
		if d.Key == "" {
			parts[i] = d.ID + " (not found)"
			continue
		}
		parts[i] = fmt.Sprintf("%s (%s)", d.Key, d.Status)
	}
	return "waiting on " + strings.Join(parts, ", ")
}

// SetDependsOn replaces the dependencies of w with deps after checking them against items, the
// Work Items of the Project (w may or may not be among them). Every dependency must exist, be a
// distinct id other than w's, and the result must stay acyclic. Dependencies may cross Work
// Packages; that they stay within one Project is for the caller, which knows the Projects.
// On error w is unchanged.
func (w *WorkItem) SetDependsOn(now time.Time, deps []string, items []WorkItem) error {
	if err := checkDependsOn(w.ID, deps); err != nil {
		return err
	}
	for _, d := range deps {
		if !slices.ContainsFunc(items, func(it WorkItem) bool { return it.ID == d }) {
			return invalid("dependency %s does not exist", d)
		}
	}
	next := slices.Clone(deps)
	graph := make([]WorkItem, 0, len(items)+1)
	for _, it := range items {
		if it.ID != w.ID {
			graph = append(graph, it)
		}
	}
	edited := *w
	edited.DependsOn = next
	graph = append(graph, edited)
	if err := ValidateDependencies(graph); err != nil {
		return err
	}
	w.DependsOn, w.UpdatedAt = next, now
	return nil
}

// ValidateDependencies checks the dependency graph of items: every dependency exists, lists are
// well formed, and there is no cycle. A cycle error names the Work Items on it.
func ValidateDependencies(items []WorkItem) error {
	byID := make(map[string]WorkItem, len(items))
	for _, it := range items {
		byID[it.ID] = it
	}
	for _, it := range items {
		if err := checkDependsOn(it.ID, it.DependsOn); err != nil {
			return err
		}
		for _, d := range it.DependsOn {
			if _, ok := byID[d]; !ok {
				return invalid("work item %s depends on %s, which does not exist", it.ID, d)
			}
		}
	}
	if cycle := DependencyCycle(items); len(cycle) > 0 {
		keys := make([]string, len(cycle))
		for i, id := range cycle {
			keys[i] = string(byID[id].Key)
			if keys[i] == "" {
				keys[i] = id
			}
		}
		return fmt.Errorf("%w: %w: %s", ErrInvalid, ErrDependencyCycle, strings.Join(keys, " -> "))
	}
	return nil
}

// DependencyCycle returns the ids on one dependency cycle among items, in dependency order, or
// nil when the graph is acyclic. Dependencies on ids not in items are ignored here; see
// ValidateDependencies for those.
func DependencyCycle(items []WorkItem) []string {
	deps := make(map[string][]string, len(items))
	for _, it := range items {
		deps[it.ID] = it.DependsOn
	}
	const (
		visiting = 1
		finished = 2
	)
	state := make(map[string]int, len(items))
	var path []string
	var visit func(id string) []string
	visit = func(id string) []string {
		switch state[id] {
		case visiting:
			start := slices.Index(path, id)
			return slices.Clone(path[start:])
		case finished:
			return nil
		}
		if _, ok := deps[id]; !ok {
			return nil
		}
		state[id] = visiting
		path = append(path, id)
		for _, d := range deps[id] {
			if c := visit(d); c != nil {
				return c
			}
		}
		path = path[:len(path)-1]
		state[id] = finished
		return nil
	}
	for _, it := range items {
		if c := visit(it.ID); c != nil {
			return c
		}
	}
	return nil
}

// Progress counts the Work Items of a package by status. It is derived on demand from the
// items and never stored, so it cannot drift from them.
type Progress struct {
	Total      int
	Todo       int
	InProgress int
	Blocked    int
	Done       int
}

// ProgressOf derives the progress of the package packageID from items. Items of other packages
// are ignored; an item with an unknown status is counted in Total only.
func ProgressOf(packageID string, items []WorkItem) Progress {
	var p Progress
	for _, it := range items {
		if it.PackageID != packageID {
			continue
		}
		p.Total++
		switch it.Status {
		case WITodo:
			p.Todo++
		case WIInProgress:
			p.InProgress++
		case WIBlocked:
			p.Blocked++
		case WIDone:
			p.Done++
		}
	}
	return p
}

// Complete reports whether the package has items and all of them are done.
func (p Progress) Complete() bool { return p.Total > 0 && p.Done == p.Total }
