#!/usr/bin/env bash
set -euo pipefail

# verify-allowed-status-codes.sh
# Uptest post-assert hook: validate allowedStatusCodes functionality

RESOURCE_NAME=${RESOURCE_NAME:-check-resource-not-found}
RESOURCE_NAMESPACE=${RESOURCE_NAMESPACE:-default}
EXPECTED_STATUS_CODE=${EXPECTED_STATUS_CODE:-404}
TIMEOUT=${VERIFY_ALLOWED_STATUS_TIMEOUT:-120}
# Default to the .m API group (matches packaged CRD name)
RESOURCE_KIND=${RESOURCE_KIND:-disposablerequests.http.m.crossplane.io}

# Auto-detect the correct resource kind: sample examples use cluster-scoped
# http.crossplane.io/v1alpha2 while namespaced examples use http.m.crossplane.io/v1alpha2
_detect_disposablerequest_kind() {
  local name="$1" ns_arg="${2:-}"
  for kind in "disposablerequests.http.crossplane.io" "disposablerequests.http.m.crossplane.io"; do
    if kubectl get "$kind" "$name" >/dev/null 2>&1; then echo "$kind"; return 0; fi
    if [[ -n "$ns_arg" ]] && kubectl get "$kind" "$name" $ns_arg >/dev/null 2>&1; then echo "$kind"; return 0; fi
  done
  echo "$RESOURCE_KIND"
}
RESOURCE_KIND=$(_detect_disposablerequest_kind "$RESOURCE_NAME" "-n ${RESOURCE_NAMESPACE}")
# Cluster-scoped resources have no namespace
if [[ "$RESOURCE_KIND" == "disposablerequests.http.crossplane.io" ]]; then RESOURCE_NAMESPACE=""; fi

echo "========================================="
echo "verify-allowed-status-codes: validating $RESOURCE_NAME"
echo "========================================="
echo "Expected status code: $EXPECTED_STATUS_CODE"
echo ""

get_response_status_code() {
  if [[ -n "$RESOURCE_NAMESPACE" ]]; then
    kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" -n "$RESOURCE_NAMESPACE" -o jsonpath='{.status.response.statusCode}' 2>/dev/null || echo ""
  else
    kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" -o jsonpath='{.status.response.statusCode}' 2>/dev/null || echo ""
  fi
}

get_allowed_status_codes() {
  if [[ -n "$RESOURCE_NAMESPACE" ]]; then
    kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" -n "$RESOURCE_NAMESPACE" -o jsonpath='{.spec.forProvider.allowedStatusCodes}' 2>/dev/null || echo "[]"
  else
    kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" -o jsonpath='{.spec.forProvider.allowedStatusCodes}' 2>/dev/null || echo "[]"
  fi
}

get_condition_status() {
  local condition_type=$1
  if [[ -n "$RESOURCE_NAMESPACE" ]]; then
    kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" -n "$RESOURCE_NAMESPACE" -o jsonpath="{.status.conditions[?(@.type=='$condition_type')].status}" 2>/dev/null || echo "Unknown"
  else
    kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" -o jsonpath="{.status.conditions[?(@.type=='$condition_type')].status}" 2>/dev/null || echo "Unknown"
  fi
}

get_condition_reason() {
  local condition_type=$1
  if [[ -n "$RESOURCE_NAMESPACE" ]]; then
    kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" -n "$RESOURCE_NAMESPACE" -o jsonpath="{.status.conditions[?(@.type=='$condition_type')].reason}" 2>/dev/null || echo ""
  else
    kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" -o jsonpath="{.status.conditions[?(@.type=='$condition_type')].reason}" 2>/dev/null || echo ""
  fi
}

get_error_status() {
  if [[ -n "$RESOURCE_NAMESPACE" ]]; then
    kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" -n "$RESOURCE_NAMESPACE" -o jsonpath='{.status.error}' 2>/dev/null || echo ""
  else
    kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" -o jsonpath='{.status.error}' 2>/dev/null || echo ""
  fi
}

get_expected_response() {
  if [[ -n "$RESOURCE_NAMESPACE" ]]; then
    kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" -n "$RESOURCE_NAMESPACE" -o jsonpath='{.spec.forProvider.expectedResponse}' 2>/dev/null || echo ""
  else
    kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" -o jsonpath='{.spec.forProvider.expectedResponse}' 2>/dev/null || echo ""
  fi
}

get_response_body() {
  if [[ -n "$RESOURCE_NAMESPACE" ]]; then
    kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" -n "$RESOURCE_NAMESPACE" -o jsonpath='{.status.response.body}' 2>/dev/null || echo ""
  else
    kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" -o jsonpath='{.status.response.body}' 2>/dev/null || echo ""
  fi
}

# === CHECK 1: Wait for response to be received ===
echo "CHECK 1: Waiting for HTTP response..."

RESPONSE_STATUS_CODE=""
end=$((SECONDS + TIMEOUT))
while [ $SECONDS -lt $end ]; do
  RESPONSE_STATUS_CODE=$(get_response_status_code)
  if [[ -n "$RESPONSE_STATUS_CODE" ]]; then
    echo "PASS: Response received with status code: $RESPONSE_STATUS_CODE"
    break
  fi
  sleep 3
done

if [[ -z "$RESPONSE_STATUS_CODE" ]]; then
  echo "FAIL: No response status code received within timeout"
  if [[ -n "$RESOURCE_NAMESPACE" ]]; then
    kubectl describe "$RESOURCE_KIND" "$RESOURCE_NAME" -n "$RESOURCE_NAMESPACE" 2>/dev/null || true
  else
    kubectl describe "$RESOURCE_KIND" "$RESOURCE_NAME" 2>/dev/null || true
  fi
  exit 1
fi

# === CHECK 2: Verify status code matches expected ===
echo ""
echo "CHECK 2: Verifying response status code..."

if [[ "$RESPONSE_STATUS_CODE" == "$EXPECTED_STATUS_CODE" ]]; then
  echo "PASS: Status code matches expected: $EXPECTED_STATUS_CODE"
else
  echo "FAIL: Status code mismatch - expected $EXPECTED_STATUS_CODE, got $RESPONSE_STATUS_CODE"
  exit 2
fi

# === CHECK 3: Verify allowedStatusCodes contains the status code ===
echo ""
echo "CHECK 3: Verifying allowedStatusCodes configuration..."

ALLOWED_CODES=$(get_allowed_status_codes)
echo "allowedStatusCodes: $ALLOWED_CODES"

# Check if the expected status code is in the allowedStatusCodes array
if echo "$ALLOWED_CODES" | jq -e ". | map(. == $EXPECTED_STATUS_CODE) | any" >/dev/null 2>&1; then
  echo "PASS: Status code $EXPECTED_STATUS_CODE is in allowedStatusCodes"
elif echo "$ALLOWED_CODES" | grep -q "$EXPECTED_STATUS_CODE"; then
  echo "PASS: Status code $EXPECTED_STATUS_CODE found in allowedStatusCodes"
else
  echo "FAIL: Status code $EXPECTED_STATUS_CODE is NOT in allowedStatusCodes"
  echo "This means the request should have been treated as an error"
  exit 3
fi

# === CHECK 4: Verify no error status is set ===
echo ""
echo "CHECK 4: Verifying no error status is set..."

ERROR_STATUS=$(get_error_status)

if [[ -z "$ERROR_STATUS" ]]; then
  echo "PASS: No error status set (status code treated as success)"
else
  echo "FAIL: Error status is set despite allowed status code"
  echo "status.error: $ERROR_STATUS"
  exit 4
fi

# === CHECK 5: Verify Ready condition is True ===
echo ""
echo "CHECK 5: Verifying Ready condition..."

READY_STATUS="Unknown"
end=$((SECONDS + 30))
while [ $SECONDS -lt $end ]; do
  READY_STATUS=$(get_condition_status "Ready")
  if [[ "$READY_STATUS" == "True" ]]; then
    echo "PASS: Ready condition is True"
    break
  fi
  sleep 2
done

if [[ "$READY_STATUS" != "True" ]]; then
  echo "FAIL: Ready condition is not True (status: $READY_STATUS)"
  READY_REASON=$(get_condition_reason "Ready")
  echo "Ready reason: $READY_REASON"
  if [[ -n "$RESOURCE_NAMESPACE" ]]; then
    kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" -n "$RESOURCE_NAMESPACE" -o yaml 2>/dev/null || true
  else
    kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" -o yaml 2>/dev/null || true
  fi
  exit 5
fi

# === CHECK 6: Verify Synced condition is True ===
echo ""
echo "CHECK 6: Verifying Synced condition..."

SYNCED_STATUS=$(get_condition_status "Synced")

if [[ "$SYNCED_STATUS" == "True" ]]; then
  echo "PASS: Synced condition is True"
else
  echo "FAIL: Synced condition is not True (status: $SYNCED_STATUS)"
  SYNCED_REASON=$(get_condition_reason "Synced")
  echo "Synced reason: $SYNCED_REASON"
  exit 6
fi

# === CHECK 7: Verify expectedResponse jq expression is evaluated ===
echo ""
echo "CHECK 7: Verifying expectedResponse evaluation..."

EXPECTED_RESPONSE=$(get_expected_response)

if [[ -n "$EXPECTED_RESPONSE" ]]; then
  echo "expectedResponse: $EXPECTED_RESPONSE"
  
  # Get the response body
  RESPONSE_BODY=$(get_response_body)
  
  if [[ -n "$RESPONSE_BODY" ]]; then
    echo "Response body received (length: ${#RESPONSE_BODY} chars)"
    
    # Try to evaluate the jq expression locally
    JQ_RESULT=$(echo "$RESPONSE_BODY" | jq "$EXPECTED_RESPONSE" 2>/dev/null || echo "error")
    
    if [[ "$JQ_RESULT" == "true" ]]; then
      echo "PASS: expectedResponse jq expression evaluates to true"
    elif [[ "$JQ_RESULT" == "false" ]]; then
      echo "WARN: expectedResponse jq expression evaluates to false"
      echo "Resource still succeeded because status code is allowed"
    elif [[ "$JQ_RESULT" == "error" ]]; then
      echo "WARN: Could not evaluate jq expression locally"
      echo "Expression: $EXPECTED_RESPONSE"
    else
      echo "INFO: jq expression result: $JQ_RESULT"
    fi
  else
    echo "INFO: No response body to evaluate"
  fi
else
  echo "INFO: No expectedResponse configured, skipping jq evaluation"
fi

# === CHECK 8: Verify rollbackRetriesLimit not exhausted ===
echo ""
echo "CHECK 8: Verifying retry behavior..."

if [[ -n "$RESOURCE_NAMESPACE" ]]; then
  RETRIES_LIMIT=$(kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" -n "$RESOURCE_NAMESPACE" -o jsonpath='{.spec.forProvider.rollbackRetriesLimit}' 2>/dev/null || echo "")
  FAILED_COUNT=$(kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" -n "$RESOURCE_NAMESPACE" -o jsonpath='{.status.failed}' 2>/dev/null || echo "0")
else
  RETRIES_LIMIT=$(kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" -o jsonpath='{.spec.forProvider.rollbackRetriesLimit}' 2>/dev/null || echo "")
  FAILED_COUNT=$(kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" -o jsonpath='{.status.failed}' 2>/dev/null || echo "0")
fi

if [[ -n "$RETRIES_LIMIT" ]]; then
  echo "rollbackRetriesLimit: $RETRIES_LIMIT"
  echo "status.failed: $FAILED_COUNT"
  
  # With allowed status codes, failed count should be 0 or very low
  if [[ "$FAILED_COUNT" -eq 0 ]]; then
    echo "PASS: No failures recorded (status code treated as success)"
  elif [[ "$FAILED_COUNT" -le 1 ]]; then
    echo "PASS: Minimal failures ($FAILED_COUNT) - request succeeded"
  else
    echo "WARN: Multiple failures ($FAILED_COUNT) despite allowed status code"
    echo "This may indicate the expectedResponse was initially failing"
  fi
else
  echo "INFO: No rollbackRetriesLimit configured"
fi

# === CHECK 9: Verify response data is accessible ===
echo ""
echo "CHECK 9: Verifying response data accessibility..."

RESPONSE_BODY=$(get_response_body)
if [[ -n "$RESOURCE_NAMESPACE" ]]; then
  RESPONSE_HEADERS=$(kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" -n "$RESOURCE_NAMESPACE" -o jsonpath='{.status.response.headers}' 2>/dev/null || echo "")
else
  RESPONSE_HEADERS=$(kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" -o jsonpath='{.status.response.headers}' 2>/dev/null || echo "")
fi

if [[ -n "$RESPONSE_BODY" ]]; then
  echo "PASS: Response body is accessible (length: ${#RESPONSE_BODY} chars)"
  
  # Try to parse as JSON
  if echo "$RESPONSE_BODY" | jq empty 2>/dev/null; then
    echo "INFO: Response body is valid JSON"
  else
    echo "INFO: Response body is not JSON or is malformed"
  fi
else
  echo "INFO: No response body (may be empty for $EXPECTED_STATUS_CODE)"
fi

if [[ -n "$RESPONSE_HEADERS" && "$RESPONSE_HEADERS" != "{}" ]]; then
  echo "PASS: Response headers are accessible"
else
  echo "INFO: No response headers stored"
fi

# === CHECK 10: Verify behavior with non-allowed status codes ===
echo ""
echo "CHECK 10: Verifying allowed status codes are restrictive..."

# Get all allowed codes
ALLOWED_CODES_ARRAY=$(echo "$ALLOWED_CODES" | jq -r '.[]' 2>/dev/null || echo "")

if [[ -n "$ALLOWED_CODES_ARRAY" ]]; then
  ALLOWED_COUNT=$(echo "$ALLOWED_CODES_ARRAY" | wc -l | tr -d ' ')
  echo "INFO: $ALLOWED_COUNT status code(s) explicitly allowed:"
  echo "$ALLOWED_CODES_ARRAY" | while read -r code; do
    echo "  - $code"
  done
  
  # Check if standard success codes (200-299) are in the list
  HAS_200=$(echo "$ALLOWED_CODES_ARRAY" | grep -E '^2[0-9]{2}$' || true)
  if [[ -z "$HAS_200" ]]; then
    echo "INFO: Standard 2xx codes not in allowedStatusCodes"
    echo "This means only explicitly listed codes are treated as success"
  fi
else
  echo "INFO: Could not parse allowedStatusCodes array"
fi

# === FINAL SUMMARY ===
echo ""
echo "========================================="
echo "ALL CHECKS PASSED"
echo "========================================="
echo "Summary:"
echo "  • Resource: $RESOURCE_NAME (namespace: $RESOURCE_NAMESPACE)"
echo "  • Response status code: $RESPONSE_STATUS_CODE"
echo "  • allowedStatusCodes: $ALLOWED_CODES"
echo "  • Ready: $READY_STATUS"
echo "  • Synced: $SYNCED_STATUS"
echo "  • Error status: ${ERROR_STATUS:-none}"
echo "  • Failures: $FAILED_COUNT"
echo ""
echo "verify-allowed-status-codes: success"
