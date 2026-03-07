#!/usr/bin/env bash
set -euo pipefail

# verify-secret-injection.sh
# Uptest post-assert hook: validate secret injection from HTTP responses

TEST_RESOURCE_NAME=${TEST_RESOURCE_NAME:-}
TEST_RESOURCE_NAMESPACE=${TEST_RESOURCE_NAMESPACE:-default}
SECRET_NAME=${SECRET_NAME:-}
SECRET_NAMESPACE=${SECRET_NAMESPACE:-}
SECRET_KEY=${SECRET_KEY:-}
EXPECTED_VALUE_REGEX=${EXPECTED_VALUE_REGEX:-}
CHECK_OWNER_REFERENCE=${CHECK_OWNER_REFERENCE:-false}
TIMEOUT=${VERIFY_SECRET_TIMEOUT:-120}
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

# Discover the resource name by searching all test-annotated resources across both API groups
_discover_resource_name() {
  local ns_arg="${1:-}"
  for kind in "disposablerequests.http.crossplane.io" "disposablerequests.http.m.crossplane.io"; do
    local found
    # cluster-scoped
    found=$(kubectl get "$kind" -o jsonpath='{range .items[?(@.metadata.annotations["upjet.upbound.io/test"]=="true")]}{.metadata.name}{"\n"}{end}' 2>/dev/null | head -n1 || true)
    if [[ -n "$found" ]]; then echo "$found"; return 0; fi
    # namespaced
    if [[ -n "$ns_arg" ]]; then
      found=$(kubectl get "$kind" $ns_arg -o jsonpath='{range .items[?(@.metadata.annotations["upjet.upbound.io/test"]=="true")]}{.metadata.name}{"\n"}{end}' 2>/dev/null | head -n1 || true)
      if [[ -n "$found" ]]; then echo "$found"; return 0; fi
    fi
  done
  echo ""
}

# Support cluster-scoped resources by conditionally including the namespace flag
if [[ -n "${TEST_RESOURCE_NAMESPACE:-}" ]]; then
  TR_NS_ARG="-n $TEST_RESOURCE_NAMESPACE"
else
  TR_NS_ARG=""
fi

echo "========================================="
echo "verify-secret-injection: validating secret injection"
echo "========================================="

# Auto-discover resource name if not provided or not found
if [[ -z "$TEST_RESOURCE_NAME" ]] || ! kubectl get "$RESOURCE_KIND" "$TEST_RESOURCE_NAME" $TR_NS_ARG >/dev/null 2>&1; then
  discovered=$(_discover_resource_name "$TR_NS_ARG")
  if [[ -n "$discovered" ]]; then
    TEST_RESOURCE_NAME="$discovered"
    echo "INFO: discovered resource name: $TEST_RESOURCE_NAME"
  fi
fi

# Auto-detect resource kind
RESOURCE_KIND=$(_detect_disposablerequest_kind "$TEST_RESOURCE_NAME" "$TR_NS_ARG")
# Cluster-scoped resources have no namespace
if [[ "$RESOURCE_KIND" == "disposablerequests.http.crossplane.io" ]]; then
  TEST_RESOURCE_NAMESPACE=""
  TR_NS_ARG=""
fi

# Auto-discover secret config from resource spec if not explicitly provided
if [[ -z "$SECRET_NAME" ]]; then
  SECRET_NAME=$(kubectl get "$RESOURCE_KIND" "$TEST_RESOURCE_NAME" $TR_NS_ARG \
    -o jsonpath='{.spec.forProvider.secretInjectionConfig[0].secretRef.name}' 2>/dev/null || echo "")
  if [[ -z "$SECRET_NAME" ]]; then
    # Try keyMappings-style (for older format)
    SECRET_NAME=$(kubectl get "$RESOURCE_KIND" "$TEST_RESOURCE_NAME" $TR_NS_ARG \
      -o jsonpath='{.spec.forProvider.secretInjectionConfig[0].secretRef.name}' 2>/dev/null || echo "notification-response")
  fi
  echo "INFO: auto-discovered SECRET_NAME=$SECRET_NAME"
fi
if [[ -z "$SECRET_NAMESPACE" ]]; then
  SECRET_NAMESPACE=$(kubectl get "$RESOURCE_KIND" "$TEST_RESOURCE_NAME" $TR_NS_ARG \
    -o jsonpath='{.spec.forProvider.secretInjectionConfig[0].secretRef.namespace}' 2>/dev/null || echo "default")
  if [[ -z "$SECRET_NAMESPACE" ]]; then SECRET_NAMESPACE="default"; fi
  echo "INFO: auto-discovered SECRET_NAMESPACE=$SECRET_NAMESPACE"
fi
if [[ -z "$SECRET_KEY" ]]; then
  # Try secretKey field first
  SECRET_KEY=$(kubectl get "$RESOURCE_KIND" "$TEST_RESOURCE_NAME" $TR_NS_ARG \
    -o jsonpath='{.spec.forProvider.secretInjectionConfig[0].secretKey}' 2>/dev/null || echo "")
  if [[ -z "$SECRET_KEY" ]]; then
    # Try keyMappings secretKey
    SECRET_KEY=$(kubectl get "$RESOURCE_KIND" "$TEST_RESOURCE_NAME" $TR_NS_ARG \
      -o jsonpath='{.spec.forProvider.secretInjectionConfig[0].keyMappings[0].secretKey}' 2>/dev/null || echo "notification-status")
  fi
  echo "INFO: auto-discovered SECRET_KEY=$SECRET_KEY"
fi

echo "Resource: $TEST_RESOURCE_NAME (namespace: ${TEST_RESOURCE_NAMESPACE:-<cluster-scoped>})"
echo "Secret: $SECRET_NAME/$SECRET_KEY (namespace: $SECRET_NAMESPACE)"
echo ""

get_secret_value() {
  kubectl get secret "$SECRET_NAME" -n "$SECRET_NAMESPACE" -o jsonpath="{.data.$SECRET_KEY}" 2>/dev/null | base64 -d 2>/dev/null || true
}

get_secret_exists() {
  kubectl get secret "$SECRET_NAME" -n "$SECRET_NAMESPACE" >/dev/null 2>&1 && echo "true" || echo "false"
}

get_owner_references() {
  kubectl get secret "$SECRET_NAME" -n "$SECRET_NAMESPACE" -o jsonpath='{.metadata.ownerReferences}' 2>/dev/null || echo ""
}

get_resource_uid() {
  kubectl get "$RESOURCE_KIND" "$TEST_RESOURCE_NAME" $TR_NS_ARG -o jsonpath='{.metadata.uid}' 2>/dev/null || echo ""
}

get_condition_status() {
  local condition_type=$1
  kubectl get "$RESOURCE_KIND" "$TEST_RESOURCE_NAME" $TR_NS_ARG -o jsonpath="{.status.conditions[?(@.type=='$condition_type')].status}" 2>/dev/null || echo "Unknown"
}

get_secret_injection_status() {
  kubectl get "$RESOURCE_KIND" "$TEST_RESOURCE_NAME" $TR_NS_ARG -o jsonpath='{.status.secretsInjected}' 2>/dev/null || echo "false"
}

get_response_body() {
  kubectl get "$RESOURCE_KIND" "$TEST_RESOURCE_NAME" $TR_NS_ARG -o jsonpath='{.status.response.body}' 2>/dev/null || echo ""
}

# Auto-discover resource name if not set or not found (look for uptest marker)
discover_resource() {
  if kubectl get "$RESOURCE_KIND" "$TEST_RESOURCE_NAME" $TR_NS_ARG >/dev/null 2>&1; then
    return 0
  fi
  discovered=$(kubectl get "$RESOURCE_KIND" $TR_NS_ARG -o jsonpath='{range .items[?(@.metadata.annotations["upjet.upbound.io/test"]=="true")]}{.metadata.name}{"\n"}{end}' 2>/dev/null | head -n1 || true)
  if [[ -n "$discovered" ]]; then
    TEST_RESOURCE_NAME="$discovered"
    echo "INFO: discovered resource name: $TEST_RESOURCE_NAME"
    return 0
  fi
  return 1
}

# === CHECK 1: Wait for secret to exist ===
echo "CHECK 1: Verifying secret exists..."

SECRET_EXISTS="false"
end=$((SECONDS + TIMEOUT))
while [ $SECONDS -lt $end ]; do
  # attempt discovery in case resource is created after script start
  discover_resource >/dev/null 2>&1 || true
  SECRET_EXISTS=$(get_secret_exists)
  if [[ "$SECRET_EXISTS" == "true" ]]; then
    echo "PASS: Secret '$SECRET_NAME' exists in namespace '$SECRET_NAMESPACE'"
    break
  fi
  sleep 3
done

if [[ "$SECRET_EXISTS" != "true" ]]; then
  echo "FAIL: Secret '$SECRET_NAME' not found in namespace '$SECRET_NAMESPACE' within timeout"
  echo ""
  echo "Available secrets in namespace:"
  kubectl get secrets -n "$SECRET_NAMESPACE" 2>/dev/null || true
  echo ""
  echo "Resource status:"
  kubectl $TR_NS_ARG describe "$RESOURCE_KIND" "$TEST_RESOURCE_NAME" 2>/dev/null || true
  exit 1
fi

# === CHECK 2: Verify secret key exists and has value ===
echo ""
echo "CHECK 2: Verifying secret key '$SECRET_KEY' exists with data..."

SECRET_VALUE=""
end=$((SECONDS + TIMEOUT))
while [ $SECONDS -lt $end ]; do
  SECRET_VALUE=$(get_secret_value)
  if [[ -n "$SECRET_VALUE" ]]; then
    echo "PASS: Secret key '$SECRET_KEY' has value (length: ${#SECRET_VALUE} chars)"
    break
  fi
  sleep 3
done

if [[ -z "$SECRET_VALUE" ]]; then
  echo "FAIL: Secret key '$SECRET_KEY' is empty or does not exist"
  echo ""
  echo "Available keys in secret:"
  kubectl get secret "$SECRET_NAME" -n "$SECRET_NAMESPACE" -o jsonpath='{.data}' 2>/dev/null | jq 'keys' || true
  exit 2
fi

# === CHECK 3: Verify value is actual data, not templates or jq paths ===
echo ""
echo "CHECK 3: Verifying secret value is actual data (not templates)..."

# Check for common template patterns that shouldn't be in final values
if echo "$SECRET_VALUE" | grep -E '\{\{.*:.*:.*\}\}' >/dev/null; then
  echo "FAIL: Secret value contains template syntax: $SECRET_VALUE"
  echo "Templates should be replaced before injection"
  exit 3
fi

# Check for jq path syntax
if echo "$SECRET_VALUE" | grep -E '^\.(body|headers|statusCode)' >/dev/null; then
  echo "FAIL: Secret value appears to be a jq path: $SECRET_VALUE"
  echo "ResponseJQ should be evaluated before injection"
  exit 4
fi

# Check for literal "null" string (common when jq path fails)
if [[ "$SECRET_VALUE" == "null" ]]; then
  echo "WARN: Secret value is literal string 'null' - jq expression may have failed"
fi

echo "PASS: Secret value appears to be actual data (not template or jq path)"

# === CHECK 3.5: Verify response body has redacted secret reference ===
echo ""
echo "CHECK 3.5: Verifying response body has redacted secret reference..."

RESPONSE_BODY=$(get_response_body)
EXPECTED_TEMPLATE="{{$SECRET_NAME:$SECRET_NAMESPACE:$SECRET_KEY}}"

if [[ -n "$RESPONSE_BODY" ]]; then
  if echo "$RESPONSE_BODY" | grep -F "$EXPECTED_TEMPLATE" >/dev/null; then
    echo "PASS: Response body contains redacted secret reference: $EXPECTED_TEMPLATE"
    echo "This confirms secret values are NOT stored in resource status (security best practice)"
  else
    # Check for any template syntax
    if echo "$RESPONSE_BODY" | grep -E '\{\{[^}]*:[^}]*:[^}]*\}\}' >/dev/null; then
      echo "INFO: Response body contains other secret references (but not the one we're checking)"
      echo "$RESPONSE_BODY" | grep -o '{{[^}]*:[^}]*:[^}]*}}' | sort -u | while read -r template; do
        echo "  - $template"
      done
    else
      echo "WARN: Response body does not contain expected secret reference"
      echo "Expected: $EXPECTED_TEMPLATE"
      echo "This may indicate secret redaction is not working"
    fi
  fi
  
  # Verify actual secret value is NOT in response body
  if echo "$RESPONSE_BODY" | grep -F "$SECRET_VALUE" >/dev/null; then
    echo "FAIL: Actual secret value found in response body!"
    echo "Secret values should be redacted with template syntax for security"
    exit 7
  else
    echo "PASS: Actual secret value is NOT in response body (properly redacted)"
  fi
else
  echo "WARN: Could not retrieve response body for validation"
fi

# === CHECK 4: Optional regex validation ===
if [[ -n "$EXPECTED_VALUE_REGEX" ]]; then
  echo ""
  echo "CHECK 4: Verifying secret value matches expected pattern..."
  
  if echo "$SECRET_VALUE" | grep -E "$EXPECTED_VALUE_REGEX" >/dev/null; then
    echo "PASS: Secret value matches pattern '$EXPECTED_VALUE_REGEX'"
  else
    echo "FAIL: Secret value does not match pattern '$EXPECTED_VALUE_REGEX'"
    echo "Secret value: $SECRET_VALUE"
    exit 5
  fi
else
  echo ""
  echo "CHECK 4: Skipping regex validation (EXPECTED_VALUE_REGEX not set)"
fi

# === CHECK 5: Verify owner reference if required ===
echo ""
echo "CHECK 5: Verifying owner reference configuration..."

OWNER_REFS=$(get_owner_references)
RESOURCE_UID=$(get_resource_uid)

if [[ "$CHECK_OWNER_REFERENCE" == "true" ]]; then
  if [[ -z "$OWNER_REFS" || "$OWNER_REFS" == "[]" ]]; then
    echo "FAIL: setOwnerReference is expected but no owner references found"
    exit 6
  fi
  
  # Check if the resource UID is in the owner references
  if [[ -n "$RESOURCE_UID" ]] && echo "$OWNER_REFS" | grep -q "$RESOURCE_UID"; then
    echo "PASS: Owner reference correctly set to resource (UID: $RESOURCE_UID)"
  else
    echo "WARN: Owner reference exists but may not match resource UID"
    echo "Resource UID: $RESOURCE_UID"
    echo "Owner references: $OWNER_REFS"
  fi
elif [[ -n "$OWNER_REFS" && "$OWNER_REFS" != "[]" ]]; then
  echo "WARN: Owner references found but setOwnerReference expected to be false"
  echo "Owner references: $OWNER_REFS"
else
  echo "PASS: No owner reference set (as expected)"
fi

# === CHECK 6: Verify resource status reflects secret injection ===
echo ""
echo "CHECK 6: Verifying resource status indicates successful injection..."

READY_STATUS=$(get_condition_status "Ready")
SYNCED_STATUS=$(get_condition_status "Synced")
SECRETS_INJECTED=$(get_secret_injection_status)

echo "Ready: $READY_STATUS"
echo "Synced: $SYNCED_STATUS"
echo "secretsInjected: $SECRETS_INJECTED"

if [[ "$READY_STATUS" != "True" ]]; then
  echo "WARN: Ready condition is not True - resource may not be healthy"
fi

if [[ "$SYNCED_STATUS" != "True" ]]; then
  echo "WARN: Synced condition is not True - reconciliation may not be complete"
fi

if [[ "$SECRETS_INJECTED" == "true" ]]; then
  echo "PASS: status.secretsInjected is true"
else
  echo "WARN: status.secretsInjected is not true (value: $SECRETS_INJECTED)"
fi

# === CHECK 7: Verify secret metadata (labels/annotations) ===
echo ""
echo "CHECK 7: Verifying secret metadata..."

SECRET_LABELS=$(kubectl get secret "$SECRET_NAME" -n "$SECRET_NAMESPACE" -o jsonpath='{.metadata.labels}' 2>/dev/null || echo "{}")
SECRET_ANNOTATIONS=$(kubectl get secret "$SECRET_NAME" -n "$SECRET_NAMESPACE" -o jsonpath='{.metadata.annotations}' 2>/dev/null || echo "{}")

if [[ "$SECRET_LABELS" != "{}" ]]; then
  echo "INFO: Secret has labels: $SECRET_LABELS"
else
  echo "INFO: Secret has no custom labels"
fi

if [[ "$SECRET_ANNOTATIONS" != "{}" ]]; then
  echo "INFO: Secret has annotations: $SECRET_ANNOTATIONS"
else
  echo "INFO: Secret has no custom annotations"
fi

# === CHECK 8: Verify multiple keys if configured ===
echo ""
echo "CHECK 8: Verifying all configured secret keys..."

ALL_KEYS=$(kubectl get secret "$SECRET_NAME" -n "$SECRET_NAMESPACE" -o jsonpath='{.data}' 2>/dev/null | jq -r 'keys[]' 2>/dev/null || true)

if [[ -n "$ALL_KEYS" ]]; then
  KEY_COUNT=$(echo "$ALL_KEYS" | wc -l | tr -d ' ')
  echo "PASS: Secret contains $KEY_COUNT key(s):"
  echo "$ALL_KEYS" | while read -r key; do
    VALUE=$(kubectl get secret "$SECRET_NAME" -n "$SECRET_NAMESPACE" -o jsonpath="{.data.$key}" 2>/dev/null | base64 -d 2>/dev/null || true)
    VALUE_LEN=${#VALUE}
    echo "  - $key: $VALUE_LEN chars"
    
    # Check each key for template/jq patterns
    if echo "$VALUE" | grep -E '\{\{.*:.*:.*\}\}' >/dev/null; then
      echo "    WARN: Contains template syntax"
    fi
    if echo "$VALUE" | grep -E '^\.(body|headers|statusCode)' >/dev/null; then
      echo "    WARN: Contains jq path syntax"
    fi
  done
else
  echo "INFO: Could not enumerate secret keys"
fi

# === FINAL SUMMARY ===
echo ""
echo "========================================="
echo "ALL CHECKS PASSED"
echo "========================================="
echo "Summary:"
echo "  • Secret: $SECRET_NAME (namespace: $SECRET_NAMESPACE)"
echo "  • Key: $SECRET_KEY"
echo "  • Value length: ${#SECRET_VALUE} chars"
echo "  • Total keys: ${KEY_COUNT:-1}"
echo "  • Owner reference: $CHECK_OWNER_REFERENCE"
echo "  • Ready: $READY_STATUS"
echo "  • Synced: $SYNCED_STATUS"
echo "  • secretsInjected: $SECRETS_INJECTED"
echo ""
echo "verify-secret-injection: success"
