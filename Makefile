# Local validation for what exists in this repository (docs/engineering/validation.md).
# Only targets for components that are actually present are defined.

DOCCHECK := tools/doccheck
SPIKE    := spikes/wp-m1-02
CONTRACTS := packages/contracts
SDK       := packages/sdk
PLATFORM  := services/platform

.PHONY: check fmt vet test docs-check spike-vet contracts-test contracts-proto contracts-ts sdk-ts spike-m1-02 platform-test

check: fmt vet test docs-check spike-vet contracts-test contracts-proto contracts-ts sdk-ts platform-test

# Fails if any Go file is not gofmt-formatted.
fmt:
	@out="$$(gofmt -l $(DOCCHECK) $(SPIKE) $(CONTRACTS) $(PLATFORM))"; if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

vet:
	cd $(DOCCHECK) && go vet ./...

test:
	cd $(DOCCHECK) && go test -count=1 ./...

# Offline Markdown links/anchors, code fences, ADR index and Skill symlinks.
docs-check:
	cd $(DOCCHECK) && go run . ../..

# Spike module (WP-M1-02): format is covered by `fmt`; vet for the host and the two other OSes (early signal only,
# not evidence for them). Needs the pinned modules in the Go module cache.
spike-vet:
	cd $(SPIKE) && go vet ./... && GOOS=windows go vet ./... && GOOS=darwin GOARCH=arm64 go vet ./...

# Contracts package: structural checks on schemas and OpenAPI, the route-drift guard, and the fixture corpus validated in Go
# (stdlib + yaml.v3 + santhosh-tekuri/jsonschema).
contracts-test:
	cd $(CONTRACTS) && go vet ./... && go test -count=1 ./...

# Worker protocol (Protobuf): lint, formatting, and breaking-change check against the committed baseline image.
# Needs the `buf` CLI (verified with 1.69.0). See packages/contracts/README.md.
contracts-proto:
	cd $(CONTRACTS) && buf lint && buf format -d --exit-code && buf breaking --against baseline/worker-v1.binpb

# Contract fixtures in TypeScript (Ajv) from the same corpus, plus the Go/TS parity check: both suites write one
# "fixture<TAB>outcome" line per fixture and the two files must be identical. Needs Node >= 22.18 and pnpm; installs from
# the committed lockfile only.
contracts-ts:
	cd $(CONTRACTS) && pnpm install --frozen-lockfile && pnpm run typecheck
	@d="$$(mktemp -d)" && cd $(CONTRACTS) \
	&& CONTRACTS_OUTCOMES_OUT="$$d/go.tsv" go test -count=1 -run TestFixturesMatchTheirDeclaredOutcome ./ \
	&& CONTRACTS_OUTCOMES_OUT="$$d/ts.tsv" pnpm run test \
	&& diff "$$d/go.tsv" "$$d/ts.tsv" && echo "contracts parity: $$(wc -l < "$$d/go.tsv") fixtures, identical outcomes in Go and TypeScript"

# TypeScript SDK: the generated types must equal a fresh generation from packages/contracts/openapi (staleness check), then
# typecheck and unit tests (typed client against a fake fetch, event stream against fake sockets). Needs Node >= 22.18 and
# pnpm; installs from the committed lockfile only.
sdk-ts:
	cd $(SDK) && pnpm install --frozen-lockfile && pnpm run check:generated && pnpm run typecheck && pnpm run test

# Platform core (WP-M3-01): vet and tests, which include the architecture test. Vet also runs for the two other OSes
# (early signal only; the OS evidence is a native run on each, see docs/engineering/validation.md).
platform-test:
	cd $(PLATFORM) && go vet ./... && GOOS=windows go vet ./... && GOOS=darwin GOARCH=arm64 go vet ./... && go test -count=1 ./...

# The real experiments: needs a bundle made by `spike bundle` (network at build time). See spikes/wp-m1-02/README.md.
spike-m1-02:
	@test -n "$$SPIKE_BUNDLE" || { echo "set SPIKE_BUNDLE to a directory made by: spike bundle -out DIR"; exit 2; }
	cd $(SPIKE) && go test -count=1 ./... && go build -o spike ./cmd/spike && ./spike pg -bundle "$$SPIKE_BUNDLE" -state "$$(mktemp -d)/pg" && ./spike nats && ./spike matrix -bundle "$$SPIKE_BUNDLE" -work "$$(mktemp -d)/mx" -fuzz 3
