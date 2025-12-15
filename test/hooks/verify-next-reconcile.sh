#!/usr/bin/env bash
set -euo pipefail

# verify-next-reconcile.sh
# Uptest post-assert hook: validate DisposableRequest nextReconcile behavior

NAMESPACE=${TEST_NAMESPACE:-default}
NAME=${TEST_NAME:-sample-nextreconcile}
NEXT_RECONCILE_SECONDS=${NEXT_RECONCILE_SECONDS:-60}
PRE_WINDOW_SECONDS=${PRE_WINDOW_SECONDS:-30}
SLEEP_BUFFER=${SLEEP_BUFFER:-10}

echo "verify-next-reconcile: validating $NAME in namespace $NAMESPACE"

get_lr() {
  kubectl get disposablerequests.http.m.crossplane.io "$NAME" -n "$NAMESPACE" -o jsonpath='{.status.lastReconcileTime}' 2>/dev/null || true
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
  kubectl -n "$NAMESPACE" describe disposablerequests.http.m.crossplane.io "$NAME" || true
  exit 1
fi

to_epoch() {
  date -u -d "$1" +%s 2>/dev/null || date -u -j -f "%Y-%m-%dT%H:%M:%SZ" "$1" +%s
}

T1=$(to_epoch "$LR")
echo "Recorded last reconcile time: $T1 (epoch)"

echo "Sleeping $PRE_WINDOW_SECONDS seconds (should NOT see a new reconcile yet)"
sleep $PRE_WINDOW_SECONDS

LR2=$(get_lr)
T2=$(to_epoch "$LR2")

if [[ "$T2" -ne "$T1" ]]; then
  echo "FAIL: LastReconcileTime changed too early (T1=$T1, T2=$T2)"
  kubectl -n "$NAMESPACE" describe disposablerequest "$NAME" || true
  exit 2
fi
echo "PASS: No premature reconcile observed"

WAIT_TOTAL=$((NEXT_RECONCILE_SECONDS + SLEEP_BUFFER))
echo "Waiting until after nextReconcile: sleeping $WAIT_TOTAL seconds"
sleep $WAIT_TOTAL

LR3=$(get_lr)
T3=$(to_epoch "$LR3")

if [[ "$T3" -le "$T1" ]]; then
  echo "FAIL: LastReconcileTime did not update after nextReconcile (T1=$T1, T3=$T3)"
  kubectl -n "$NAMESPACE" describe disposablerequest "$NAME" || true
  exit 3
fi

echo "PASS: LastReconcileTime updated after nextReconcile (T1=$T1, T3=$T3)"
echo "verify-next-reconcile: success"
