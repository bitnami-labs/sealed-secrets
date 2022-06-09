#!/usr/bin/env bash

# Copyright 2020-2022 the Kubeapps contributors.
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

# Constants
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." >/dev/null && pwd)"

# Load Libraries
# shellcheck disable=SC1090
. "${ROOT_DIR}/script/libtest.sh"
# shellcheck disable=SC1090
. "${ROOT_DIR}/script/liblog.sh"

namespace="harbor"
while [[ "$#" -gt 0 ]]; do
    case "$1" in
    -n | --namespace)
        shift
        namespace="${1:?missing namespace}"
        ;;
    *)
        echo "Invalid command line flag $1" >&2
        return 1
        ;;
    esac
    shift
done

# Uninstall Harbor
info "Uninstalling Harbor in namespace '$namespace'..."
silence helm uninstall harbor -n "$namespace"
silence kubectl delete pvc -n "$namespace" $(kubectl get pvc -n "$namespace" -o jsonpath='{.items[*].metadata.name}')
info "Deleting '$namespace' namespace..."
silence kubectl delete ns "$namespace"
