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

namespace="kubeapps"
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

# Uninstall Kubeapps
info "Uninstalling Kubeapps in namespace '$namespace'..."
silence helm uninstall kubeapps -n "$namespace"
silence kubectl delete rolebinding example-kubeapps-repositories-write -n "$namespace"
info "Deleting '$namespace' namespace..."
silence kubectl delete ns "$namespace"

# Delete serviceAccount
info "Deleting 'example' serviceAccount and related RBAC objects..."
silence kubectl delete serviceaccount example --namespace default
silence kubectl delete rolebinding example-edit
