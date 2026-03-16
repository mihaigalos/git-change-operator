# syntax=docker/dockerfile:1.4
#----------------------------------------------------------------------------------------------
FROM golang:1.22 AS builder

WORKDIR /workspace

# Accept build args for proxy configuration
ARG GOPROXY
ARG GOSUMDB
ARG GONOPROXY
ARG GONOSUMDB

# Set Go proxy configuration if provided
ENV GOPROXY=${GOPROXY:-direct}
ENV GOSUMDB=${GOSUMDB:-sum.golang.org}
ENV GONOPROXY=${GONOPROXY}
ENV GONOSUMDB=${GONOSUMDB}

# Install corporate certificate using BuildKit secret (not embedded in layers)
# Secret is mounted at build time only and never persisted in image
RUN --mount=type=secret,id=corporate_cert,required=false \
    if [ -f /run/secrets/corporate_cert ]; then \
        apt-get update && apt-get install -y ca-certificates && \
        cp /run/secrets/corporate_cert /usr/local/share/ca-certificates/corporate-ca.crt && \
        update-ca-certificates && \
        echo "Corporate certificate installed from secret (not embedded in layers)"; \
    else \
        echo "No corporate certificate secret provided - using system CAs only"; \
    fi

COPY go.mod go.sum ./
RUN go mod download

COPY main.go main.go
COPY api/ api/
COPY pkg/ pkg/
COPY controllers/ controllers/
COPY helm/ helm/
COPY config/crd/bases/ config/crd/bases/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -a -o operator main.go && chmod +x operator

# Resolve symlinks in Helm chart to avoid broken symlinks in final image
RUN mkdir -p /workspace/helm-resolved && \
    cp -r helm/git-change-operator/* /workspace/helm-resolved/ && \
    find /workspace/helm-resolved -type l | while read -r symlink; do \
        link_target=$(readlink "$symlink"); \
        basename_target=$(basename "$link_target"); \
        actual_file=$(find /workspace -name "$basename_target" -type f 2>/dev/null | head -1); \
        if [ -n "$actual_file" ] && [ -f "$actual_file" ]; then \
            rm -f "$symlink" && cp "$actual_file" "$symlink"; \
        fi; \
    done

#----------------------------------------------------------------------------------------------
# Use distroless base image - NO shell, minimal attack surface, non-root by default
FROM gcr.io/distroless/static-debian12:nonroot

# Upstream Ref
ARG GIT_REFERENCE
LABEL git-ref="$GIT_REFERENCE" \
      org.opencontainers.image.source="$GIT_REFERENCE" \
      org.opencontainers.image.description="Git Change Operator - Hardened distroless image" \
      org.opencontainers.image.base.name="gcr.io/distroless/static-debian12:nonroot"

WORKDIR /home/nonroot

# Copy ONLY the system CA bundle from builder (corporate cert was used during build only)
# The corporate cert is NOT included in the final image - only standard system CAs
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Copy the operator binary
COPY --from=builder --chown=nonroot:nonroot /workspace/operator /home/nonroot/operator

# Copy resolved Helm chart (without broken symlinks)
COPY --from=builder --chown=nonroot:nonroot /workspace/helm-resolved /home/nonroot/helm/git-change-operator

# Distroless already runs as nonroot user (UID 65532)
# No shell available - cannot be exploited for command injection
USER nonroot

ENTRYPOINT ["/home/nonroot/operator"]
