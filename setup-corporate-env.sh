#!/bin/bash
# Corporate environment setup script
# Run: source setup-corporate-env.sh

export GOPROXY="https://artifacts.rbi.tech/artifactory/proxy-golang-org-go-proxy/,direct"
export GOSUMDB="off"  # Disable checksum database when using corporate proxy
export GONOPROXY=""
export GONOSUMDB=""

echo "Corporate Go proxy environment configured:"
echo "GOPROXY: $GOPROXY"
echo "GOSUMDB: $GOSUMDB"

# Test if certificate exists
if [ -f "/Users/$(whoami)/certs/zscaler.pem" ]; then
    echo "Certificate: found at /Users/$(whoami)/certs/zscaler.pem"
else
    echo "Certificate: not found (will skip certificate installation)"
fi