# Custom Registry Deployment Guide

This guide explains how to build and deploy provider-http to your private container registry (e.g., Azure Container Registry, AWS ECR, Google GCR, or private Docker registry).

## Prerequisites

- Docker installed and running
- Access to your container registry
- Kubernetes cluster with Crossplane installed
- kubectl configured for your cluster

## Azure Container Registry (ACR)

### 1. Login to ACR

```bash
# Using Azure CLI
az acr login --name yourregistry

# Or using docker login
docker login yourregistry.azurecr.io \
  --username <username> \
  --password <password>
```

### 2. Build Image

**For ARM64 (Mac M1/M2/M3):**

```bash
docker buildx build \
  --platform linux/arm64 \
  --build-arg ARCH=arm64 \
  -t yourregistry.azurecr.io/provider-http:v1.0.0-custom \
  -f cluster/Dockerfile \
  --load .
```

**For AMD64 (Linux/Windows):**

```bash
docker buildx build \
  --platform linux/amd64 \
  --build-arg ARCH=amd64 \
  -t yourregistry.azurecr.io/provider-http:v1.0.0-custom \
  -f cluster/Dockerfile \
  --load .
```

**For Multi-Platform:**

```bash
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t yourregistry.azurecr.io/provider-http:v1.0.0-custom \
  -f cluster/Dockerfile \
  --push .
```

### 3. Push Image

```bash
docker push yourregistry.azurecr.io/provider-http:v1.0.0-custom
```

### 4. Create Pull Secret

```bash
# Get ACR credentials
ACR_USERNAME=$(az acr credential show --name yourregistry --query username -o tsv)
ACR_PASSWORD=$(az acr credential show --name yourregistry --query passwords[0].value -o tsv)

# Create Kubernetes secret
kubectl create secret docker-registry acr-credentials \
  --docker-server=yourregistry.azurecr.io \
  --docker-username=$ACR_USERNAME \
  --docker-password=$ACR_PASSWORD \
  --namespace=crossplane-system
```

### 5. Install Provider

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-http-custom
spec:
  package: yourregistry.azurecr.io/provider-http:v1.0.0-custom
  packagePullSecrets:
    - name: acr-credentials
```

Apply:
```bash
kubectl apply -f provider.yaml
```

### 6. Verify Installation

```bash
# Check provider status
kubectl get providers

# Check provider pods
kubectl get pods -n crossplane-system

# Check provider logs
kubectl logs -n crossplane-system -l pkg.crossplane.io/provider=provider-http-custom
```

## AWS Elastic Container Registry (ECR)

### 1. Login to ECR

```bash
# Get login password
aws ecr get-login-password --region us-east-1 | \
  docker login --username AWS \
  --password-stdin 123456789012.dkr.ecr.us-east-1.amazonaws.com
```

### 2. Create Repository

```bash
aws ecr create-repository \
  --repository-name provider-http \
  --region us-east-1
```

### 3. Build and Push

```bash
# Build
docker build \
  -t 123456789012.dkr.ecr.us-east-1.amazonaws.com/provider-http:v1.0.0-custom \
  -f cluster/Dockerfile .

# Push
docker push 123456789012.dkr.ecr.us-east-1.amazonaws.com/provider-http:v1.0.0-custom
```

### 4. Create Pull Secret

```bash
# Create ECR credentials secret
kubectl create secret docker-registry ecr-credentials \
  --docker-server=123456789012.dkr.ecr.us-east-1.amazonaws.com \
  --docker-username=AWS \
  --docker-password=$(aws ecr get-login-password --region us-east-1) \
  --namespace=crossplane-system
```

### 5. Install Provider

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-http-custom
spec:
  package: 123456789012.dkr.ecr.us-east-1.amazonaws.com/provider-http:v1.0.0-custom
  packagePullSecrets:
    - name: ecr-credentials
```

## Google Container Registry (GCR)

### 1. Setup Authentication

```bash
# Authenticate with GCP
gcloud auth login
gcloud auth configure-docker
```

### 2. Build and Push

```bash
# Build
docker build \
  -t gcr.io/your-project-id/provider-http:v1.0.0-custom \
  -f cluster/Dockerfile .

# Push
docker push gcr.io/your-project-id/provider-http:v1.0.0-custom
```

### 3. Create Pull Secret

```bash
# Create service account key
gcloud iam service-accounts keys create key.json \
  --iam-account=your-sa@your-project-id.iam.gserviceaccount.com

# Create secret from JSON key
kubectl create secret docker-registry gcr-credentials \
  --docker-server=gcr.io \
  --docker-username=_json_key \
  --docker-password="$(cat key.json)" \
  --namespace=crossplane-system
```

### 4. Install Provider

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-http-custom
spec:
  package: gcr.io/your-project-id/provider-http:v1.0.0-custom
  packagePullSecrets:
    - name: gcr-credentials
```

## Private Docker Registry

### 1. Build and Push

```bash
# Build
docker build \
  -t registry.example.com/provider-http:v1.0.0-custom \
  -f cluster/Dockerfile .

# Login
docker login registry.example.com \
  --username <username> \
  --password <password>

# Push
docker push registry.example.com/provider-http:v1.0.0-custom
```

### 2. Create Pull Secret

```bash
kubectl create secret docker-registry registry-credentials \
  --docker-server=registry.example.com \
  --docker-username=<username> \
  --docker-password=<password> \
  --namespace=crossplane-system
```

### 3. Install Provider

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-http-custom
spec:
  package: registry.example.com/provider-http:v1.0.0-custom
  packagePullSecrets:
    - name: registry-credentials
```

## Troubleshooting

### Image Pull Errors

```bash
# Check provider status
kubectl describe provider provider-http-custom

# Check package revision
kubectl get providerrevision

# Check package revision events
kubectl describe providerrevision <revision-name>
```

Common errors:
- **"ImagePullBackOff"**: Check pull secret is created and credentials are correct
- **"Failed to pull image"**: Verify image exists in registry
- **"Unauthorized"**: Check credentials and permissions

### Verify Image Exists

```bash
# ACR
az acr repository show-tags --name yourregistry --repository provider-http

# ECR
aws ecr describe-images --repository-name provider-http --region us-east-1

# GCR
gcloud container images list-tags gcr.io/your-project-id/provider-http

# Docker
curl -X GET https://registry.example.com/v2/provider-http/tags/list
```

### Check Pull Secret

```bash
# View secret
kubectl get secret acr-credentials -n crossplane-system -o yaml

# Verify secret data
kubectl get secret acr-credentials -n crossplane-system -o jsonpath='{.data.\.dockerconfigjson}' | base64 -d | jq
```

### Provider Logs

```bash
# Get provider pod name
POD=$(kubectl get pods -n crossplane-system -l pkg.crossplane.io/provider=provider-http-custom -o name)

# View logs
kubectl logs -n crossplane-system $POD

# Follow logs
kubectl logs -n crossplane-system $POD -f
```

## Best Practices

1. **Tagging:**
   - Use semantic versioning: `v1.0.0`, `v1.0.1`, etc.
   - Add git commit SHA: `v1.0.0-abc1234`
   - Use descriptive suffixes: `v1.0.0-tls`, `v1.0.0-custom`

2. **Multi-Platform:**
   - Build for both `linux/amd64` and `linux/arm64` if deploying to mixed clusters
   - Use `docker buildx` for multi-platform builds

3. **Security:**
   - Use service accounts with minimal permissions
   - Rotate registry credentials regularly
   - Store credentials in secret management tools (Vault, Sealed Secrets, etc.)

4. **CI/CD Integration:**
   - Automate builds in CI pipeline
   - Push to registry on tag or release
   - Run tests before building images

## Example Complete Workflow

```bash
# 1. Set variables
REGISTRY="yourregistry.azurecr.io"
IMAGE="provider-http"
VERSION="v1.0.0-custom"

# 2. Login
az acr login --name yourregistry

# 3. Build
docker buildx build \
  --platform linux/arm64 \
  --build-arg ARCH=arm64 \
  -t $REGISTRY/$IMAGE:$VERSION \
  -f cluster/Dockerfile \
  --load .

# 4. Push
docker push $REGISTRY/$IMAGE:$VERSION

# 5. Create pull secret
kubectl create secret docker-registry acr-credentials \
  --docker-server=$REGISTRY \
  --docker-username=$(az acr credential show --name yourregistry --query username -o tsv) \
  --docker-password=$(az acr credential show --name yourregistry --query passwords[0].value -o tsv) \
  --namespace=crossplane-system

# 6. Create provider manifest
cat <<EOF | kubectl apply -f -
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-http-custom
spec:
  package: $REGISTRY/$IMAGE:$VERSION
  packagePullSecrets:
    - name: acr-credentials
EOF

# 7. Wait for provider to be ready
kubectl wait --for=condition=healthy provider/provider-http-custom --timeout=5m

# 8. Verify
kubectl get providers
kubectl get pods -n crossplane-system
```
