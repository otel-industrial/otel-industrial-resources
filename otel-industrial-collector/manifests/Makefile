# Build the OCB collector distributions defined by the manifests in this directory.
#
#   make                        # build every distribution (default)
#   make modbus-edge-gateway    # build a single distribution
#   make clean                  # remove built output directories
#
# Override the collector version or link flags on the command line:
#   make OCB_VERSION=0.160.0     # build against a different collector release
#   make LDFLAGS=                # keep symbols (debuggable, unstripped) build

OCB_VERSION ?= 0.160.0
OCB         ?= go run go.opentelemetry.io/collector/cmd/builder@v$(OCB_VERSION)

# The builder already strips symbols by default (-s -w); setting it here makes
# that explicit and lets `make LDFLAGS=` produce an unstripped, debuggable build.
LDFLAGS ?= -s -w

# One target per manifest, derived from the *.yaml files present.
DISTS := $(patsubst %.yaml,%,$(wildcard *.yaml))

.DEFAULT_GOAL := all
.PHONY: all clean $(DISTS)

# Default target: build every individual distribution.
all: $(DISTS)

# Individual targets. Output dir names (dist.name) differ from the target names,
# so these are phony and always rebuild.
$(DISTS): %: %.yaml
	$(OCB) --config $< --ldflags="$(LDFLAGS)"

clean:
	rm -rf otelcol-*/
