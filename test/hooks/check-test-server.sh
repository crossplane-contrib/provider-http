#!/usr/bin/env bash
set -euo pipefail

# check-test-server.sh
# Uptest pre-assert hook: wait until the test-server endpoint responds to POST /v1/notify

NAMESPACE=${TEST_NAMESPACE:-default}
SERVICE_HOST=${TEST_SERVICE_HOST:-test-server.${NAMESPACE}.svc.cluster.local}
URL="http://${SERVICE_HOST}/v1/notify"
AUTH_TOKEN=${TEST_AUTH_TOKEN:-my-secret-value}
TIMEOUT=${TEST_SERVER_TIMEOUT:-120}

echo "check-test-server: waiting for $URL to respond (timeout ${TIMEOUT}s)"
end=$((SECONDS + TIMEOUT))
while [ $SECONDS -lt $end ]; do
  status=$(kubectl -n "$NAMESPACE" run --rm -i --restart=Never curl-test \
    --image=curlimages/curl --command -- /bin/sh -c \
    "curl -s -o /dev/null -w '%{http_code}' -H \"Authorization: Bearer ${AUTH_TOKEN}\" -X POST \"${URL}\"" 2>/dev/null || true)

  status=${status//[^0-9]/}
  if [ "$status" = "201" ]; then
    echo "check-test-server: endpoint is ready (HTTP $status)"
    exit 0
  fi
  echo "check-test-server: not ready yet (status=$status), retrying..."
  sleep 3
done

echo "check-test-server: timeout waiting for $URL"
exit 1
