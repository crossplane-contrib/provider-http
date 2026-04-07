#!/usr/bin/env bash
set -euo pipefail

export RESOURCE_NAME="manage-user-delete-remove-e2e-sample"
export RESOURCE_KIND="requests.http.crossplane.io"
export RESOURCE_NAMESPACE=""

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_HOOK="${SCRIPT_DIR}/../../../../test/hooks/verify-remove-on-delete.sh"

exec "${ROOT_HOOK}" "$@"
