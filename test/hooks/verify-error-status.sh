#!/usr/bin/env bash
set -euo pipefail

# verify-error-status.sh
# Uptest post-assert hook: validate DisposableRequest error handling

NAMESPACE=${TEST_NAMESPACE:-default}
NAME=${TEST_NAME:-sample-error}
EXPECT_ERROR_REGEX=${EXPECT_ERROR_REGEX:-}
TIMEOUT=${VERIFY_ERROR_TIMEOUT:-120}
# Resource kind (namespaced by default). Override with RESOURCE_KIND env var for cluster tests.
# Default to the .m API group (matches packaged CRD name)
RESOURCE_KIND=${RESOURCE_KIND:-disposablerequests.http.m.crossplane.io}

# Support cluster-scoped resources by conditionally including the namespace flag
if [[ -n "${NAMESPACE:-}" ]]; then
  NS_ARG="-n $NAMESPACE"
else
  NS_ARG=""
fi

# Initialize ACTUAL_CALLS so it is always defined (updated inside CHECK 7 if the
# test-server pod is found; otherwise stays 0 so the final summary never fails
# with an "unbound variable" error under set -u).
ACTUAL_CALLS=0

echo "========================================="
echo "verify-error-status: validating $NAME in namespace $NAMESPACE"
echo "========================================="

# Record test start time to ignore stale logs from previous runs
RUN_START=$(date -u +"%Y/%m/%d %H:%M:%S")
RUN_START_EPOCH=$(date -u -d "$RUN_START" +%s 2>/dev/null || date -u -j -f "%Y/%m/%d %H:%M:%S" "$RUN_START" +%s 2>/dev/null || echo "")
# Prefer to filter logs starting from the resource's LastReconcileTime or creationTimestamp if available
LAST_RECONCILE=$(kubectl get "$RESOURCE_KIND" "$NAME" $NS_ARG -o jsonpath='{.status.lastReconcileTime}' 2>/dev/null || echo "")
LAST_RECONCILE_EPOCH=""
if [[ -n "$LAST_RECONCILE" ]]; then
  LAST_RECONCILE_EPOCH=$(date -u -d "$LAST_RECONCILE" +%s 2>/dev/null || date -u -j -f "%Y-%m-%dT%H:%M:%SZ" "$LAST_RECONCILE" +%s 2>/dev/null || echo "")
fi
# Also get creation timestamp so we can capture calls that occurred at resource creation
CREATION_TS=$(kubectl get "$RESOURCE_KIND" "$NAME" $NS_ARG -o jsonpath='{.metadata.creationTimestamp}' 2>/dev/null || echo "")
CREATION_TS_EPOCH=""
if [[ -n "$CREATION_TS" ]]; then
  CREATION_TS_EPOCH=$(date -u -d "$CREATION_TS" +%s 2>/dev/null || date -u -j -f "%Y-%m-%dT%H:%M:%SZ" "$CREATION_TS" +%s 2>/dev/null || echo "")
fi
# Determine the effective filter start epoch.
# Use the earlier of LastReconcileTime and RUN_START so we don't miss calls
# that occurred immediately before the recorded last reconcile. Apply a small
# backward buffer (OFFSET) to account for clock skew/logging delays.
OFFSET=5
FILTER_SOURCE=""
# Choose the earliest of creationTimestamp, lastReconcile, and test start
FILTER_SOURCE=""
START_EPOCH=""
# pick the earliest non-empty epoch
for e in "$CREATION_TS_EPOCH" "$LAST_RECONCILE_EPOCH" "$RUN_START_EPOCH"; do
  if [[ -n "$e" ]]; then
    if [[ -z "$START_EPOCH" || "$e" -lt "$START_EPOCH" ]]; then
      START_EPOCH=$e
    fi
  fi
done
# determine source label for logging
if [[ -n "$CREATION_TS_EPOCH" && "$CREATION_TS_EPOCH" -eq "$START_EPOCH" ]]; then
  FILTER_SOURCE="CreationTimestamp"
elif [[ -n "$LAST_RECONCILE_EPOCH" && "$LAST_RECONCILE_EPOCH" -eq "$START_EPOCH" ]]; then
  FILTER_SOURCE="LastReconcileTime"
else
  FILTER_SOURCE="TestStart"
fi

# subtract OFFSET but ensure non-negative
if [[ -z "$START_EPOCH" ]]; then
  FILTER_START_EPOCH=$RUN_START_EPOCH
else
  if [[ $START_EPOCH -le $OFFSET ]]; then
    FILTER_START_EPOCH=0
  else
    FILTER_START_EPOCH=$((START_EPOCH - OFFSET))
  fi
fi
# Human-readable start time for logs
FILTER_START_TIME=$(date -u -d "@$FILTER_START_EPOCH" +"%Y/%m/%d %H:%M:%S" 2>/dev/null || date -u -r "$FILTER_START_EPOCH" +"%Y/%m/%d %H:%M:%S" 2>/dev/null || echo "")
if [[ -n "$FILTER_START_TIME" ]]; then
  echo "Filtering logs from $FILTER_SOURCE (UTC): $FILTER_START_TIME (epoch: $FILTER_START_EPOCH)"
else
  echo "Filtering logs from $FILTER_SOURCE (epoch: $FILTER_START_EPOCH)"
fi

get_error() {
  kubectl get "$RESOURCE_KIND" "$NAME" $NS_ARG -o jsonpath='{.status.error}' 2>/dev/null || true
}

get_failed_count() {
  kubectl get "$RESOURCE_KIND" "$NAME" $NS_ARG -o jsonpath='{.status.failed}' 2>/dev/null || echo "0"
}

get_condition() {
  local condition_type=$1
  kubectl get "$RESOURCE_KIND" "$NAME" $NS_ARG -o jsonpath="{.status.conditions[?(@.type=='$condition_type')]}" 2>/dev/null || true
}

get_condition_status() {
  local condition_type=$1
  kubectl get "$RESOURCE_KIND" "$NAME" $NS_ARG -o jsonpath="{.status.conditions[?(@.type=='$condition_type')].status}" 2>/dev/null || echo "Unknown"
}

get_condition_message() {
  local condition_type=$1
  kubectl get "$RESOURCE_KIND" "$NAME" $NS_ARG -o jsonpath="{.status.conditions[?(@.type=='$condition_type')].message}" 2>/dev/null || echo ""
}

# === CHECK 1: Wait for status.error to be set ===
echo ""
echo "CHECK 1: Verifying status.error is populated..."

# Wait until status.error is set (timeout)
ERROR=""
end=$((SECONDS + TIMEOUT))
while [ $SECONDS -lt $end ]; do
  ERROR=$(get_error)
  if [[ -n "$ERROR" ]]; then
    echo "PASS: Observed status.error: $ERROR"
    break
  fi
  sleep 3
done

if [[ -z "$ERROR" ]]; then
  echo "FAIL: status.error not set within timeout"
  kubectl $NS_ARG describe "$RESOURCE_KIND" "$NAME" || true
  exit 1
fi

# Optional regex validation
if [[ -n "$EXPECT_ERROR_REGEX" ]]; then
  if ! echo "$ERROR" | grep -E "${EXPECT_ERROR_REGEX}" >/dev/null; then
    echo "FAIL: status.error does not match EXPECT_ERROR_REGEX (${EXPECT_ERROR_REGEX})"
    echo "status.error: $ERROR"
    exit 2
  fi
  echo "PASS: status.error matches expected pattern"
fi

# === CHECK 2: Verify status.failed counter ===
echo ""
echo "CHECK 2: Verifying status.failed counter..."
RETRIES_LIMIT=$(kubectl get "$RESOURCE_KIND" "$NAME" -n "$NAMESPACE" -o jsonpath='{.spec.forProvider.rollbackRetriesLimit}' 2>/dev/null || echo "1")
FAILED_COUNT=$(get_failed_count)

echo "rollbackRetriesLimit: $RETRIES_LIMIT"
echo "status.failed: $FAILED_COUNT"

# Failed count should be <= rollbackRetriesLimit (1 initial attempt + retries)
if [[ "$FAILED_COUNT" -gt "$RETRIES_LIMIT" ]]; then
  echo "WARN: status.failed ($FAILED_COUNT) exceeds rollbackRetriesLimit ($RETRIES_LIMIT)"
  echo "This may indicate retry logic is not stopping properly"
elif [[ "$FAILED_COUNT" -eq 0 ]]; then
  echo "FAIL: status.failed is 0 but status.error is set - counter not incrementing"
  exit 3
else
  echo "PASS: status.failed counter is within expected range"
fi

# === CHECK 3: Verify ErrorObserved condition exists ===
echo ""
echo "CHECK 3: Verifying ErrorObserved condition..."
ERROR_OBSERVED=$(get_condition "ErrorObserved")

if [[ -z "$ERROR_OBSERVED" ]]; then
  echo "FAIL: ErrorObserved condition not found"
  echo "Available conditions:"
  kubectl get "$RESOURCE_KIND" "$NAME" -n "$NAMESPACE" -o jsonpath='{.status.conditions[*].type}' 2>/dev/null || true
  exit 4
fi

ERROR_OBSERVED_STATUS=$(get_condition_status "ErrorObserved")
if [[ "$ERROR_OBSERVED_STATUS" != "True" ]]; then
  echo "FAIL: ErrorObserved condition status is '$ERROR_OBSERVED_STATUS', expected 'True'"
  exit 5
fi

echo "PASS: ErrorObserved condition exists with status=True"

# === CHECK 4: Verify ErrorObserved message matches status.error ===
echo ""
echo "CHECK 4: Verifying ErrorObserved message consistency..."
ERROR_OBSERVED_MESSAGE=$(get_condition_message "ErrorObserved")

if [[ -z "$ERROR_OBSERVED_MESSAGE" ]]; then
  echo "WARN: ErrorObserved condition has no message"
elif [[ "$ERROR_OBSERVED_MESSAGE" != "$ERROR" ]]; then
  echo "WARN: ErrorObserved message doesn't match status.error"
  echo "  ErrorObserved.message: $ERROR_OBSERVED_MESSAGE"
  echo "  status.error:          $ERROR"
else
  echo "PASS: ErrorObserved message matches status.error"
fi

# === CHECK 5: Verify Ready condition is False ===
echo ""
echo "CHECK 5: Verifying Ready condition..."
READY_STATUS=$(get_condition_status "Ready")

if [[ "$READY_STATUS" == "False" ]]; then
  echo "PASS: Ready condition is False (resource not healthy)"
elif [[ "$READY_STATUS" == "True" ]]; then
  echo "FAIL: Ready condition is True but resource has an error"
  exit 6
else
  echo "WARN: Ready condition status is '$READY_STATUS'"
fi

# === CHECK 6: Verify Synced condition is True ===
echo ""
echo "CHECK 6: Verifying Synced condition..."
SYNCED_STATUS=$(get_condition_status "Synced")

if [[ "$SYNCED_STATUS" == "True" ]]; then
  echo "PASS: Synced condition is True (reconciliation completed)"
elif [[ "$SYNCED_STATUS" == "False" ]]; then
  echo "WARN: Synced condition is False - reconciliation may still be in progress"
else
  echo "WARN: Synced condition status is '$SYNCED_STATUS'"
fi

# === CHECK 7: Verify API call count (no infinite retries) ===
echo ""
echo "CHECK 7: Verifying API is not being called repeatedly..."

# Expected number of actual HTTP requests = rollbackRetriesLimit
# Each retry attempt makes one API call
EXPECTED_CALLS=$RETRIES_LIMIT
echo "Expected API calls from provider: $EXPECTED_CALLS (rollbackRetriesLimit)"

# Extract the endpoint path from the resource URL
FULL_URL=$(kubectl get "$RESOURCE_KIND" "$NAME" -n "$NAMESPACE" -o jsonpath='{.spec.forProvider.url}' 2>/dev/null || echo "")
ENDPOINT_PATH=$(echo "$FULL_URL" | sed 's|http[s]*://[^/]*/|/|')
echo "Testing endpoint: $ENDPOINT_PATH"

# Get test-server pod logs and count POST requests to our endpoint
TEST_SERVER_POD=$(kubectl get pods -n "$NAMESPACE" -l app=test-server -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)

if [[ -n "$TEST_SERVER_POD" ]]; then
  # Wait a bit to ensure all logs are flushed
  sleep 5

  # Collect recent test-server logs and only consider lines after RUN_START
  LOGS=$(kubectl logs -n "$NAMESPACE" "$TEST_SERVER_POD" 2>/dev/null || echo "")

  TOTAL_API_CALLS=0
  CALL_LINES=()
  while IFS= read -r line; do
    # Each log line starts with timestamp like: 2025/12/21 00:44:48 ...
    ts=$(echo "$line" | awk '{print $1" "$2}')
    EPOCH=$(date -u -d "$ts" +%s 2>/dev/null || date -u -j -f "%Y/%m/%d %H:%M:%S" "$ts" +%s 2>/dev/null || echo "")
    if [[ -n "$EPOCH" && -n "$FILTER_START_EPOCH" && "$EPOCH" -ge "$FILTER_START_EPOCH" ]]; then
      if echo "$line" | grep -q "\[POST\] $ENDPOINT_PATH" && echo "$line" | grep -q "Go-http-client"; then
        TOTAL_API_CALLS=$((TOTAL_API_CALLS+1))
        CALL_LINES+=("$line")
      fi
    fi
  done <<< "$LOGS"

  echo "INFO: Found $TOTAL_API_CALLS $ENDPOINT_PATH calls in logs since test start"

  # If there are more calls than expected, fail immediately — extra calls are unexpected
  if [[ "$TOTAL_API_CALLS" -gt "$EXPECTED_CALLS" ]]; then
    echo "FAIL: API called $TOTAL_API_CALLS times (expected $EXPECTED_CALLS)"
    echo "Retry logic is NOT stopping - extra retries detected!"
    echo "Recent test-server logs (tail 50):"
    kubectl logs -n "$NAMESPACE" "$TEST_SERVER_POD" --tail=50 2>/dev/null || true
    exit 7
  fi

  ACTUAL_CALLS=$TOTAL_API_CALLS

  echo "Actual API calls from provider: $ACTUAL_CALLS"

  # Strict check: we expect exactly EXPECTED_CALLS attempts when status.failed equals that limit
  if [[ "$ACTUAL_CALLS" -eq "$EXPECTED_CALLS" ]]; then
    echo "PASS: API called exactly $EXPECTED_CALLS time(s) - retries properly stopped"
  else
    echo "FAIL: API called $ACTUAL_CALLS times (expected exactly $EXPECTED_CALLS)."
    # Provide helpful diagnostics
    echo "  • Resource .status.failed = $FAILED_COUNT"
    echo "  • Expected based on rollbackRetriesLimit = $EXPECTED_CALLS"
    echo "Dumping recent test-server logs (tail 100) for diagnosis:"
    kubectl logs -n "$NAMESPACE" "$TEST_SERVER_POD" --tail=100 2>/dev/null || true
    echo "Dumping resource for inspection:"
    kubectl -n "$NAMESPACE" get "$RESOURCE_KIND" "$NAME" -o yaml || true
    exit 12
  fi

  # Save recent call lines for timestamp checks (guard against unset array)
  if [[ -n "${CALL_LINES+x}" && ${#CALL_LINES[@]} -gt 0 ]]; then
    RECENT_CALL_LINES=$(printf "%s\n" "${CALL_LINES[@]}")
  else
    RECENT_CALL_LINES=""
  fi

else
  echo "INFO: Could not find test-server pod to verify API call count, skipping this check"
fi

# === CHECK 7.5: Verify retries respect nextReconcile timing ===
echo ""
echo "CHECK 7.5: Verifying retries respect nextReconcile timing..."

# Get nextReconcile value from spec
NEXT_RECONCILE=$(kubectl get "$RESOURCE_KIND" "$NAME" -n "$NAMESPACE" -o jsonpath='{.spec.forProvider.nextReconcile}' 2>/dev/null || echo "")

if [[ -n "$NEXT_RECONCILE" && -n "$TEST_SERVER_POD" && "$ACTUAL_CALLS" -gt 1 ]]; then
  # Parse nextReconcile to seconds (e.g., "10s" -> 10)
  if [[ "$NEXT_RECONCILE" =~ ^([0-9]+)s$ ]]; then
    NEXT_RECONCILE_SECONDS="${BASH_REMATCH[1]}"
    echo "nextReconcile: ${NEXT_RECONCILE_SECONDS}s"
    
    # Extract timestamps of POST requests to our endpoint from Go client
    # Log format: 2025/12/21 00:44:48 [POST] $ENDPOINT_PATH ... Go-http-client/...
    # Only get the most recent N calls (where N = EXPECTED_CALLS)
    TIMESTAMPS=$(kubectl logs -n "$NAMESPACE" "$TEST_SERVER_POD" 2>/dev/null | \
      grep "\[POST\] $ENDPOINT_PATH" | \
      grep "Go-http-client" | \
      tail -n "$EXPECTED_CALLS" | \
      awk '{print $1, $2}' || echo "")
    
    if [[ -n "$TIMESTAMPS" ]]; then
      # Convert timestamps to epoch and calculate gaps
      declare -a EPOCHS
      while IFS= read -r ts; do
        if [[ -n "$ts" ]]; then
          # Convert timestamp to epoch (format: 2025/12/21 00:44:48)
          EPOCH=$(date -u -d "$ts" +%s 2>/dev/null || date -u -j -f "%Y/%m/%d %H:%M:%S" "$ts" +%s 2>/dev/null || echo "")
          if [[ -n "$EPOCH" ]]; then
            EPOCHS+=("$EPOCH")
          fi
        fi
      done <<< "$TIMESTAMPS"
      
      # Check gaps between consecutive calls
      if [[ "${#EPOCHS[@]}" -ge 2 ]]; then
        echo "Found ${#EPOCHS[@]} API calls with timestamps"
        IMMEDIATE_RETRY_DETECTED=false
        TOLERANCE=5  # Allow 5 second tolerance
        
        for (( i=1; i<${#EPOCHS[@]}; i++ )); do
          PREV=${EPOCHS[$((i-1))]}
          CURR=${EPOCHS[$i]}
          GAP=$((CURR - PREV))
          
          echo "  Call $i -> Call $((i+1)): ${GAP}s gap"
          
          # Check if gap is too small (immediate retry)
          if [[ "$GAP" -lt "$TOLERANCE" ]]; then
            IMMEDIATE_RETRY_DETECTED=true
            echo "IMMEDIATE RETRY DETECTED (gap: ${GAP}s < ${TOLERANCE}s)"
          elif [[ "$GAP" -lt $((NEXT_RECONCILE_SECONDS - TOLERANCE)) ]]; then
            echo "Retry too fast (gap: ${GAP}s < expected: ${NEXT_RECONCILE_SECONDS}s)"
            IMMEDIATE_RETRY_DETECTED=true
          fi
        done
        
        if [[ "$IMMEDIATE_RETRY_DETECTED" == true ]]; then
          echo "FAIL: Retries fired immediately without respecting nextReconcile"
          echo "retries should wait ${NEXT_RECONCILE_SECONDS}s between attempts"
          exit 10
        else
          echo "PASS: Retries respect nextReconcile timing (${NEXT_RECONCILE_SECONDS}s between attempts)"
        fi

        # Extra validation: wait one additional nextReconcile interval (+tolerance) to ensure no further calls occur
        echo "Waiting an additional $((NEXT_RECONCILE_SECONDS + 5))s to ensure no extra calls appear..."
        sleep $((NEXT_RECONCILE_SECONDS + 5))

        # Re-count calls since filter start
        LOGS_AFTER=$(kubectl logs -n "$NAMESPACE" "$TEST_SERVER_POD" 2>/dev/null || echo "")
        NEW_TOTAL_API_CALLS=0
        while IFS= read -r line; do
          ts=$(echo "$line" | awk '{print $1" "$2}')
          EPOCH=$(date -u -d "$ts" +%s 2>/dev/null || date -u -j -f "%Y/%m/%d %H:%M:%S" "$ts" +%s 2>/dev/null || echo "")
          if [[ -n "$EPOCH" && -n "$FILTER_START_EPOCH" && "$EPOCH" -ge "$FILTER_START_EPOCH" ]]; then
            if echo "$line" | grep -q "\[POST\] $ENDPOINT_PATH" && echo "$line" | grep -q "Go-http-client"; then
              NEW_TOTAL_API_CALLS=$((NEW_TOTAL_API_CALLS+1))
            fi
          fi
        done <<< "$LOGS_AFTER"

        if [[ "$NEW_TOTAL_API_CALLS" -gt "$EXPECTED_CALLS" ]]; then
          echo "FAIL: Extra API call detected after waiting ${NEXT_RECONCILE_SECONDS}s (calls: $NEW_TOTAL_API_CALLS, expected: $EXPECTED_CALLS)"
          exit 11
        elif [[ "$NEW_TOTAL_API_CALLS" -lt "$EXPECTED_CALLS" ]]; then
          echo "FAIL: Fewer API calls than expected after waiting (calls: $NEW_TOTAL_API_CALLS, expected: $EXPECTED_CALLS)"
          echo "Dumping recent test-server logs (tail 200) for diagnosis:"
          kubectl logs -n "$NAMESPACE" "$TEST_SERVER_POD" --tail=200 2>/dev/null || true
          kubectl $NS_ARG get "$RESOURCE_KIND" "$NAME" -o yaml || true
          exit 13
        else
          echo "PASS: No extra API calls observed after waiting"
        fi
      else
        echo "INFO: Not enough calls with timestamps to verify gaps"
      fi
    else
      echo "INFO: Could not extract timestamps from logs"
    fi
  else
    echo "INFO: nextReconcile format not recognized: $NEXT_RECONCILE"
  fi
else
  if [[ -z "$NEXT_RECONCILE" ]]; then
    echo "INFO: nextReconcile not set, skipping timing validation"
  elif [[ -z "$TEST_SERVER_POD" ]]; then
    echo "INFO: Test server pod not found, skipping timing validation"
  elif [[ "$ACTUAL_CALLS" -le 1 ]]; then
    echo "INFO: Only one API call detected, no retry timing to verify"
  fi
fi

# === CHECK 8: Verify error persists across reconciliations ===
echo ""
echo "CHECK 8: Verifying error state persists..."
sleep 3  # Wait for potential additional reconcile cycles

ERROR_AFTER=$(get_error)
FAILED_AFTER=$(get_failed_count)

if [[ "$ERROR_AFTER" != "$ERROR" ]]; then
  echo "FAIL: status.error changed from '$ERROR' to '$ERROR_AFTER'"
  echo "Error state is not persisting across reconciliations!"
  exit 8
fi

if [[ "$FAILED_AFTER" != "$FAILED_COUNT" ]]; then
  echo "FAIL: status.failed changed from $FAILED_COUNT to $FAILED_AFTER"
  echo "Failed counter is still incrementing - retries not stopped!"
  exit 9
fi

echo "PASS: Error state persists (not being overwritten)"

# === FINAL SUMMARY ===
echo ""
echo "========================================="
echo "ALL CHECKS PASSED"
echo "========================================="
echo "Summary:"
echo "  • status.error: $ERROR"
echo "  • status.failed: $FAILED_COUNT (limit: $RETRIES_LIMIT)"
echo "  • ErrorObserved: True"
echo "  • Ready: $READY_STATUS"
echo "  • Synced: $SYNCED_STATUS"
echo "  • API calls: $ACTUAL_CALLS (expected: $EXPECTED_CALLS)"
echo ""
echo "verify-error-status: success"
