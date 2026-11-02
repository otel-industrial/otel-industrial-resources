# Release tooling for the Go modules in this repo, following the
# opentelemetry-collector multimod flow. See docs/release.md.

TOOLS_MOD_DIR := $(CURDIR)/internal/tools
# The repo root has no go.mod, so run tools from the tools module directory.
GO_TOOL       := go -C $(TOOLS_MOD_DIR) tool

ALL_MODULES := $(shell find . -type f -name go.mod -not -path './internal/tools/*' -not -path './otel-industrial-collector/manifests/*' -exec dirname {} \; | sort)

MODSET ?= industrial

.PHONY: gotidy
gotidy:
	@set -e; for dir in $(ALL_MODULES); do \
		echo "go mod tidy in $${dir}"; \
		(cd "$${dir}" && go mod tidy); \
	done

# multimod treats every go.mod on disk as a module, including OCB build output.
.PHONY: clean-dists
clean-dists:
	$(MAKE) -C otel-industrial-collector/manifests clean

.PHONY: multimod-verify
multimod-verify: clean-dists
	$(GO_TOOL) multimod verify

.PHONY: multimod-prerelease
multimod-prerelease: clean-dists
	$(GO_TOOL) multimod prerelease -s=true -b=false -v $(CURDIR)/versions.yaml -m $(MODSET)
	$(MAKE) gotidy

COMMIT ?= HEAD
REMOTE ?= git@github.com:otel-industrial/otel-industrial-resources.git
# Tags every module in the set at COMMIT. This uses git directly, since
# `multimod tag` always creates GPG-signed tags.
.PHONY: push-tags
push-tags: clean-dists
	$(GO_TOOL) multimod verify
	set -e; version=`sed -n 's/^    version: //p' versions.yaml`; \
	tags=`sed -n "s#^      - github.com/otel-industrial/otel-industrial-resources/\(.*\)#\1/$${version}#p" versions.yaml`; \
	for tag in $${tags}; do git tag -a -m "$${tag}" "$${tag}" $(COMMIT); done; \
	for tag in $${tags}; do echo "pushing tag $${tag}"; git push $(REMOTE) "$${tag}"; done

.PHONY: check-changes
check-changes:
	$(GO_TOOL) multimod diff -p $(PREVIOUS_VERSION) -m $(MODSET)

.PHONY: prepare-release
prepare-release:
ifndef PREVIOUS_VERSION
	$(error usage: make prepare-release PREVIOUS_VERSION=<eg 0.1.0> RELEASE_CANDIDATE=<eg 0.2.0>)
endif
ifndef RELEASE_CANDIDATE
	$(error usage: make prepare-release PREVIOUS_VERSION=<eg 0.1.0> RELEASE_CANDIDATE=<eg 0.2.0>)
endif
	# ensure a clean branch
	git diff -s --exit-code || (echo "local repository not clean"; exit 1)
	# update files with new version; match only "v<version>" or "version: <version>"
	# so pseudo-versions of external modules are left alone
	sed -i.bak 's/\([v ]\)$(subst .,\.,$(PREVIOUS_VERSION))\b/\1$(RELEASE_CANDIDATE)/g' \
		versions.yaml otel-industrial-collector/manifests/*.yaml
	find . -name "*.bak" -type f -delete
	# commit changes before running multimod
	git add versions.yaml otel-industrial-collector/manifests/*.yaml
	git commit -m "prepare release $(RELEASE_CANDIDATE)"
	$(MAKE) multimod-prerelease
	git add .
	git commit -m "add multimod changes $(RELEASE_CANDIDATE)" || (echo "no multimod changes to commit")
