#!/usr/bin/env bash
set -euo pipefail

# verify-next-reconcile.sh
# Uptest post-assert hook: validate DisposableRequest nextReconcile behavior

NAMESPACE=${TEST_NAMESPACE:-default}
NAME=${TEST_NAME:-sample-nextreconcile}
NEXT_RECONCILE_SECONDS=${NEXT_RECONCILE_SECONDS:-60}
PRE_WINDOW_SECONDS=${PRE_WINDOW_SECONDS:-30}
SLEEP_BUFFER=${SLEEP_BUFFER:-10}
# Resource kind (namespaced by default). Override with RESOURCE_KIND for cluster tests.
# Default to the .m API group (matches packaged CRD name)
RESOURCE_KIND=${RESOURCE_KIND:-disposablerequests.http.m.crossplane.io}

# Support cluster-scoped resources by conditionally including the namespace flag
if [[ -n "${NAMESPACE:-}" ]]; then
  NS_ARG="-n $NAMESPACE"
else
  NS_ARG=""
fi

echo "verify-next-reconcile: validating $NAME in namespace $NAMESPACE"

get_lr() {
  kubectl get "$RESOURCE_KIND" "$NAME" $NS_ARG -o jsonpath='{.status.lastReconcileTime}' 2>/dev/null || true
}

echo "Waiting for status.lastReconcileTime to be set (timeout 2m)"
LR=""
for i in {1..24}; do
  LR=$(get_lr)
  if [[ -n "$LR" ]]; then
    echo "LastReconcileTime observed: $LR"
    break
  fi
  sleep 5
done

if [[ -z "$LR" ]]; then
  echo "ERROR: LastReconcileTime not set within timeout"
  kubectl $NS_ARG describe "$RESOURCE_KIND" "$NAME" || true
  exit 1
fi

to_epoch() {
  date -u -d "$1" +%s 2>/dev/null || date -u -j -f "%Y-%m-%dT%H:%M:%SZ" "$1" +%s
}

T1=$(to_epoch "$LR")
NOW=$(date -u +%s)
TIME_SINCE_LAST_RECONCILE=$((NOW - T1))

echo "Recorded last reconcile time: $T1 (epoch)"
echo "Current time: $NOW (epoch)"
echo "Time since last reconcile: ${TIME_SINCE_LAST_RECONCILE}s"

# If we're too close to the next reconcile window, wait for it to happen first
# then use that as our baseline
if [[ "$TIME_SINCE_LAST_RECONCILE" -gt "$PRE_WINDOW_SECONDS" ]]; then
  echo "WARNING: Test started late (${TIME_SINCE_LAST_RECONCILE}s after last reconcile)"
  echo "Waiting for next reconcile to establish a fresh baseline..."
  
  # Wait for the next reconcile to happen
  WAIT_FOR_NEXT=$((NEXT_RECONCILE_SECONDS - TIME_SINCE_LAST_RECONCILE + 15))
  if [[ "$WAIT_FOR_NEXT" -gt 0 ]]; then
    sleep $WAIT_FOR_NEXT
  else
    sleep 15
  fi
  
  # Get the new baseline
  LR=$(get_lr)
  T1=$(to_epoch "$LR")
  echo "New baseline established: $LR (epoch: $T1)"
fi

echo "Sleeping $PRE_WINDOW_SECONDS seconds (should NOT see a new reconcile yet)"
sleep $PRE_WINDOW_SECONDS

LR2=$(get_lr)
T2=$(to_epoch "$LR2")

if [[ "$T2" -ne "$T1" ]]; then
  ELAPSED=$((T2 - T1))
  echo "FAIL: LastReconcileTime changed too early after only ${ELAPSED}s (expected no change within ${PRE_WINDOW_SECONDS}s)"
  echo "T1=$T1 ($LR)"
  echo "T2=$T2 ($LR2)"
  kubectl $NS_ARG describe "$RESOURCE_KIND" "$NAME" || true
  exit 2
fi
echo "PASS: No premature reconcile observed"

WAIT_TOTAL=$((NEXT_RECONCILE_SECONDS + SLEEP_BUFFER - PRE_WINDOW_SECONDS))
echo "Waiting until after nextReconcile: sleeping $WAIT_TOTAL seconds"
sleep $WAIT_TOTAL

LR3=$(get_lr)
T3=$(to_epoch "$LR3")

if [[ "$T3" -le "$T1" ]]; then
  echo "FAIL: LastReconcileTime did not update after nextReconcile (T1=$T1, T3=$T3)"
  kubectl $NS_ARG describe "$RESOURCE_KIND" "$NAME" || true
  exit 3
fi

echo "PASS: LastReconcileTime updated after nextReconcile (T1=$T1, T3=$T3)"
echo "verify-next-reconcile: success"
