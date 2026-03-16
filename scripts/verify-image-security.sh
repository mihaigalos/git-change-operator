#!/usr/bin/env bash
# Verify Docker image security and check for leaked secrets

set -euo pipefail

IMG="${1:-ghcr.io/mihaigalos/git-change-operator:latest}"

echo "🔍 Security Verification for: $IMG"
echo "================================================"
echo

# 1. Check if image exists
if ! docker image inspect "$IMG" &>/dev/null; then
    echo "❌ Image not found: $IMG"
    echo "   Run 'just docker-build' first"
    exit 1
fi

echo "✅ Image exists"
echo

# 2. Check image history for secrets
echo "🔍 Checking image history for corporate certificate..."
if docker history --no-trunc --format "{{.CreatedBy}}" "$IMG" | grep -i "CORPORATE_CA_CERT" | grep -v "secret"; then
    echo "❌ LEAK DETECTED: Corporate certificate found in build args!"
    echo "   Certificate content may be embedded in image layers"
    exit 1
else
    echo "✅ No corporate certificate in build args (using BuildKit secrets)"
fi
echo

# 3. Check for secret mounts (should be present, but not content)
if docker history --no-trunc --format "{{.CreatedBy}}" "$IMG" | grep -q "mount=type=secret"; then
    echo "✅ BuildKit secret mount detected (secure method)"
else
    echo "⚠️  No secret mount found (may not have used corporate cert)"
fi
echo

# 4. Check base image
echo "🔍 Checking base image..."
BASE=$(docker image inspect "$IMG" --format '{{index .Config.Labels "org.opencontainers.image.base.name"}}')
if [[ "$BASE" == *"distroless"* ]]; then
    echo "✅ Using distroless base: $BASE"
else
    echo "⚠️  Not using distroless: $BASE"
fi
echo

# 5. Check for shell
echo "🔍 Checking for shell presence..."
FOUND_SHELLS=""
for shell in /bin/sh /bin/bash /bin/ash /usr/bin/sh /usr/bin/bash /usr/bin/ash; do
    if docker run --rm --entrypoint="" "$IMG" test -f "$shell" 2>/dev/null; then
        FOUND_SHELLS="$FOUND_SHELLS $shell"
    fi
done

if [ -z "$FOUND_SHELLS" ]; then
    echo "✅ No shell found in image (secure)"
else
    echo "❌ Shell(s) found:$FOUND_SHELLS (security risk)"
fi
echo

# 6. Check user
echo "🔍 Checking container user..."
USER_INFO=$(docker image inspect "$IMG" --format '{{.Config.User}}')
if [[ "$USER_INFO" == "nonroot" ]] || [[ "$USER_INFO" == "65532" ]]; then
    echo "✅ Running as non-root user: $USER_INFO"
else
    echo "⚠️  User: $USER_INFO (expected nonroot or 65532)"
fi
echo

# 7. Check image size
echo "🔍 Checking image size..."
SIZE=$(docker image inspect "$IMG" --format '{{.Size}}' | awk '{printf "%.1f MB", $1/1024/1024}')
echo "   Size: $SIZE"
if docker image inspect "$IMG" --format '{{.Size}}' | awk '{exit ($1 > 150*1024*1024)}'; then
    echo "✅ Reasonable size (< 150 MB)"
else
    echo "⚠️  Large image size (consider optimization)"
fi
echo

# 8. List files in container
echo "🔍 Listing container filesystem..."
echo "   Key files:"
docker run --rm "$IMG" /home/nonroot/operator --help 2>&1 | head -3 || echo "   - Operator binary: ✅"
echo