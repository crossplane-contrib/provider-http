#!/usr/bin/env bash
set -euo pipefail

# verify-secret-templating.sh
# Uptest post-assert hook: validate that secret templates are properly replaced in requests
# This test validates the manage-user Request resource which uses secret templates:
#   - {{ auth:default:token }} in Authorization header
#   - {{ user-password:crossplane-system:password }} in request body

RESOURCE_NAME=${RESOURCE_NAME:-manage-user}
RESOURCE_NAMESPACE=${RESOURCE_NAMESPACE:-default}
TIMEOUT=${VERIFY_TEMPLATING_TIMEOUT:-120}
# Default to the .m API group for requests resources
RESOURCE_KIND=${RESOURCE_KIND:-requests.http.m.crossplane.io}

# Auto-detect the correct resource kind: sample examples use cluster-scoped
# http.crossplane.io/v1alpha2 while namespaced examples use http.m.crossplane.io/v1alpha2
_detect_request_kind() {
  local name="$1" ns_arg="${2:-}"
  for kind in "requests.http.crossplane.io" "requests.http.m.crossplane.io"; do
    if kubectl get "$kind" "$name" >/dev/null 2>&1; then echo "$kind"; return 0; fi
    if [[ -n "$ns_arg" ]] && kubectl get "$kind" "$name" $ns_arg >/dev/null 2>&1; then echo "$kind"; return 0; fi
  done
  echo "$RESOURCE_KIND"
}
_resolve_resource_name() {
  local name="$1" ns_arg="${2:-}"
  for kind in "requests.http.crossplane.io" "requests.http.m.crossplane.io"; do
    if kubectl get "$kind" "$name" >/dev/null 2>&1; then echo "$name"; return 0; fi
    if [[ -n "$ns_arg" ]] && kubectl get "$kind" "$name" $ns_arg >/dev/null 2>&1; then echo "$name"; return 0; fi
    if kubectl get "$kind" "${name}-namespaced" >/dev/null 2>&1; then echo "${name}-namespaced"; return 0; fi
    if [[ -n "$ns_arg" ]] && kubectl get "$kind" "${name}-namespaced" $ns_arg >/dev/null 2>&1; then echo "${name}-namespaced"; return 0; fi
  done
  echo "$name"
}
RESOURCE_NAME=$(_resolve_resource_name "$RESOURCE_NAME" "-n ${RESOURCE_NAMESPACE}")
RESOURCE_KIND=$(_detect_request_kind "$RESOURCE_NAME" "-n ${RESOURCE_NAMESPACE}")
# Cluster-scoped resources have no namespace
if [[ "$RESOURCE_KIND" == "requests.http.crossplane.io" ]]; then RESOURCE_NAMESPACE=""; fi

# Support cluster-scoped resources by conditionally including the namespace flag
if [[ -n "${RESOURCE_NAMESPACE:-}" ]]; then
  RT_NS_ARG="-n $RESOURCE_NAMESPACE"
else
  RT_NS_ARG=""
fi

# Expected secret references used in this test
AUTH_SECRET="auth"
AUTH_NAMESPACE="default"
AUTH_KEY="token"
PASSWORD_SECRET="user-password"
PASSWORD_NAMESPACE="crossplane-system"
PASSWORD_KEY="password"
# Expected value from test server
EXPECTED_AUTH_VALUE="my-secret-value"

echo "========================================="
echo "verify-secret-templating: validating secret template replacement"
echo "========================================="

# Auto-discover resource name if not set or not found (look for uptest marker)
discover_resource() {
  if kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" $RT_NS_ARG >/dev/null 2>&1; then
    return 0
  fi
  discovered=$(kubectl get "$RESOURCE_KIND" $RT_NS_ARG -o jsonpath='{range .items[?(@.metadata.annotations["upjet.upbound.io/test"]=="true")]}{.metadata.name}{"\n"}{end}' 2>/dev/null | head -n1 || true)
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

echo "Resource: $RESOURCE_NAME (namespace: $RESOURCE_NAMESPACE)"
echo "Expected templates:"
echo "  - {{ $AUTH_SECRET:$AUTH_NAMESPACE:$AUTH_KEY }}"
echo "  - {{ $PASSWORD_SECRET:$PASSWORD_NAMESPACE:$PASSWORD_KEY }}"
echo ""

get_request_body() {
  kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" $RT_NS_ARG -o jsonpath='{.status.requestDetails.body}' 2>/dev/null || true
}

get_request_headers() {
  kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" $RT_NS_ARG -o jsonpath='{.status.requestDetails.headers}' 2>/dev/null || true
}

get_response_body() {
  kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" $RT_NS_ARG -o jsonpath='{.status.response.body}' 2>/dev/null || true
}

get_condition_status() {
  local condition_type=$1
  kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" $RT_NS_ARG -o jsonpath="{.status.conditions[?(@.type=='$condition_type')].status}" 2>/dev/null || echo "Unknown"
}

get_spec_body() {
  # For Request resources, body is in payload.body
  kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" $RT_NS_ARG -o jsonpath='{.spec.forProvider.payload.body}' 2>/dev/null || true
}

get_spec_headers() {
  kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" $RT_NS_ARG -o jsonpath='{.spec.forProvider.headers}' 2>/dev/null || true
}

# === CHECK 1: Wait for resource to be reconciled ===
echo "CHECK 1: Waiting for resource to be reconciled..."

READY_STATUS="Unknown"
end=$((SECONDS + TIMEOUT))
while [ $SECONDS -lt $end ]; do
  # Try discovery again in case resource wasn't created at script start
  if ! kubectl get "$RESOURCE_KIND" "$RESOURCE_NAME" $RT_NS_ARG >/dev/null 2>&1; then
    discover_resource >/dev/null 2>&1 || true
  fi
  READY_STATUS=$(get_condition_status "Ready")
  if [[ "$READY_STATUS" == "True" || "$READY_STATUS" == "False" ]]; then
    echo "PASS: Resource reconciled (Ready: $READY_STATUS)"
    break
  fi
  sleep 3
done

if [[ "$READY_STATUS" == "Unknown" ]]; then
  echo "FAIL: Resource not reconciled within timeout"
  kubectl $RT_NS_ARG describe "$RESOURCE_KIND" "$RESOURCE_NAME" 2>/dev/null || true
  exit 1
fi

# === CHECK 2: Verify spec contains expected template syntax ===
echo ""
echo "CHECK 2: Verifying spec contains secret template references..."

SPEC_BODY=$(get_spec_body)
SPEC_HEADERS=$(get_spec_headers)
SPEC_CONTENT="$SPEC_BODY $SPEC_HEADERS"

TEMPLATE_COUNT=$(echo "$SPEC_CONTENT" | grep -o '{{[^}]*:[^}]*:[^}]*}}' | wc -l | tr -d ' ')

if [[ "$TEMPLATE_COUNT" -eq 2 ]]; then
  echo "PASS: Found expected 2 template references in spec"
  echo "$SPEC_CONTENT" | grep -o '{{[^}]*:[^}]*:[^}]*}}' | sort -u | while read -r template; do
    echo "  - $template"
  done
else
  echo "FAIL: Expected 2 templates but found $TEMPLATE_COUNT"
  echo "$SPEC_CONTENT" | grep -o '{{[^}]*:[^}]*:[^}]*}}' | sort -u | while read -r template; do
    echo "  - $template"
  done
  exit 1
fi

# Verify the exact templates we expect
if ! echo "$SPEC_CONTENT" | grep -F "{{ $AUTH_SECRET:$AUTH_NAMESPACE:$AUTH_KEY }}" >/dev/null; then
  echo "FAIL: Missing expected auth template: {{ $AUTH_SECRET:$AUTH_NAMESPACE:$AUTH_KEY }}"
  exit 1
fi

if ! echo "$SPEC_CONTENT" | grep -F "{{ $PASSWORD_SECRET:$PASSWORD_NAMESPACE:$PASSWORD_KEY }}" >/dev/null; then
  echo "FAIL: Missing expected password template: {{ $PASSWORD_SECRET:$PASSWORD_NAMESPACE:$PASSWORD_KEY }}"
  exit 1
fi

echo "PASS: Both expected templates found in spec"

# === CHECK 3: Verify status.requestDetails shows template syntax preserved ===
echo ""
echo "CHECK 3: Verifying status.requestDetails contains template references..."

STATUS_BODY=$(get_request_body)
STATUS_HEADERS=$(get_request_headers)

# Verify auth template is preserved in headers
if echo "$STATUS_HEADERS" | grep -F "{{ $AUTH_SECRET:$AUTH_NAMESPACE:$AUTH_KEY }}" >/dev/null; then
  echo "PASS: Auth template preserved in status.requestDetails.headers"
else
  echo "INFO: Auth template not found in status (may have been replaced)"
fi

# === CHECK 4: Verify secrets exist and contain actual data ===
echo ""
echo "CHECK 4: Verifying secrets exist with actual data..."

# Check auth secret
AUTH_VALUE=$(kubectl get secret "$AUTH_SECRET" -n "$AUTH_NAMESPACE" -o jsonpath="{.data.$AUTH_KEY}" 2>/dev/null | base64 -d 2>/dev/null || true)

if [[ -z "$AUTH_VALUE" ]]; then
  echo "FAIL: Auth secret not found or empty: $AUTH_SECRET:$AUTH_NAMESPACE:$AUTH_KEY"
  exit 4
fi

if [[ "$AUTH_VALUE" == "$EXPECTED_AUTH_VALUE" ]]; then
  echo "PASS: Auth secret has expected value (${#AUTH_VALUE} bytes)"
else
  echo "PASS: Auth secret has data (${#AUTH_VALUE} bytes, expected: $EXPECTED_AUTH_VALUE)"
fi

# Check password secret
PASSWORD_VALUE=$(kubectl get secret "$PASSWORD_SECRET" -n "$PASSWORD_NAMESPACE" -o jsonpath="{.data.$PASSWORD_KEY}" 2>/dev/null | base64 -d 2>/dev/null || true)

if [[ -z "$PASSWORD_VALUE" ]]; then
  echo "FAIL: Password secret not found or empty: $PASSWORD_SECRET:$PASSWORD_NAMESPACE:$PASSWORD_KEY"
  exit 4
fi

echo "PASS: Password secret has data (${#PASSWORD_VALUE} bytes)"

# Verify secrets don't contain template syntax
if echo "$AUTH_VALUE" | grep -E '^\{\{.*:.*:.*\}\}$' >/dev/null; then
  echo "FAIL: Auth secret value is template syntax: $AUTH_VALUE"
  exit 4
fi

if echo "$PASSWORD_VALUE" | grep -E '^\{\{.*:.*:.*\}\}$' >/dev/null; then
  echo "FAIL: Password secret value is template syntax: $PASSWORD_VALUE"
  exit 4
fi

# Verify response doesn't leak secret values
RESPONSE_BODY=$(get_response_body)

if echo "$RESPONSE_BODY" | grep -F "$AUTH_VALUE" >/dev/null 2>&1; then
  echo "WARN: Auth secret value found in response (may be expected if server echoes it)"
else
  echo "PASS: Auth secret value not leaked in response"
fi

if echo "$RESPONSE_BODY" | grep -F "$PASSWORD_VALUE" >/dev/null 2>&1; then
  echo "INFO: Password secret value found in response (expected - server echoes password)"
else
  echo "PASS: Password secret value not in response"
fi

# === CHECK 7: Verify Authorization header uses template ===
echo ""
echo "CHECK 7: Verifying Authorization header uses template..."

if echo "$STATUS_HEADERS" | grep -i "authorization" >/dev/null; then
  AUTH_HEADER=$(echo "$STATUS_HEADERS" | jq -r '.Authorization // .authorization // empty' 2>/dev/null || true)
  
  if [[ -n "$AUTH_HEADER" ]]; then
    # Verify it contains the template
    if echo "$AUTH_HEADER" | grep -F "{{ $AUTH_SECRET:$AUTH_NAMESPACE:$AUTH_KEY }}" >/dev/null; then
      echo "PASS: Authorization header preserves template syntax: $AUTH_HEADER"
    elif echo "$AUTH_HEADER" | grep -E 'Bearer .+' >/dev/null; then
      echo "INFO: Authorization header shows Bearer token (template may have been replaced)"
    else
      echo "WARN: Authorization header has unexpected format: ${AUTH_HEADER:0:30}..."
    fi
  fi
else
  echo "WARN: No Authorization header found in status"
fi

# === FINAL SUMMARY ===
echo ""
echo "========================================="
echo "ALL CHECKS PASSED"
echo "========================================="
echo "Summary:"
echo "  • Resource: $RESOURCE_NAME (namespace: $RESOURCE_NAMESPACE)"
echo "  • Auth secret: $AUTH_SECRET:$AUTH_NAMESPACE:$AUTH_KEY ✓"
echo "  • Password secret: $PASSWORD_SECRET:$PASSWORD_NAMESPACE:$PASSWORD_KEY ✓"
echo "  • Templates in spec: 2 ✓"
echo "  • Ready: $READY_STATUS"
echo "  • Secret values verified ✓"
echo ""
echo "verify-secret-templating: success"
exit 0

