SHELL=/bin/bash
JSONNET_FMT=--indent 2 --string-style d --comment-style s --no-pad-arrays --pad-objects --pretty-field-names

ALL_JSONNET=$(wildcard *.jsonnet)
UNITTEST_JSONNET=$(wildcard unittest*.jsonnet)
LIB_JSONNET=$(wildcard ../*.libsonnet)
ALL_K8S_VALIDATE_JSONNET=$(wildcard *-validate.jsonnet)

PHONY_GOLDEN=$(patsubst %.jsonnet,golden/%.json,$(ALL_JSONNET))
PHONY_DIFF=$(patsubst %.jsonnet,%.diff,$(ALL_JSONNET))
PHONY_PARSE=$(patsubst %.jsonnet,%.parse,$(ALL_JSONNET))

## These need to be in-sync with docker-compose.yaml
DOCKER_E2E=e2e-test
TMP_RANCHER=./tmp-rancher
PROJECT=kubelibsonnet

tests: docker-compose-tests

docker-compose-tests:
	install -d $(TMP_RANCHER) $(TMP_RANCHER)/etc && touch $(TMP_RANCHER)/etc/k3s.yaml
	USERID=$$(id -u) docker-compose -p $(PROJECT) up -d
	rc=$$(timeout 60s docker wait $(DOCKER_E2E)) || rc=255 ;\
	   test $$rc -ne 0 && docker logs k3s-api;\
	   docker logs $(DOCKER_E2E);\
	   docker-compose -p $(PROJECT) down;\
	   exit $$rc
	rm -rf ./$(TMP_RANCHER)

test-srcs: unittests lint parse diff
test-kube: validate

# NB: unittest jsonnet files are also covered by parse and diff targets,
#     called out here for convenience
unittests:
	jsonnet $(UNITTEST_JSONNET)

lint:
	@set -e; errs=0; \
        for f in $(ALL_JSONNET) $(LIB_JSONNET); do \
	  if ! jsonnetfmt --test $(JSONNET_FMT) -- $$f; then \
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

diff: diff-help $(PHONY_DIFF)

validate:
	timeout 10 kubectl api-versions > /dev/null \
	|| { echo "WARNING: no usable runtime kube context, skipping."; exit 0 ;} \
	&& kubectl version --short && kubecfg version && kubecfg validate --ignore-unknown=false $(ALL_K8S_VALIDATE_JSONNET)

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
	  echo jsonnetfmt -i $(JSONNET_FMT) -- $$f; \
	  jsonnetfmt -i $(JSONNET_FMT) -- $$f; \
	done

gen-golden: $(PHONY_GOLDEN)

.PHONY: unittests lint parse validate diff %.parse %.diff golden/%.json diff-help fix-lint gen-golden
