#!/usr/bin/env bash
set -aeuo pipefail

# Default to crossplane-contrib, but allow override via environment variable
TEST_SERVER_IMAGE=${TEST_SERVER_IMAGE:-"ghcr.io/crossplane-contrib/provider-http-server:latest"}

echo "Running setup.sh"
echo "Using test server image: ${TEST_SERVER_IMAGE}"

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
