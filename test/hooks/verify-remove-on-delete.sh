#!/usr/bin/env bash
set -euo pipefail

# verify-remove-on-delete.sh
# Uptest post-assert hook: validate Request REMOVE (DELETE) is sent on resource deletion

RESOURCE_NAME=${RESOURCE_NAME:-${TEST_NAME:-}}
RESOURCE_NAMESPACE=${RESOURCE_NAMESPACE:-${TEST_NAMESPACE:-default}}
RESOURCE_KIND=${RESOURCE_KIND:-requests.http.m.crossplane.io}
TIMEOUT=${VERIFY_REMOVE_TIMEOUT:-120}
LOG_NAMESPACE=${LOG_NAMESPACE:-default}
LOG_SELECTOR=${LOG_SELECTOR:-app=test-server}
LOG_TAIL=${LOG_TAIL:-300}

resource_exists() {
  local kind="$1"
  local name="$2"
  local namespace="${3:-}"

  if [[ "$kind" == "requests.http.crossplane.io" ]]; then
    kubectl get "$kind" "$name" >/dev/null 2>&1
    return
  fi

  kubectl get "$kind" "$name" -n "$namespace" >/dev/null 2>&1
}

to_epoch() {
  local ts="$1"
  date -u -d "$ts" +%s 2>/dev/null || date -u -j -f "%Y-%m-%dT%H:%M:%SZ" "$ts" +%s 2>/dev/null || echo ""
}

log_line_epoch() {
  local line="$1"
  local ts
  ts=$(echo "$line" | awk '{print $1" "$2}')
  date -u -d "$ts" +%s 2>/dev/null || date -u -j -f "%Y/%m/%d %H:%M:%S" "$ts" +%s 2>/dev/null || echo ""
}

print_diagnostics() {
  echo ""
  echo "=== Diagnostics ==="
  if kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" $RR_NS_ARG >/dev/null 2>&1; then
    echo "Resource still exists; describe output:"
    kubectl describe "$RESOURCE_KIND" "$RESOURCE_NAME" $RR_NS_ARG || true
  else
    echo "Resource no longer exists."
  fi
  if [[ -n "${TEST_SERVER_POD:-}" ]]; then
    echo "Recent test-server logs (tail ${LOG_TAIL}):"
    kubectl logs -n "$LOG_NAMESPACE" "$TEST_SERVER_POD" --tail="$LOG_TAIL" 2>/dev/null || true
  else
    echo "No test-server pod discovered."
  fi
}

if ! resource_exists "$RESOURCE_KIND" "$RESOURCE_NAME" "$RESOURCE_NAMESPACE"; then
  for kind in "requests.http.crossplane.io" "requests.http.m.crossplane.io"; do
    if resource_exists "$kind" "$RESOURCE_NAME" "$RESOURCE_NAMESPACE"; then
      RESOURCE_KIND="$kind"
      break
    fi
  done
fi

if [[ "$RESOURCE_KIND" == "requests.http.crossplane.io" ]]; then
  RESOURCE_NAMESPACE=""
fi

if [[ -n "${RESOURCE_NAMESPACE:-}" ]]; then
  RR_NS_ARG="-n $RESOURCE_NAMESPACE"
else
  RR_NS_ARG=""
fi

echo "========================================="
echo "verify-remove-on-delete: validating DELETE on Request deletion"
echo "Resource: $RESOURCE_KIND/$RESOURCE_NAME (namespace: ${RESOURCE_NAMESPACE:-<cluster-scoped>})"
echo "========================================="

if [[ -z "$RESOURCE_NAME" ]]; then
  echo "FAIL: RESOURCE_NAME is empty; set RESOURCE_NAME explicitly in uptest.upbound.io/post-assert-hook"
  exit 1
fi

if ! kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" $RR_NS_ARG >/dev/null 2>&1; then
  echo "FAIL: Resource not found: $RESOURCE_KIND/$RESOURCE_NAME"
  exit 1
fi

RESPONSE_BODY=$(kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" $RR_NS_ARG -o jsonpath='{.status.response.body}' 2>/dev/null || echo "")
if [[ -z "$RESPONSE_BODY" ]]; then
  echo "FAIL: status.response.body is empty; cannot derive external id"
  print_diagnostics
  exit 2
fi

if ! echo "$RESPONSE_BODY" | jq empty >/dev/null 2>&1; then
  echo "FAIL: status.response.body is not valid JSON"
  echo "Body: $RESPONSE_BODY"
  print_diagnostics
  exit 3
fi

USER_ID=$(echo "$RESPONSE_BODY" | jq -r '.id // empty')
if [[ -z "$USER_ID" || "$USER_ID" == "null" ]]; then
  echo "FAIL: Could not extract .id from status.response.body"
  echo "Body: $RESPONSE_BODY"
  print_diagnostics
  exit 4
fi

TEST_START_ISO=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
TEST_START_EPOCH=$(to_epoch "$TEST_START_ISO")
echo "Start time (UTC): $TEST_START_ISO (epoch: $TEST_START_EPOCH)"
echo "Derived external user id: $USER_ID"

TEST_SERVER_POD=$(kubectl get pods -n "$LOG_NAMESPACE" -l "$LOG_SELECTOR" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
if [[ -z "$TEST_SERVER_POD" ]]; then
  echo "FAIL: Could not find test-server pod in namespace '$LOG_NAMESPACE' with selector '$LOG_SELECTOR'"
  print_diagnostics
  exit 5
fi
echo "Using test-server pod: $TEST_SERVER_POD"

echo "Deleting resource..."
kubectl delete "$RESOURCE_KIND" "$RESOURCE_NAME" $RR_NS_ARG --wait=false >/dev/null

echo "Waiting for resource deletion (timeout: ${TIMEOUT}s)..."
if ! kubectl wait --for=delete "$RESOURCE_KIND/$RESOURCE_NAME" $RR_NS_ARG --timeout="${TIMEOUT}s" >/dev/null 2>&1; then
  echo "FAIL: Resource was not deleted within timeout"
  print_diagnostics
  exit 6
fi
echo "PASS: Resource deletion observed"

EXPECTED_PATH="/v1/users/$USER_ID"
echo "Checking test-server logs for provider DELETE to $EXPECTED_PATH ..."

FOUND=0
MATCHED_LINES=""
LOGS=$(kubectl logs -n "$LOG_NAMESPACE" "$TEST_SERVER_POD" 2>/dev/null || echo "")
while IFS= read -r line; do
  [[ -z "$line" ]] && continue
  line_epoch=$(log_line_epoch "$line")
  if [[ -z "$line_epoch" || -z "$TEST_START_EPOCH" ]]; then
    continue
  fi
  if [[ "$line_epoch" -lt "$TEST_START_EPOCH" ]]; then
    continue
  fi
  if echo "$line" | grep -q "\\[DELETE\\] $EXPECTED_PATH" && echo "$line" | grep -q "Go-http-client"; then
    FOUND=1
    MATCHED_LINES="${MATCHED_LINES}${line}"$'\n'
  fi
done <<< "$LOGS"

if [[ "$FOUND" -ne 1 ]]; then
  echo "FAIL: No provider-origin DELETE log line found for $EXPECTED_PATH after test start"
  print_diagnostics
  exit 7
fi

echo "PASS: Found provider-origin DELETE log line(s):"
echo "$MATCHED_LINES"
echo "verify-remove-on-delete: success"
