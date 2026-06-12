#!/bin/bash
#
# Prepare certificate ConfigMap for kustomize from placement-custom-route secret
#

set -e

if [[ -z "${NAMESPACE}" ]]; then
    echo "Error: NAMESPACE environment variable must be set"
    exit 1
fi

# Wait for the secret to be created (retry up to 3 times with 10s wait)
echo "Waiting for placement-custom-route secret to be created..."
MAX_RETRIES=3
RETRY_COUNT=0
SECRET_EXISTS=false

while [[ ${RETRY_COUNT} -lt ${MAX_RETRIES} ]]; do
    if oc get secret placement-custom-route -n ${NAMESPACE} &>/dev/null; then
        echo "Secret placement-custom-route found in namespace ${NAMESPACE}"
        SECRET_EXISTS=true
        break
    fi

    RETRY_COUNT=$((RETRY_COUNT + 1))
    if [[ ${RETRY_COUNT} -lt ${MAX_RETRIES} ]]; then
        echo "Secret not found yet, waiting 10s... (attempt ${RETRY_COUNT}/${MAX_RETRIES})"
        sleep 10
    fi
done

if [[ "${SECRET_EXISTS}" != "true" ]]; then
    echo "Error: Secret placement-custom-route not found in namespace ${NAMESPACE} after ${MAX_RETRIES} attempts"
    exit 1
fi

echo "Fetching certificates from placement-custom-route secret and creating ConfigMap..."

# Get the directory where the script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Write to a temp file to avoid parallel workers clobbering a shared repo path.
# The kustomize overlay at config/samples/tls/custom_route_cert references this
# file, but since the parallel runner assigns each test directory to a single
# worker, only one invocation of this test can run at a time.
CONFIGMAP_FILE="/tmp/placement-cert-data-${NAMESPACE}.yaml"

echo "Creating ConfigMap file at: ${CONFIGMAP_FILE}"

cat > "${CONFIGMAP_FILE}" << EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: placement-cert-data
  namespace: ${NAMESPACE}
data:
  tls.crt: |
$(oc get secret placement-custom-route -n ${NAMESPACE} -o jsonpath='{.data.tls\.crt}' | base64 -d | sed 's/^/    /')
  tls.key: |
$(oc get secret placement-custom-route -n ${NAMESPACE} -o jsonpath='{.data.tls\.key}' | base64 -d | sed 's/^/    /')
  ca.crt: |
$(oc get secret placement-custom-route -n ${NAMESPACE} -o jsonpath='{.data.ca\.crt}' | base64 -d | sed 's/^/    /')
EOF

# Copy to the kustomize directory so oc kustomize can find it
KUSTOMIZE_DIR="${SCRIPT_DIR}/../../../config/samples/tls/custom_route_cert"
cp "${CONFIGMAP_FILE}" "${KUSTOMIZE_DIR}/placement-cert-data.yaml"

oc apply -f "${CONFIGMAP_FILE}"

echo "ConfigMap placement-cert-data created at ${CONFIGMAP_FILE} and applied to namespace ${NAMESPACE}"
