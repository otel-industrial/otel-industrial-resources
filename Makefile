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

# Every component with a metadata.yaml, the same set CI regenerates.
GENERATE_DIRS := $(shell find . -name metadata.yaml -not -path '*/vendor/*' -not -path '*/testdata/*' -exec dirname {} \; | sort)

.PHONY: generate
generate:
	@set -e; for dir in $(GENERATE_DIRS); do \
		echo "make generate in $${dir}"; \
		$(MAKE) -C "$${dir}" generate; \
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
# `multimod tag` always creates GPG-signed tags. COMMIT must be checked out, so
# the versions.yaml read here is the one being tagged; when it is not, this
# offers to check it out and returns to the current branch afterwards. Stops if
# any tag is already on REMOTE, pushes all tags or none, and deletes the local
# tags if the run fails or is interrupted before the push succeeds.
.PHONY: push-tags
push-tags: clean-dists
	set -e; \
	orig=`git symbolic-ref -q --short HEAD || git rev-parse HEAD`; \
	commit=`git rev-parse "$(COMMIT)^{commit}"`; \
	switched=; created=; pushed=; \
	trap 'set +e; test -n "$${pushed}" || test -z "$${created}" || git tag -d $${created}; test -z "$${switched}" || git checkout -q "$${orig}"' EXIT; \
	trap 'exit 1' HUP INT TERM; \
	if [ "`git rev-parse HEAD`" != "$${commit}" ]; then \
		test -z "`git status --porcelain`" || { echo "local repository not clean; cannot check out $(COMMIT)"; exit 1; }; \
		[ -t 0 ] || { echo "check out $(COMMIT) before tagging it"; exit 1; }; \
		printf '%s is not checked out. Check it out now? You will be returned to %s afterwards. [y/N] ' "$(COMMIT)" "$${orig}"; \
		read answer; \
		case "$${answer}" in [yY]*) switched=1; git checkout -q --detach "$${commit}" ;; *) exit 1 ;; esac; \
	fi; \
	$(GO_TOOL) multimod verify; \
	version=`sed -n 's/^    version: //p' versions.yaml`; \
	tags=`sed -n "s#^      - github.com/otel-industrial/otel-industrial-resources/\(.*\)#\1/$${version}#p" versions.yaml`; \
	existing=`for tag in $${tags}; do git ls-remote --tags $(REMOTE) "refs/tags/$${tag}"; done`; \
	test -z "$${existing}" || { echo "tags already on $(REMOTE):"; echo "$${existing}"; exit 1; }; \
	for tag in $${tags}; do \
		git tag -a -m "$${tag}" "$${tag}" $${commit}; \
		created="$${created} $${tag}"; \
	done; \
	echo "pushing $${tags}"; \
	git push --atomic $(REMOTE) $${tags}; \
	pushed=1

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
	test -z "$$(git status --porcelain)" || (echo "local repository not clean"; exit 1)
	# update files with new version; match only "v<version>" or "version: <version>"
	# so pseudo-versions and pre-releases of external modules are left alone
	sed -i.bak -E 's/([v ])$(subst .,\.,$(PREVIOUS_VERSION))([^0-9.-]|$$)/\1$(RELEASE_CANDIDATE)\2/g' \
		versions.yaml otel-industrial-collector/manifests/*.yaml
	find . -name "*.bak" -type f -delete
	# commit changes before running multimod
	git add versions.yaml otel-industrial-collector/manifests/*.yaml
	git commit -m "prepare release $(RELEASE_CANDIDATE)"
	$(MAKE) multimod-prerelease
	git add .
	git commit -m "add multimod changes $(RELEASE_CANDIDATE)" || (echo "no multimod changes to commit")

# Bumps the OTel collector core and contrib modules everywhere to one upstream
# release. OTEL_CONTRIB_VERSION defaults to OTEL_VERSION.
#   make update-otel OTEL_VERSION=0.164.0 OTEL_STABLE_VERSION=1.70.0 OTEL_CONTRIB_VERSION=0.164.1
.PHONY: update-otel
update-otel:
	sh .github/scripts/update-otel.sh $(OTEL_VERSION) $(OTEL_STABLE_VERSION) $(OTEL_CONTRIB_VERSION)

# Moves an out-of-OTel receiver's manifest pin to its latest commit, or to
# VERSION (a tag, branch, or commit).
#   make update-sparkplug [VERSION=<ref>]
.PHONY: update-sparkplug
update-sparkplug:
	sh .github/scripts/update-module.sh github.com/jmacd/opentelemetry-mqtt-sparkplug $(VERSION)

.PHONY: update-opcua
update-opcua:
	sh .github/scripts/update-module.sh github.com/bruegth/opentelemetry-collector-opcua-receiver/receiver/opcua $(VERSION)

# Runs update-otel, then moves the OPC UA and Sparkplug receivers to their
# latest commits.
#   make update-deps OTEL_VERSION=0.164.0 OTEL_STABLE_VERSION=1.70.0 OTEL_CONTRIB_VERSION=0.164.1
.PHONY: update-deps
update-deps: update-otel
	$(MAKE) update-opcua VERSION=
	$(MAKE) update-sparkplug VERSION=
