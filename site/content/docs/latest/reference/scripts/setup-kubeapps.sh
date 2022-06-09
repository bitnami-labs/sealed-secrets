#!/usr/bin/env bash

# Copyright 2020-2022 the Kubeapps contributors.
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

# Constants
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." >/dev/null && pwd)"
RESET='\033[0m'
GREEN='\033[38;5;2m'
RED='\033[38;5;1m'
YELLOW='\033[38;5;3m'

# Load Libraries
# shellcheck disable=SC1090
. "${ROOT_DIR}/script/libtest.sh"
# shellcheck disable=SC1090
. "${ROOT_DIR}/script/liblog.sh"

# Axiliar functions
print_menu() {
    local script
    script=$(basename "${BASH_SOURCE[0]}")
    log "${RED}NAME${RESET}"
    log "    $(basename -s .sh "${BASH_SOURCE[0]}")"
    log ""
    log "${RED}SYNOPSIS${RESET}"
    log "    $script [${YELLOW}-dh${RESET}] [${YELLOW}-n ${GREEN}\"namespace\"${RESET}] [${YELLOW}--initial-repos ${GREEN}\"name\" \"url\"${RESET}]"
    log ""
    log "${RED}DESCRIPTION${RESET}"
    log "    Script to setup Kubeapps on your K8s cluster."
    log ""
    log "    The options are as follow:"
    log ""
    log "      ${YELLOW}-n, --namespace ${GREEN}[namespace]${RESET}           Namespace to use for Kubeapps."
    log "      ${YELLOW}--initial-repos ${GREEN}[repo_name repo_url]${RESET}   Initial repositories to configure on Kubeapps. This flag can be used several times."
    log "      ${YELLOW}-h, --help${RESET}                            Print this help menu."
    log "      ${YELLOW}-u, --dry-run${RESET}                         Enable \"dry run\" mode."
    log ""
    log "${RED}EXAMPLES${RESET}"
    log "      $script --help"
    log "      $script --namespace \"kubeapps\""
    log "      $script --namespace \"kubeapps\" --initial-repos \"harbor-library\" \"http://harbor.harbor.svc.cluster.local/chartrepo/library\""
    log ""
}

namespace="kubeapps"
initial_repos=("bitnami https://charts.bitnami.com/bitnami")
help_menu=0
dry_run=0
while [[ "$#" -gt 0 ]]; do
    case "$1" in
    -h | --help)
        help_menu=1
        ;;
    -u | --dry-run)
        dry_run=1
        ;;
    --initial-repos)
        shift
        repo_name="${1:?missing repo name}"
        shift
        repo_url="${1:?missing repo url}"
        initial_repos=("${initial_repos[@]}" "$repo_name $repo_url")
        ;;
    -n | --namespace)
        shift
        namespace="${1:?missing namespace}"
        ;;
    *)
        error "Invalid command line flag $1" >&2
        exit 1
        ;;
    esac
    shift
done

if [[ "$help_menu" -eq 1 ]]; then
    print_menu
    exit 0
fi

# Kubeapps values
values="$(
    cat <<EOF
useHelm3: true
apprepository:
  initialRepos:
EOF
)"
for repo in "${initial_repos[@]}"; do
    values="$(
        cat <<EOF
$values
    - name: $(echo "$repo" | awk '{print $1}')
      url: $(echo "$repo" | awk '{print $2}')
EOF
    )"
done

if [[ "$dry_run" -eq 1 ]]; then
    info "DRY RUN mode enabled!"
    info "Namespace: $namespace"
    info "Generated values.yaml:"
    printf '#####\n\n%s\n\n#####\n' "$values"
    exit 0
fi

# Install Kubeapps
info "Using the values.yaml below:"
printf '#####\n\n%s\n\n#####\n' "$values"
info "Installing Kubeapps in namespace '$namespace'..."
silence kubectl create ns "$namespace"
silence helm install kubeapps \
    --namespace "$namespace" \
    -f <(echo "$values") \
    bitnami/kubeapps
# Wait for Kubeapps components
info "Waiting for Kubeapps components to be ready..."
deployments=(
    "kubeapps"
    "kubeapps-internal-apprepository-controller"
    "kubeapps-internal-dashboard"
)

for dep in "${deployments[@]}"; do
    k8s_wait_for_deployment "$namespace" "$dep"
    info "Deployment ${dep} ready!"
done
echo

# Create serviceAccount
info "Creating 'example' serviceAccount and adding RBAC permissions for 'default' namespace..."
silence kubectl create serviceaccount example --namespace default
silence kubectl create -n default rolebinding example-edit --clusterrole=edit --serviceaccount default:example
silence kubectl create -n "$namespace" rolebinding example-kubeapps-repositories-write --clusterrolerole=kubeapps:kubeapps:apprepositories-write --serviceaccount default:example
echo

info "Use this command for port forwading to Kubeapps Dashboard:"
info "kubectl port-forward --namespace $namespace svc/kubeapps 8080:80 >/dev/null 2>&1 &"
info "Kubeapps URL: http://127.0.0.1:8080"
info "Kubeppas API Token:"
kubectl get -n default secret "$(kubectl get serviceaccount example --namespace default -o jsonpath='{.secrets[].name}')" -o go-template='{{.data.token | base64decode}}' && echo
echo
