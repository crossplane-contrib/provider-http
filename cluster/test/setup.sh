#!/usr/bin/env bash
set -aeuo pipefail

echo "Running setup.sh"
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

# Check if we're running Crossplane v2 and create namespaced provider configurations
if [ -z "${CROSSPLANE_VERSION:-}" ]; then
    echo "ERROR: CROSSPLANE_VERSION environment variable must be set"
    exit 1
fi
MAJOR_VERSION=$(echo "$CROSSPLANE_VERSION" | cut -d. -f1)

if [ "$MAJOR_VERSION" = "2" ]; then
    echo "Detected Crossplane v2, creating namespaced provider configurations..."
    
    # Create namespaced ProviderConfig
    cat <<EOF | ${KUBECTL} apply -f -
apiVersion: http.m.crossplane.io/v1alpha2
kind: ProviderConfig
metadata:
  name: http-conf-namespaced
  namespace: default
spec:
  credentials:
    source: None
EOF

    # Create ClusterProviderConfig for cross-namespace access
    cat <<EOF | ${KUBECTL} apply -f -
apiVersion: http.m.crossplane.io/v1alpha2
kind: ClusterProviderConfig
metadata:
  name: http-conf-cluster
spec:
  credentials:
    source: None
EOF
    
    # Create additional secrets needed for namespaced examples
    cat <<EOF | ${KUBECTL} apply -f -
kind: Secret
apiVersion: v1
metadata:
  name: basic-auth
  namespace: default
type: Opaque
data:
  token: bXktc2VjcmV0LXZhbHVl
EOF
    
    echo "Namespaced provider configurations created successfully"
else
    echo "Detected Crossplane v1, skipping namespaced configurations"
fi

cat <<EOF | ${KUBECTL} apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: flask-api
  namespace: default
  labels:
    app: flask-api
spec:
  replicas: 3
  selector:
    matchLabels:
      app: flask-api
  template:
    metadata:
      labels:
        app: flask-api
    spec:
      containers:
      - name: flask-api
        image: arielsepton/flask-api:v1.0.0
        ports:
        - containerPort: 5000
---
apiVersion: v1
kind: Service
metadata:
  name: flask-api
  namespace: default
spec:
  selector:
    app: flask-api
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
