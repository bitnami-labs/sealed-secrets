SHELL=/bin/bash
JSONNET_FMT=--indent 2 --string-style d --comment-style s --no-pad-arrays --pad-objects --pretty-field-names

ALL_JSONNET=$(wildcard *.jsonnet)
UNITTEST_JSONNET=$(wildcard unittest*.jsonnet)
LIB_JSONNET=$(wildcard ../*.libsonnet)
ALL_K8S_VALIDATE_JSONNET=$(wildcard *-validate.jsonnet)

PHONY_GOLDEN=$(patsubst %.jsonnet,golden/%.json,$(ALL_JSONNET))
PHONY_DIFF=$(patsubst %.jsonnet,%.diff,$(ALL_JSONNET))
PHONY_PARSE=$(patsubst %.jsonnet,%.parse,$(ALL_JSONNET))

test: unittests lint parse validate diff

# NB: unittest jsonnet files are also covered by parse and diff targets,
#     called out here for convenience
unittests:
	jsonnet $(UNITTEST_JSONNET)

lint:
	@set -e; errs=0; \
        for f in $(ALL_JSONNET) $(LIB_JSONNET); do \
	  if ! jsonnet fmt --test $(JSONNET_FMT) -- $$f; then \
	    echo "FAILED lint: $$f" >&2; \
	    errs=$$(( $$errs + 1 )); \
	  fi; \
	done; \
	if [ $$errs -gt 0 ]; then \
	  echo "NOTE: if the 'lint' target fails, run:"; \
	  echo "      $(MAKE) fix-lint lint"; \
	  exit 1; \
	fi


parse: $(PHONY_PARSE)

validate:
	timeout 10 kubectl api-versions > /dev/null \
	|| { echo "WARNING: no usable runtime kube context, skipping."; exit 0 ;} \
	&& kubecfg validate $(ALL_K8S_VALIDATE_JSONNET)

diff: diff-help $(PHONY_DIFF)

%.diff: %.jsonnet
	diff -u golden/$(*).json <(jsonnet $(<))

%.parse: %.jsonnet
	jsonnet $(<) > /dev/null

golden/%.json: %.jsonnet
	jsonnet $(<) > $(@)

diff-help:
	@echo "NOTE: if the 'diff' target fails, review output and run:"
	@echo "      $(MAKE) gen-golden diff"
	@echo

fix-lint:
	@set -e; \
	for f in $(ALL_JSONNET) $(LIB_JSONNET); do \
	  echo jsonnet fmt -i $(JSONNET_FMT) -- $$f; \
	  jsonnet fmt -i $(JSONNET_FMT) -- $$f; \
	done

gen-golden: $(PHONY_GOLDEN)

.PHONY: unittests lint parse validate diff %.parse %.diff golden/%.json diff-help fix-lint gen-golden
