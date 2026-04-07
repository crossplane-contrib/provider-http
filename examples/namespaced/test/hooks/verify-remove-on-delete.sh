#!/usr/bin/env bash
set -euo pipefail

export RESOURCE_NAME="manage-user-delete-remove-e2e"
export RESOURCE_KIND="requests.http.m.crossplane.io"
export RESOURCE_NAMESPACE="default"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_HOOK="${SCRIPT_DIR}/../../../../test/hooks/verify-remove-on-delete.sh"

exec "${ROOT_HOOK}" "$@"
