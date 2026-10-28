package ports

import (
	"os"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// protoPath is the Worker protocol of packages/contracts, relative to this package.
const protoPath = "../../../../packages/contracts/proto/amaru/worker/v1/worker.proto"

var (
	messageRe = regexp.MustCompile(`(?s)\nmessage (\w+) \{(.*?)\n\}`)
	fieldRe   = regexp.MustCompile(`(?m)^\s*(?:repeated |optional )?[\w.]+ (\w+) = (\d+);`)
	enumRe    = regexp.MustCompile(`(?s)\nenum (\w+) \{(.*?)\n\}`)
	enumValRe = regexp.MustCompile(`(?m)^\s*(\w+) = (\d+);`)
)

// TestMirroredMessagesMatchWorkerProto keeps the Go messages honest: every proto field of a
// mirrored message is either carried (a `proto` tag) or listed as not carried yet, and no tag names
// a field the proto does not have.
func TestMirroredMessagesMatchWorkerProto(t *testing.T) {
	raw, err := os.ReadFile(protoPath)
	if err != nil {
		t.Fatal(err)
	}
	src := "\n" + string(raw)
	fields := map[string]map[string]bool{}
	for _, m := range messageRe.FindAllStringSubmatch(src, -1) {
		set := map[string]bool{}
		for _, f := range fieldRe.FindAllStringSubmatch(m[2], -1) {
			set[f[1]] = true
		}
		fields[m[1]] = set
	}

	// Go type → proto message, and the proto fields deliberately not carried yet.
	mirrored := []struct {
		goType     any
		message    string
		notCarried []string
	}{
		{Offer{}, "Offer", nil},
		{Assignment{}, "Assignment", []string{"revision", "grant_ids"}},
		{Lease{}, "Lease", nil},
		{AcceptAttempt{}, "AcceptAttempt", nil},
		{RejectAttempt{}, "RejectAttempt", nil},
		{LeaseRenew{}, "LeaseRenew", nil},
		{LeaseRevoked{}, "LeaseRevoked", nil},
		{Cancel{}, "Cancel", nil},
		{Result{}, "Result", []string{"outputs", "usage"}},
		{ResultAck{}, "ResultAck", nil},
	}
	for _, m := range mirrored {
		t.Run(m.message, func(t *testing.T) {
			want, ok := fields[m.message]
			if !ok {
				t.Fatalf("message %s not found in %s", m.message, protoPath)
			}
			got := map[string]bool{}
			rt := reflect.TypeOf(m.goType)
			for i := 0; i < rt.NumField(); i++ {
				tag := rt.Field(i).Tag.Get("proto")
				if tag == "" {
					t.Errorf("field %s has no proto tag", rt.Field(i).Name)
					continue
				}
				if !want[tag] {
					t.Errorf("field %s: proto %s has no field %q", rt.Field(i).Name, m.message, tag)
				}
				got[tag] = true
			}
			skip := map[string]bool{}
			for _, n := range m.notCarried {
				skip[n] = true
				if got[n] {
					t.Errorf("%q is listed as not carried but is carried", n)
				}
			}
			var missing []string
			for f := range want {
				if !got[f] && !skip[f] {
					missing = append(missing, f)
				}
			}
			sort.Strings(missing)
			if len(missing) > 0 {
				t.Errorf("proto %s fields neither carried nor listed as not carried: %v", m.message, missing)
			}
		})
	}
}

// TestEnumsMatchWorkerProto compares the numeric values the Go constants carry with the proto enums.
func TestEnumsMatchWorkerProto(t *testing.T) {
	raw, err := os.ReadFile(protoPath)
	if err != nil {
		t.Fatal(err)
	}
	enums := map[string]map[string]int{}
	for _, m := range enumRe.FindAllStringSubmatch("\n"+string(raw), -1) {
		vals := map[string]int{}
		for _, v := range enumValRe.FindAllStringSubmatch(m[2], -1) {
			n, _ := strconv.Atoi(v[2])
			vals[v[1]] = n
		}
		enums[m[1]] = vals
	}
	check := func(enum, prefix string, goVals map[string]int) {
		t.Helper()
		for name, n := range goVals {
			protoName := prefix + strings.ToUpper(name)
			if got, ok := enums[enum][protoName]; !ok || got != n {
				t.Errorf("%s: Go %s = %d, proto %s = %d (found %v)", enum, name, n, protoName, got, ok)
			}
		}
	}
	check("RejectReason", "REJECT_REASON_", map[string]int{
		"NO_CAPACITY": int(RejectNoCapacity), "CAPABILITY_UNAVAILABLE": int(RejectCapabilityUnavailable),
		"REVISION_UNAVAILABLE": int(RejectRevisionUnavailable), "DRAINING": int(RejectDraining)})
	check("ResultOutcome", "RESULT_OUTCOME_", map[string]int{
		"SUCCEEDED": int(OutcomeSucceeded), "FAILED": int(OutcomeFailed), "CANCELLED": int(OutcomeCancelled)})
	check("ResultDisposition", "RESULT_DISPOSITION_", map[string]int{
		"ACCEPTED": int(ResultAccepted), "DUPLICATE": int(ResultDuplicate), "REJECTED_STALE": int(ResultRejectedStale)})
}
