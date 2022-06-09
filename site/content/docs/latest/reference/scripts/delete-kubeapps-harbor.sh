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

# Delete Harbor
info "---------------------"
info "-- Harbor deletion --"
info "---------------------"
echo
"$ROOT_DIR"/script/delete-harbor.sh --namespace "harbor"
# Delete Kubeapps
info "-----------------------"
info "-- Kubeapps deletion --"
info "-----------------------"
echo
"$ROOT_DIR"/script/delete-kubeapps.sh --namespace "kubeapps"
