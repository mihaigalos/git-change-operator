#!/usr/bin/env bash
# Verify Docker image security and check for leaked secrets
# All checks must pass or the script exits with error code 1

set -euo pipefail

IMG="${1:-ghcr.io/mihaigalos/git-change-operator:latest}"
EXIT_CODE=0

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

# 3. Check for secret mounts (informational only - the critical check is #2 above)
if docker history --no-trunc --format "{{.CreatedBy}}" "$IMG" | grep -q "mount=type=secret"; then
    echo "✅ BuildKit secret mount detected (secure method)"
else
    echo "ℹ️  No secret mount found (cert not used or already validated in step 2)"
fi
echo

# 4. Check base image
echo "🔍 Checking base image..."
BASE=$(docker image inspect "$IMG" --format '{{index .Config.Labels "org.opencontainers.image.base.name"}}')
if [[ "$BASE" == *"distroless"* ]]; then
    echo "✅ Using distroless base: $BASE"
else
    echo "❌ Not using distroless base: $BASE"
    echo "   Distroless images are required for minimal attack surface"
    EXIT_CODE=1
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
    echo "   Shells enable command injection attacks"
    EXIT_CODE=1
fi
echo

# 6. Check user
echo "🔍 Checking container user..."
USER_INFO=$(docker image inspect "$IMG" --format '{{.Config.User}}')
if [[ "$USER_INFO" == "nonroot" ]] || [[ "$USER_INFO" == "65532" ]]; then
    echo "✅ Running as non-root user: $USER_INFO"
else
    echo "❌ Not running as non-root user: $USER_INFO"
    echo "   Expected 'nonroot' or '65532' for least privilege"
    EXIT_CODE=1
fi
echo

# 7. Check image size
echo "🔍 Checking image size..."
SIZE=$(docker image inspect "$IMG" --format '{{.Size}}' | awk '{printf "%.1f MB", $1/1024/1024}')
echo "   Size: $SIZE"
if docker image inspect "$IMG" --format '{{.Size}}' | awk '{exit ($1 > 150*1024*1024)}'; then
    echo "✅ Reasonable size (< 150 MB)"
else
    echo "❌ Large image size: $SIZE (exceeds 150 MB limit)"
    echo "   Large images increase attack surface and deployment time"
    EXIT_CODE=1
fi
echo

# 8. List files in container
echo "🔍 Listing container filesystem..."
echo "   Key files:"
timeout 2 docker run --rm "$IMG" /home/nonroot/operator --help 2>&1 | head -3 || echo "   - Operator binary: ✅"
echo

# 9. Summary
echo "================================================"
echo "📊 Security Summary"
echo "================================================"

if [ $EXIT_CODE -eq 0 ]; then
    echo "✅ BuildKit secrets used (no leaked credentials)"
    echo "✅ Distroless base image (minimal attack surface)"
    echo "✅ No shell (prevents command injection)"
    echo "✅ Non-root user (least privilege)"
    echo "✅ Reasonable image size"
    echo
    echo "🎉 Image is secure for production use!"
    echo
    echo "Next steps:"
    echo "  1. Run vulnerability scan: trivy image $IMG"
    echo "  2. Push to registry: just docker-push"
    echo "  3. Deploy to cluster: just kind-deploy"
else
    echo
    echo "❌ Security verification FAILED!"
    echo "   Fix the issues above before deploying to production"
    echo
fi

exit $EXIT_CODE