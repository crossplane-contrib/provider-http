#!/usr/bin/env bash
set -aeuo pipefail

# Default to crossplane-contrib, but allow override via environment variable
TEST_SERVER_IMAGE=${TEST_SERVER_IMAGE:-"ghcr.io/crossplane-contrib/provider-http-server:latest"}

echo "Running setup.sh"
echo "Using test server image: ${TEST_SERVER_IMAGE}"

# Load test server image into kind cluster if using local image
# Check if we're in a kind cluster
if kubectl config current-context | grep -q "kind-"; then
  CLUSTER_NAME=$(kubectl config current-context | sed 's/kind-//')
  echo "Detected kind cluster: ${CLUSTER_NAME}"
  
  # Check if image exists locally
  if docker image inspect "${TEST_SERVER_IMAGE}" >/dev/null 2>&1; then
    echo "Loading test server image into kind cluster..."
    # Find kind binary (check common locations)
    KIND_BIN=""
    if command -v kind >/dev/null 2>&1; then
      KIND_BIN="kind"
    elif [ -f ".cache/tools/linux_x86_64/kind-v0.23.0" ]; then
      KIND_BIN=".cache/tools/linux_x86_64/kind-v0.23.0"
    elif [ -f "/workspaces/provider-http/.cache/tools/linux_x86_64/kind-v0.23.0" ]; then
      KIND_BIN="/workspaces/provider-http/.cache/tools/linux_x86_64/kind-v0.23.0"
    fi
    
    if [ -n "${KIND_BIN}" ]; then
      ${KIND_BIN} load docker-image "${TEST_SERVER_IMAGE}" --name "${CLUSTER_NAME}" || echo "Warning: Failed to load image, will try to pull from registry"
    else
      echo "Warning: kind binary not found, skipping image load"
    fi
  fi
fi

echo "Creating the provider config with cluster admin permissions in cluster..."
SA=$(${KUBECTL} -n crossplane-system get sa -o name | grep provider-http | sed -e 's|serviceaccount\/|crossplane-system:|g')
${KUBECTL} create clusterrolebinding provider-http-admin-binding --clusterrole cluster-admin --serviceaccount="${SA}" --dry-run=client -o yaml | ${KUBECTL} apply -f -

cat <<EOF | ${KUBECTL} apply -f -
apiVersion: http.crossplane.io/v1alpha1
kind: ProviderConfig
metadata:
  name: http-conf
spec:
  credentials:
    source: None
EOF

cat <<EOF | ${KUBECTL} apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-server
  namespace: default
  labels:
    app: test-server
spec:
  replicas: 3
  selector:
    matchLabels:
      app: test-server
  template:
    metadata:
      labels:
        app: test-server
    spec:
      containers:
      - name: server
        image: ${TEST_SERVER_IMAGE}
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 5000
---
apiVersion: v1
kind: Service
metadata:
  name: test-server
  namespace: default
spec:
  selector:
    app: test-server
  ports:
  - protocol: TCP
    port: 80
    targetPort: 5000
  type: ClusterIP
EOF

cat <<EOF | kubectl apply -f -
kind: Secret
apiVersion: v1
metadata:
  name: auth
  namespace: default
type: Opaque
data:
  token: bXktc2VjcmV0LXZhbHVl
EOF

cat <<EOF | kubectl apply -f -
kind: Secret
apiVersion: v1
metadata:
  name: user-password
  namespace: crossplane-system
type: Opaque
data:
  password: bXktc2VjcmV0LXZhbHVl
EOF

cat <<EOF | kubectl apply -f -
kind: Secret
apiVersion: v1
metadata:
  name: basic-auth
  namespace: crossplane-system
type: Opaque
data:
  token: bXktc2VjcmV0LXZhbHVl
EOF
