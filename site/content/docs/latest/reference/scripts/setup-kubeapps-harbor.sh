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

info "Updating chart repositories..."
silence helm repo update
echo

# Install Harbor
info "-------------------------"
info "-- Harbor installation --"
info "-------------------------"
echo
"$ROOT_DIR"/script/setup-harbor.sh --namespace "harbor" --disable-clair --disable-notary
# Install Kubeapps
info "---------------------------"
info "-- Kubeapps installation --"
info "---------------------------"
echo
"$ROOT_DIR"/script/setup-kubeapps.sh --namespace "kubeapps" --initial-repos "harbor-library" "http://harbor.harbor.svc.cluster.local/chartrepo/library"
