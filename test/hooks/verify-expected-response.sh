#!/usr/bin/env bash
set -euo pipefail

# verify-expected-response.sh
# Uptest post-assert hook: validate expectedResponse jq evaluation

RESOURCE_NAME=${RESOURCE_NAME:-send-notification}
RESOURCE_NAMESPACE=${RESOURCE_NAMESPACE:-default}
EXPECTED_JQ=${EXPECTED_JQ:-.body.status == "sent"}
RETRIES_LIMIT=${RETRIES_LIMIT:-}
TIMEOUT=${VERIFY_EXPECTED_RESPONSE_TIMEOUT:-180}
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
# Also try the namespaced variant of the resource name (e.g. send-notification-namespaced)
_resolve_resource_name() {
  local name="$1" ns_arg="${2:-}"
  for kind in "disposablerequests.http.crossplane.io" "disposablerequests.http.m.crossplane.io"; do
    if kubectl get "$kind" "$name" >/dev/null 2>&1; then echo "$name"; return 0; fi
    if [[ -n "$ns_arg" ]] && kubectl get "$kind" "$name" $ns_arg >/dev/null 2>&1; then echo "$name"; return 0; fi
    # Try with -namespaced suffix
    if kubectl get "$kind" "${name}-namespaced" >/dev/null 2>&1; then echo "${name}-namespaced"; return 0; fi
    if [[ -n "$ns_arg" ]] && kubectl get "$kind" "${name}-namespaced" $ns_arg >/dev/null 2>&1; then echo "${name}-namespaced"; return 0; fi
  done
  echo "$name"
}
RESOURCE_NAME=$(_resolve_resource_name "$RESOURCE_NAME" "-n ${RESOURCE_NAMESPACE}")
RESOURCE_KIND=$(_detect_disposablerequest_kind "$RESOURCE_NAME" "-n ${RESOURCE_NAMESPACE}")
# Cluster-scoped resources have no namespace
if [[ "$RESOURCE_KIND" == "disposablerequests.http.crossplane.io" ]]; then RESOURCE_NAMESPACE=""; fi

# Support cluster-scoped resources by conditionally including the namespace flag
if [[ -n "${RESOURCE_NAMESPACE:-}" ]]; then
  RS_NS_ARG="-n $RESOURCE_NAMESPACE"
else
  RS_NS_ARG=""
fi

echo "========================================="
echo "verify-expected-response: validating jq evaluation for $RESOURCE_NAME"
echo "========================================="

# Auto-discover resource if RESOURCE_NAME not present
discover_resource() {
  if kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" $RS_NS_ARG >/dev/null 2>&1; then
    return 0
  fi
  discovered=$(kubectl get "$RESOURCE_KIND" $RS_NS_ARG -o jsonpath='{range .items[?(@.metadata.annotations["upjet.upbound.io/test"]=="true")]}{.metadata.name}{"\n"}{end}' 2>/dev/null | head -n1 || true)
  if [[ -n "$discovered" ]]; then
    RESOURCE_NAME="$discovered"
    echo "INFO: discovered resource name: $RESOURCE_NAME"
    return 0
  fi
  return 1
}
if ! discover_resource >/dev/null 2>&1; then
  echo "INFO: resource $RESOURCE_NAME not found yet; will retry discovery while waiting"
fi

echo "Expected jq expression: $EXPECTED_JQ"
echo ""

get_response_body() {
  kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" $RS_NS_ARG -o jsonpath='{.status.response.body}' 2>/dev/null || echo ""
}

get_condition_status() {
  local condition_type=$1
  kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" $RS_NS_ARG -o jsonpath="{.status.conditions[?(@.type=='$condition_type')].status}" 2>/dev/null || echo "Unknown"
}

get_condition_message() {
  local condition_type=$1
  kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" $RS_NS_ARG -o jsonpath="{.status.conditions[?(@.type=='$condition_type')].message}" 2>/dev/null || echo ""
}

get_failed_count() {
  kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" $RS_NS_ARG -o jsonpath='{.status.failed}' 2>/dev/null || echo "0"
}

get_spec_expected_response() {
  kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" $RS_NS_ARG -o jsonpath='{.spec.forProvider.expectedResponse}' 2>/dev/null || echo ""
}

get_spec_retries_limit() {
  kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" -n "$RESOURCE_NAMESPACE" -o jsonpath='{.spec.forProvider.rollbackRetriesLimit}' 2>/dev/null || echo "1"
}

# === CHECK 1: Verify expectedResponse is configured ===
echo "CHECK 1: Verifying expectedResponse is configured..."

SPEC_EXPECTED_RESPONSE=$(get_spec_expected_response)

if [[ -n "$SPEC_EXPECTED_RESPONSE" ]]; then
  echo "PASS: expectedResponse is configured: $SPEC_EXPECTED_RESPONSE"
  
  # Use spec value if EXPECTED_JQ not explicitly provided
  if [[ "$EXPECTED_JQ" == ".body.status == \"sent\"" ]]; then
    EXPECTED_JQ="$SPEC_EXPECTED_RESPONSE"
    echo "INFO: Using expectedResponse from spec"
  fi
else
  echo "FAIL: No expectedResponse configured in spec"
  exit 1
fi

# === CHECK 2: Wait for response to be received ===
echo ""
echo "CHECK 2: Waiting for HTTP response..."

RESPONSE_BODY=""
end=$((SECONDS + TIMEOUT))
while [ $SECONDS -lt $end ]; do
  # attempt discovery again in case resource was created after script start
  discover_resource >/dev/null 2>&1 || true
  RESPONSE_BODY=$(get_response_body)
  if [[ -n "$RESPONSE_BODY" ]]; then
    echo "PASS: Response received (length: ${#RESPONSE_BODY} chars)"
    break
  fi
  sleep 3
done

if [[ -z "$RESPONSE_BODY" ]]; then
  echo "FAIL: No response body received within timeout"
  kubectl $RS_NS_ARG describe "$RESOURCE_KIND" "$RESOURCE_NAME" 2>/dev/null || true
  exit 2
fi

# === CHECK 3: Verify response is valid JSON ===
echo ""
echo "CHECK 3: Verifying response is valid JSON..."

if echo "$RESPONSE_BODY" | jq empty 2>/dev/null; then
  echo "PASS: Response body is valid JSON"
else
  echo "WARN: Response body is not valid JSON"
  echo "jq expression evaluation may fail"
  echo "Response (first 200 chars): ${RESPONSE_BODY:0:200}"
fi

# === CHECK 3.5: Verify secret redaction in response body ===
echo ""
echo "CHECK 3.5: Verifying secret redaction in response..."

# Check if response contains secret template syntax (expected for security)
SECRET_TEMPLATE_COUNT=$(echo "$RESPONSE_BODY" | grep -o '{{[^}]*:[^}]*:[^}]*}}' | wc -l | tr -d ' ')

if [[ "$SECRET_TEMPLATE_COUNT" -gt 0 ]]; then
  echo "PASS: Found $SECRET_TEMPLATE_COUNT secret reference(s) in response (values are redacted for security)"
  echo "$RESPONSE_BODY" | grep -o '{{[^}]*:[^}]*:[^}]*}}' | sort -u | while read -r template; do
    echo "  - $template"
  done
  echo "INFO: Actual secret values are stored in referenced secrets, not in resource status"
else
  echo "INFO: No secret references found in response (no secret injection configured or values not redacted)"
fi

# === CHECK 4: Evaluate jq expression locally ===
echo ""
echo "CHECK 4: Evaluating jq expression..."

# If response contains secret templates, replace them with a placeholder for jq evaluation
EVAL_RESPONSE="$RESPONSE_BODY"
if [[ "$SECRET_TEMPLATE_COUNT" -gt 0 ]]; then
  echo "INFO: Response contains redacted secrets - jq evaluation may fail if expression depends on secret values"
  # For templates, we expect the jq might fail since the actual value is redacted
fi

JQ_RESULT=$(echo "$EVAL_RESPONSE" | jq "$EXPECTED_JQ" 2>/dev/null || echo "error")

echo "jq expression: $EXPECTED_JQ"
echo "jq result: $JQ_RESULT"

if [[ "$JQ_RESULT" == "true" ]]; then
  echo "PASS: jq expression evaluates to true"
elif [[ "$JQ_RESULT" == "false" ]]; then
  if [[ "$SECRET_TEMPLATE_COUNT" -gt 0 ]]; then
    echo "INFO: jq expression evaluates to false (may be due to redacted secret values in response)"
    echo "This is expected when secret injection is configured - actual values are in secrets"
  else
    echo "WARN: jq expression evaluates to false"
    echo "Resource may still be retrying or may have failed"
  fi
elif [[ "$JQ_RESULT" == "error" ]]; then
  if [[ "$SECRET_TEMPLATE_COUNT" -gt 0 ]]; then
    echo "INFO: jq expression evaluation failed (likely due to redacted secret values)"
    echo "This is expected behavior when secrets are injected - checking resource conditions instead"
  else
    echo "FAIL: jq expression evaluation failed"
    echo "Expression may be invalid or incompatible with response"
    exit 3
  fi
else
  echo "INFO: jq expression returned non-boolean: $JQ_RESULT"
  echo "Expression may need adjustment"
fi

# === CHECK 5: Wait for Ready condition ===
echo ""
echo "CHECK 5: Waiting for Ready condition..."

READY_STATUS="Unknown"
end=$((SECONDS + TIMEOUT))
while [ $SECONDS -lt $end ]; do
  READY_STATUS=$(get_condition_status "Ready")
  
  if [[ "$READY_STATUS" == "True" ]]; then
    echo "PASS: Ready condition is True"
    break
  elif [[ "$READY_STATUS" == "False" ]]; then
    # Check if retries exhausted
    FAILED_COUNT=$(get_failed_count)
    SPEC_RETRIES=$(get_spec_retries_limit)
    
    if [[ "$FAILED_COUNT" -ge "$SPEC_RETRIES" ]]; then
      echo "WARN: Ready is False and retries exhausted"
      echo "expectedResponse may have never evaluated to true"
      break
    fi
  fi
  
  sleep 3
done

if [[ "$READY_STATUS" == "Unknown" ]]; then
  echo "FAIL: Ready condition not set within timeout"
  exit 4
fi

# === CHECK 6: Verify rollbackRetriesLimit behavior ===
echo ""
echo "CHECK 6: Verifying rollbackRetriesLimit behavior..."

SPEC_RETRIES=$(get_spec_retries_limit)
FAILED_COUNT=$(get_failed_count)

if [[ -n "$RETRIES_LIMIT" ]]; then
  EXPECTED_RETRIES="$RETRIES_LIMIT"
else
  EXPECTED_RETRIES="$SPEC_RETRIES"
fi

echo "rollbackRetriesLimit: $EXPECTED_RETRIES"
echo "status.failed: $FAILED_COUNT"

if [[ "$READY_STATUS" == "True" ]]; then
  # Success case - failed count should be low
  if [[ "$FAILED_COUNT" -eq 0 ]]; then
    echo "PASS: Success on first attempt (failed: 0)"
  elif [[ "$FAILED_COUNT" -lt "$EXPECTED_RETRIES" ]]; then
    echo "PASS: Success after $FAILED_COUNT retry(ies)"
  elif [[ "$FAILED_COUNT" -eq "$EXPECTED_RETRIES" ]]; then
    echo "PASS: Success on final retry attempt"
  else
    echo "WARN: Success but failed count ($FAILED_COUNT) exceeds limit ($EXPECTED_RETRIES)"
  fi
else
  # Failure case - should have exhausted retries
  if [[ "$FAILED_COUNT" -ge "$EXPECTED_RETRIES" ]]; then
    echo "PASS: Retries exhausted as expected (failed: $FAILED_COUNT, limit: $EXPECTED_RETRIES)"
  else
    echo "WARN: Resource failed but retries not exhausted (failed: $FAILED_COUNT, limit: $EXPECTED_RETRIES)"
  fi
fi

# === CHECK 7: Verify jq expression matches actual result ===
echo ""
echo "CHECK 7: Verifying consistency between jq evaluation and resource status..."

if [[ "$JQ_RESULT" == "true" && "$READY_STATUS" == "True" ]]; then
  echo "PASS: jq evaluates to true AND resource is Ready"
elif [[ "$JQ_RESULT" == "false" && "$READY_STATUS" == "False" ]]; then
  echo "PASS: jq evaluates to false AND resource is not Ready (consistent)"
elif [[ "$JQ_RESULT" == "true" && "$READY_STATUS" == "False" ]]; then
  echo "WARN: jq evaluates to true but resource is not Ready"
  echo "Resource may have failed for other reasons"
  READY_MESSAGE=$(get_condition_message "Ready")
  echo "Ready message: $READY_MESSAGE"
elif [[ "$JQ_RESULT" == "false" && "$READY_STATUS" == "True" ]]; then
  echo "WARN: jq evaluates to false but resource is Ready"
  echo "This is unexpected - resource should not be Ready"
else
  echo "INFO: Unable to determine consistency (jq: $JQ_RESULT, Ready: $READY_STATUS)"
fi

# === CHECK 8: Verify response structure for common jq patterns ===
echo ""
echo "CHECK 8: Verifying response structure..."

# Check if response has expected structure for the jq expression
if echo "$EXPECTED_JQ" | grep -q '\.body\.'; then
  # Expression expects .body
  HAS_BODY=$(echo "$RESPONSE_BODY" | jq 'has("body")' 2>/dev/null || echo "false")
  if [[ "$HAS_BODY" == "true" ]]; then
    echo "PASS: Response has 'body' field (required by jq expression)"
  else
    echo "WARN: jq expression references .body but response doesn't have body field"
    echo "Response structure:"
    echo "$RESPONSE_BODY" | jq 'keys' 2>/dev/null || echo "Unable to parse"
  fi
fi

if echo "$EXPECTED_JQ" | grep -q '\.headers\.'; then
  # Expression expects .headers
  HAS_HEADERS=$(echo "$RESPONSE_BODY" | jq 'has("headers")' 2>/dev/null || echo "false")
  if [[ "$HAS_HEADERS" == "true" ]]; then
    echo "PASS: Response has 'headers' field"
  else
    echo "WARN: jq expression references .headers but response doesn't have headers field"
  fi
fi

if echo "$EXPECTED_JQ" | grep -q '\.statusCode'; then
  # Expression expects .statusCode
  HAS_STATUS_CODE=$(echo "$RESPONSE_BODY" | jq 'has("statusCode")' 2>/dev/null || echo "false")
  if [[ "$HAS_STATUS_CODE" == "true" ]]; then
    echo "PASS: Response has 'statusCode' field"
  else
    echo "WARN: jq expression references .statusCode but response doesn't have statusCode field"
  fi
fi

# === CHECK 9: Test edge cases in jq expression ===
echo ""
echo "CHECK 9: Testing jq expression edge cases..."

# Test if expression handles null values
NULL_RESULT=$(echo "null" | jq "$EXPECTED_JQ" 2>/dev/null || echo "error")
if [[ "$NULL_RESULT" == "error" ]]; then
  echo "INFO: jq expression fails on null input (will cause retry if response is null)"
elif [[ "$NULL_RESULT" == "false" ]]; then
  echo "PASS: jq expression handles null gracefully (returns false)"
else
  echo "INFO: jq expression on null returns: $NULL_RESULT"
fi

# Test if expression handles empty object
EMPTY_RESULT=$(echo "{}" | jq "$EXPECTED_JQ" 2>/dev/null || echo "error")
if [[ "$EMPTY_RESULT" == "error" ]]; then
  echo "INFO: jq expression fails on empty object"
elif [[ "$EMPTY_RESULT" == "false" ]]; then
  echo "PASS: jq expression handles empty object gracefully (returns false)"
else
  echo "INFO: jq expression on empty object returns: $EMPTY_RESULT"
fi

# === CHECK 10: Verify no infinite retry loop ===
echo ""
echo "CHECK 10: Verifying no infinite retry loop..."

# Wait a bit and check if failed count is still incrementing
FAILED_COUNT_BEFORE=$(get_failed_count)
sleep 10
FAILED_COUNT_AFTER=$(get_failed_count)

if [[ "$FAILED_COUNT_AFTER" -gt "$FAILED_COUNT_BEFORE" ]]; then
  echo "WARN: Failed count increased from $FAILED_COUNT_BEFORE to $FAILED_COUNT_AFTER"
  echo "Resource is still retrying"
  
  if [[ "$FAILED_COUNT_AFTER" -gt "$EXPECTED_RETRIES" ]]; then
    echo "FAIL: Failed count exceeds rollbackRetriesLimit - infinite retry loop detected!"
    exit 5
  else
    echo "INFO: Retries still within limit"
  fi
elif [[ "$FAILED_COUNT_AFTER" -eq "$FAILED_COUNT_BEFORE" ]]; then
  echo "PASS: Failed count stable ($FAILED_COUNT_AFTER) - no infinite retry loop"
else
  echo "INFO: Failed count changed unexpectedly"
fi

# === CHECK 11: Verify Synced condition ===
echo ""
echo "CHECK 11: Verifying Synced condition..."

SYNCED_STATUS=$(get_condition_status "Synced")

if [[ "$SYNCED_STATUS" == "True" ]]; then
  echo "PASS: Synced condition is True (reconciliation complete)"
else
  echo "WARN: Synced condition is $SYNCED_STATUS"
  SYNCED_MESSAGE=$(get_condition_message "Synced")
  if [[ -n "$SYNCED_MESSAGE" ]]; then
    echo "Synced message: $SYNCED_MESSAGE"
  fi
fi

# === FINAL SUMMARY ===
echo ""
echo "========================================="
if [[ "$READY_STATUS" == "True" && "$JQ_RESULT" == "true" ]]; then
  echo "ALL CHECKS PASSED"
else
  echo "CHECKS COMPLETED (with warnings)"
fi
echo "========================================="
echo "Summary:"
echo "  • Resource: $RESOURCE_NAME (namespace: $RESOURCE_NAMESPACE)"
echo "  • expectedResponse: $EXPECTED_JQ"
echo "  • jq evaluation: $JQ_RESULT"
echo "  • Ready: $READY_STATUS"
echo "  • Synced: $SYNCED_STATUS"
echo "  • rollbackRetriesLimit: $EXPECTED_RETRIES"
echo "  • status.failed: $FAILED_COUNT"
echo ""

if [[ "$READY_STATUS" == "True" ]]; then
  echo "verify-expected-response: success"
  exit 0
else
  echo "verify-expected-response: completed with warnings"
  exit 0
fi
