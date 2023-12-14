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
    source: InjectedIdentity
EOF

cat <<EOF | ${KUBECTL} apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: todo
  labels:
    app: todo
spec:
  replicas: 1 # Number of replicas you want
  selector:
    matchLabels:
      app: todo
  template:
    metadata:
      labels:
        app: todo
    spec:
      containers:
      - name: todo
        image: danielsinai/todo:v1.0.0
        env:
        - name: PORT
          value: "80"
        ports:
        - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: todo
spec:
  type: ClusterIP
  ports:
  - name: "todo"
    port: 80
  selector:
    app: todo
EOF


