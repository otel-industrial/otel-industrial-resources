# Build the OCB collector distributions defined by the manifests in this directory.
#
#   make                        # build every distribution (default)
#   make modbus-edge-gateway    # build a single distribution
#   make clean                  # remove built output directories
#   make LOCAL=1                # build the in-repo modules from this checkout
#
# Override the collector version or link flags on the command line:
#   make OCB_VERSION=0.163.0     # build against a different collector release
#   make LDFLAGS=                # keep symbols (debuggable, unstripped) build

OCB_VERSION ?= 0.163.0
OCB         ?= go run go.opentelemetry.io/collector/cmd/builder@v$(OCB_VERSION)

# The builder already strips symbols by default (-s -w); setting it here makes
# that explicit and lets `make LDFLAGS=` produce an unstripped, debuggable build.
LDFLAGS ?= -s -w

# LOCAL=1 builds the in-repo modules from this checkout instead of their tagged
# versions, through a copy of the manifest with a replace for every in-repo
# module (the contrib ocb-add-replaces approach). CI uses it, since a release
# PR pins versions that are not tagged until it merges.
LOCAL ?=
REPO_MOD      := github.com/otel-industrial/otel-industrial-resources/otel-industrial-collector
LOCAL_MODULES := $(patsubst ../%/go.mod,%,$(wildcard ../*/go.mod ../*/*/go.mod))

# One target per manifest, derived from the *.yaml files present.
DISTS := $(patsubst %.yaml,%,$(wildcard *.yaml))

.DEFAULT_GOAL := all
.PHONY: all clean $(DISTS)

# Default target: build every individual distribution.
all: $(DISTS)

# Individual targets. Output dir names (dist.name) differ from the target names,
# so these are phony and always rebuild.
$(DISTS): %: %.yaml
ifeq ($(LOCAL),1)
	{ cat $<; echo "replaces:"; $(foreach m,$(LOCAL_MODULES),echo "  - $(REPO_MOD)/$(m) => ../../$(m)";) } > .$*.local.yaml
	$(OCB) --config .$*.local.yaml --ldflags="$(LDFLAGS)"
else
	$(OCB) --config $< --ldflags="$(LDFLAGS)"
endif

clean:
	rm -rf otelcol-*/ .*.local.yaml
