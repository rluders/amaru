# Local validation for what exists in this repository (docs/engineering/validation.md).
# Only targets for components that are actually present are defined.

DOCCHECK := tools/doccheck

.PHONY: check fmt vet test docs-check

check: fmt vet test docs-check

# Fails if any Go file is not gofmt-formatted.
fmt:
	@out="$$(gofmt -l $(DOCCHECK))"; if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

vet:
	cd $(DOCCHECK) && go vet ./...

test:
	cd $(DOCCHECK) && go test -count=1 ./...

# Offline Markdown links/anchors, code fences, ADR index and Skill symlinks.
docs-check:
	cd $(DOCCHECK) && go run . ../..
