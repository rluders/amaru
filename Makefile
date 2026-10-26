# Local validation for what exists in this repository (docs/engineering/validation.md).
# Only targets for components that are actually present are defined.

DOCCHECK := tools/doccheck
SPIKE    := spikes/wp-m1-02

.PHONY: check fmt vet test docs-check spike-vet spike-m1-02

check: fmt vet test docs-check spike-vet

# Fails if any Go file is not gofmt-formatted.
fmt:
	@out="$$(gofmt -l $(DOCCHECK) $(SPIKE))"; if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

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

# The real experiments: needs a bundle made by `spike bundle` (network at build time). See spikes/wp-m1-02/README.md.
spike-m1-02:
	@test -n "$$SPIKE_BUNDLE" || { echo "set SPIKE_BUNDLE to a directory made by: spike bundle -out DIR"; exit 2; }
	cd $(SPIKE) && go test -count=1 ./...
