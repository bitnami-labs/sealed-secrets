#!/bin/sh
set -eu
echo "INFO: Starting tests: unit, lint ..."
(set -x
  make -C tests local-tests
)
export KUBECONFIG=/tmp/kubeconfig
echo "INFO: Waiting for kube-api to be available ..."
(set +e
until kubectl get nodes; do
  sleep 1
  # Found that k3s releases create k3s.yaml under diff paths,
  # redirecting stderr just to avoid red-herrings errors
  sed -e s/localhost/kube-api/ -e s/127.0.0.1/kube-api/ \
    /tmp/rancher/k3s.yaml /tmp/rancher/etc/k3s.yaml \
    > ${KUBECONFIG:?} 2>/dev/null
done
)
echo "INFO: initializing kube cluster: ..."
(set -x
  make -C tests kube-init
)
echo "INFO: Starting tests: test-kube ..."
(set -x
  make -C tests kube-validate
)
