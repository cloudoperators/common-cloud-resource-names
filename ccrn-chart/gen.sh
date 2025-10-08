#!/bin/bash
# Generate new self-signed certificates for the webhook

# Set variables
CERT_DIR="./certs"
SERVICE_NAME="ccrn"
NAMESPACE="ccrn-system"
DNS_NAMES="${SERVICE_NAME},${SERVICE_NAME}.${NAMESPACE},${SERVICE_NAME}.${NAMESPACE}.svc"

# Create directory for certificates
mkdir -p ${CERT_DIR}

# Generate CA certificate
openssl genrsa -out ${CERT_DIR}/ca.key 2048
openssl req -x509 -new -nodes -key ${CERT_DIR}/ca.key -days 3650 -out ${CERT_DIR}/ca.crt -subj "/CN=CCRN Webhook CA"

# Generate server certificate
openssl genrsa -out ${CERT_DIR}/server.key 2048

# Create config for certificate
cat > ${CERT_DIR}/csr.conf <<EOF
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names
[alt_names]
DNS.1 = ${SERVICE_NAME}
DNS.2 = ${SERVICE_NAME}.${NAMESPACE}
DNS.3 = ${SERVICE_NAME}.${NAMESPACE}.svc
EOF

# Generate server certificate
openssl req -new -key ${CERT_DIR}/server.key -out ${CERT_DIR}/server.csr -subj "/CN=${SERVICE_NAME}.${NAMESPACE}.svc" -config ${CERT_DIR}/csr.conf
openssl x509 -req -in ${CERT_DIR}/server.csr -CA ${CERT_DIR}/ca.crt -CAkey ${CERT_DIR}/ca.key -CAcreateserial -out ${CERT_DIR}/server.crt -days 3650 -extensions v3_req -extfile ${CERT_DIR}/csr.conf

# Base64 encode certificates for Kubernetes
BASE64_CA=$(cat ${CERT_DIR}/ca.crt | base64 | tr -d '\n')
BASE64_SERVER_CERT=$(cat ${CERT_DIR}/server.crt | base64 | tr -d '\n')
BASE64_SERVER_KEY=$(cat ${CERT_DIR}/server.key | base64 | tr -d '\n')

# Output the encoded certificates
echo "CA bundle for webhook configuration:"
echo "${BASE64_CA}"
echo
echo "Server certificate for static-certs.yaml:"
echo "${BASE64_SERVER_CERT}"
echo
echo "Server key for static-certs.yaml:"
echo "${BASE64_SERVER_KEY}"

# Output update instructions
echo
echo "Replace the CA bundle in webhook-configuration.yaml with the CA bundle value above."
echo "Replace the tls.crt and tls.key in static-certs.yaml with the server certificate and key values above."
echo "Replace the ca.crt in static-certs.yaml with the CA bundle value above."