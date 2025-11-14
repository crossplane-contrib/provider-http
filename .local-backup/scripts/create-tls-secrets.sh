#!/bin/bash
set -e

# Script to create Kubernetes secrets from PEM certificates in cert/ directory
# Usage: ./scripts/create-tls-secrets.sh [namespace]

NAMESPACE="${1:-default}"
CERT_DIR="./cert"

echo "Creating TLS secrets from certificates in ${CERT_DIR}/"
echo "Target namespace: ${NAMESPACE}"
echo ""

# Check if cert directory exists
if [ ! -d "$CERT_DIR" ]; then
    echo "Error: Certificate directory ${CERT_DIR} not found"
    exit 1
fi

# Function to create secret from a single PEM file
create_secret_from_pem() {
    local file=$1
    local filename=$(basename "$file")
    local secret_name=$(echo "$filename" | sed 's/\.pem$//' | tr '[:upper:]' '[:lower:]' | tr '_' '-' | tr '.' '-')
    
    echo "Creating secret: ${secret_name} from ${filename}"
    kubectl create secret generic "${secret_name}" \
        --from-file=tls.pem="$file" \
        --namespace="${NAMESPACE}" \
        --dry-run=client -o yaml | kubectl apply -f -
    
    echo "✓ Secret ${secret_name} created/updated"
    echo ""
}

# Function to create secret from certificate and key pair
create_secret_from_cert_key() {
    local cert_file=$1
    local key_file=$2
    local base_name=$(basename "$cert_file" | sed 's/\.pem$/\|\.crt$/\|\.cert$//' | tr '[:upper:]' '[:lower:]' | tr '_' '-')
    local secret_name="${base_name}-tls"
    
    echo "Creating TLS secret: ${secret_name}"
    echo "  Certificate: $(basename $cert_file)"
    echo "  Key: $(basename $key_file)"
    
    kubectl create secret generic "${secret_name}" \
        --from-file=tls.crt="$cert_file" \
        --from-file=tls.key="$key_file" \
        --namespace="${NAMESPACE}" \
        --dry-run=client -o yaml | kubectl apply -f -
    
    echo "✓ Secret ${secret_name} created/updated"
    echo ""
}

# Create CA bundle secret if ca-bundle.pem or ca.pem exists
if [ -f "${CERT_DIR}/ca-bundle.pem" ]; then
    create_secret_from_pem "${CERT_DIR}/ca-bundle.pem"
elif [ -f "${CERT_DIR}/ca.pem" ]; then
    create_secret_from_pem "${CERT_DIR}/ca.pem"
fi

# Process all PEM files in cert directory
for pem_file in "${CERT_DIR}"/*.pem; do
    if [ -f "$pem_file" ]; then
        filename=$(basename "$pem_file")
        
        # Skip if already processed as CA bundle
        if [ "$filename" = "ca-bundle.pem" ] || [ "$filename" = "ca.pem" ]; then
            continue
        fi
        
        # Check if this is a certificate with a matching key file
        base_name="${filename%.pem}"
        key_file="${CERT_DIR}/${base_name}.key"
        
        if [ -f "$key_file" ]; then
            # Create secret with both cert and key
            create_secret_from_cert_key "$pem_file" "$key_file"
        else
            # Create secret with just the PEM file (could be CA cert or standalone cert)
            create_secret_from_pem "$pem_file"
        fi
    fi
done

# Also process .crt and .key pairs
for cert_file in "${CERT_DIR}"/*.crt; do
    if [ -f "$cert_file" ]; then
        base_name=$(basename "$cert_file" .crt)
        key_file="${CERT_DIR}/${base_name}.key"
        
        if [ -f "$key_file" ]; then
            create_secret_from_cert_key "$cert_file" "$key_file"
        else
            create_secret_from_pem "$cert_file"
        fi
    fi
done

echo "==================================="
echo "All secrets created in namespace: ${NAMESPACE}"
echo ""
echo "List secrets:"
echo "  kubectl get secrets -n ${NAMESPACE}"
echo ""
echo "View secret details:"
echo "  kubectl describe secret <secret-name> -n ${NAMESPACE}"
echo ""
echo "Next steps:"
echo "1. Create imagePullSecret for ACR:"
echo "   kubectl create secret docker-registry acr-secret \\"
echo "     --docker-server=iaactmpreg.azurecr.io \\"
echo "     --docker-username=<username> \\"
echo "     --docker-password=<password> \\"
echo "     --namespace=${NAMESPACE}"
echo ""
echo "2. Deploy the provider:"
echo "   kubectl apply -f deploy/provider.yaml"
echo ""
echo "3. Create ProviderConfig referencing these secrets:"
echo "   kubectl apply -f deploy/providerconfig.yaml"
