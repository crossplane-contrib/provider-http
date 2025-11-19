#!/usr/bin/env bash
set -aeuo pipefail

# Default to local image if not overridden
TEST_SERVER_IMAGE=${TEST_SERVER_IMAGE:-"provider-http-test-server:latest"}

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

    # Create user-password secret in default namespace for namespaced examples
    cat <<EOF | ${KUBECTL} apply -f -
kind: Secret
apiVersion: v1
metadata:
  name: user-password
  namespace: default
type: Opaque
data:
  password: bXktc2VjcmV0LXZhbHVl
EOF

    # Create auth secret in crossplane-system namespace for clusterproviderconfig examples
    cat <<EOF | ${KUBECTL} apply -f -
kind: Secret
apiVersion: v1
metadata:
  name: auth
  namespace: crossplane-system
type: Opaque
data:
  token: bXktc2VjcmV0LXZhbHVl
EOF

    # Create admin-user secret in crossplane-system namespace for clusterproviderconfig examples
    cat <<EOF | ${KUBECTL} apply -f -
kind: Secret
apiVersion: v1
metadata:
  name: admin-user
  namespace: crossplane-system
type: Opaque
data:
  username: YWRtaW4=
EOF

    # Create admin-token secret in crossplane-system namespace for clusterproviderconfig examples
    # Using same token value as auth:default to ensure compatibility with flask-api
    cat <<EOF | ${KUBECTL} apply -f -
kind: Secret
apiVersion: v1
metadata:
  name: admin-token
  namespace: crossplane-system
type: Opaque
data:
  token: bXktc2VjcmV0LXZhbHVl
EOF

    echo "Waiting for secrets to be available..."
    # Wait for all secrets to be available before proceeding
    for secret in "user-password:default" "auth:crossplane-system" "admin-user:crossplane-system" "admin-token:crossplane-system"; do
        name=$(echo $secret | cut -d: -f1)
        namespace=$(echo $secret | cut -d: -f2)
        echo "Waiting for secret $name in namespace $namespace..."
        while ! ${KUBECTL} get secret $name -n $namespace >/dev/null 2>&1; do
            sleep 1
        done
        echo "Secret $name in namespace $namespace is available"
    done
    
    # Additional wait to ensure provider has processed the secrets
    echo "Waiting 10 seconds for provider to process secrets..."
    sleep 10
    
    echo "Namespaced provider configurations created successfully"
else
    echo "Detected Crossplane v1, skipping namespaced configurations"
fi

cat <<EOF | ${KUBECTL} apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-server
  namespace: default
  labels:
    app: test-server
spec:
  replicas: 1
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
