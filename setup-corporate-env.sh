#!/bin/bash
# Corporate environment setup script
# Run: source setup-corporate-env.sh
# Or: source corporate-config.env

export GOPROXY="https://proxy.golang.org,direct"
export GOSUMDB="sum.golang.org"  # Default Go checksum database
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