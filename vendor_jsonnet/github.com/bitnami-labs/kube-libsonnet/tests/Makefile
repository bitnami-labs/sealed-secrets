# K3S_<KUBE_MAJOR_RELEASE> as a "mapping" from KUBE_MAJOR_RELEASE to k3s tag
K3S_V1_13=v0.3.0 # no-more to support v1.18 previous Ingress apiVersion deprecation
K3S_V1_14=v0.8.0
K3S_V1_15=v0.9.0
K3S_V1_16=v1.0.1
K3S_V1_17=v1.17.2-k3s1
K3S_V1_18=v1.18.2-k3s1
#
# Since https://github.com/bitnami-labs/kube-libsonnet/issues/32 we support
# kubernetes v1.14+ (Ingress apiVersion deprecated in v1.18, available since v1.14).
#
# Kubernetes releases we cover with e2e testing:
E2E_K3S_VERSIONS=$(K3S_V1_14) $(K3S_V1_15) $(K3S_V1_16) $(K3S_V1_17) $(K3S_V1_18)

SHELL=/bin/bash
# Rather arbitrary Bitnami style choice
JSONNET_FMT=--indent 2 --string-style d --comment-style s --no-pad-arrays --pad-objects --pretty-field-names

LIB_JSONNET=$(wildcard ../*.libsonnet)
ALL_K8S_VALIDATE_JSONNET=$(wildcard *-validate.pass.jsonnet)

# Filenames provide testing objective
PASS_JSONNET=$(sort $(wildcard test*pass.jsonnet))
FAIL_JSONNET=$(sort $(wildcard test*fail.jsonnet))
UNIT_JSONNET=$(sort $(wildcard unittest*.jsonnet))
ALL_JSONNET=$(sort $(wildcard *.jsonnet))

# Phony %.diff and golden/*.json targets
PHONY_GOLDEN=$(patsubst %.jsonnet,golden/%.json,$(PASS_JSONNET))
PHONY_DIFF=$(patsubst %.jsonnet,%.diff,$(PASS_JSONNET))

# Phony %.eval-pass and %.eval-fail targets
PHONY_EVAL_PASS=$(patsubst %.jsonnet,%.eval-pass,$(PASS_JSONNET))
PHONY_EVAL_FAIL=$(patsubst %.jsonnet,%.eval-fail,$(FAIL_JSONNET))

## NOTE: below values need to be in-sync with docker-compose.yaml
##       (not automated to avoid over-engineering parsing/alike)
DOCKER_E2E=e2e-test
TMP_RANCHER=./tmp-rancher
PROJECT=kubelibsonnet

tests: $(patsubst %,e2e-tests-%,$(E2E_K3S_VERSIONS))
	@echo "SUCCESS: verified Kubernetes versions:"
	@cat $(TMP_RANCHER)/report.txt
	@rm -rf ./$(TMP_RANCHER)

e2e-tests-%: req-docker req-docker-compose
	install -d $(TMP_RANCHER)/root/etc && touch $(TMP_RANCHER)/root/etc/k3s.yaml
	env USERID=$$(id -u) K3S_VERSION=$(*) docker-compose -p $(PROJECT) up -d
	rc=$$(timeout 60s docker wait $(DOCKER_E2E)) || rc=255 ;\
	   test $$rc -ne 0 && docker logs k3s-api;\
	   docker logs $(DOCKER_E2E); \
	   exit $$rc
	docker logs $(DOCKER_E2E)| egrep '^Server.Version.+' | sort -u >> $(TMP_RANCHER)/report.txt
	docker-compose -p $(PROJECT) down
	rm -rf ./$(TMP_RANCHER)/root

local-tests: req-jsonnet unittests lint $(PHONY_EVAL_PASS) $(PHONY_EVAL_FAIL) diff

# NB: unittest jsonnet files are also covered by eval-pass and diff targets,
#     called out here for convenience
unittests:
	jsonnet $(UNIT_JSONNET)

lint: req-jsonnetfmt
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

diff: diff-help $(PHONY_DIFF)

# Used to initialize docker'ized KubeAPI via k3s
kube-init: req-kubectl req-kubecfg
	kubectl version --short | grep k3s # void falsely initializing live clusters
	kubecfg update init-kube.jsonnet

kube-validate: req-kubectl req-kubecfg
	timeout 10 kubectl api-versions > /dev/null \
	|| { echo "WARNING: no usable runtime kube context, skipping."; exit 0 ;} \
	&& kubectl version --short && kubecfg version && kubecfg validate --ignore-unknown=false $(ALL_K8S_VALIDATE_JSONNET)

%.diff: %.jsonnet
	diff -u golden/$(*).json <(jsonnet $(<))

%.eval-pass: %.jsonnet
	@echo "INFO: must PASS: $(<)"
	@(jsonnet $(<) > /dev/null)
	@echo "OK[PASS]: $(<)"

%.eval-fail: %.jsonnet
	@echo "INFO: must FAIL: $(<)"
	@echo -n "| "; (jsonnet $(<) > /dev/null) 2>&1 | grep RUNTIME.ERROR
	@echo "OK[FAIL]: $(<)"


golden/%.json: %.jsonnet
	jsonnet $(<) > $(@)

diff-help:
	@echo "NOTE: if the 'diff' target fails, review output and run:"
	@echo "      $(MAKE) gen-golden diff"
	@echo

fix-lint: req-jsonnetfmt
	@set -e; \
	for f in $(ALL_JSONNET) $(LIB_JSONNET); do \
	  echo jsonnetfmt -i $(JSONNET_FMT) -- $$f; \
	  jsonnetfmt -i $(JSONNET_FMT) -- $$f; \
	done

req-%:
	@which $(*) >/dev/null && exit 0; echo "ERROR: '$(*)' is required in PATH"; exit 1

gen-golden: $(PHONY_GOLDEN)

.PHONY: unittests lint eval-pass eval-fail validate diff %.eval-pass %.evail-fail %.diff golden/%.json diff-help fix-lint gen-golden
