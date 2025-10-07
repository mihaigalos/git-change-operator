#----------------------------------------------------------------------------------------------
FROM golang:1.22 AS builder

WORKDIR /workspace

# Accept build args for proxy configuration
ARG GOPROXY
ARG GOSUMDB
ARG GONOPROXY
ARG GONOSUMDB

# Accept build arg for corporate certificate content
ARG CORPORATE_CA_CERT

# Set Go proxy configuration if provided
ENV GOPROXY=${GOPROXY:-direct}
ENV GOSUMDB=${GOSUMDB:-sum.golang.org}
ENV GONOPROXY=${GONOPROXY}
ENV GONOSUMDB=${GONOSUMDB}

# Install corporate certificate if provided
RUN if [ -n "$CORPORATE_CA_CERT" ]; then \
        apt-get update && apt-get install -y ca-certificates && \
        echo "$CORPORATE_CA_CERT" > /usr/local/share/ca-certificates/corporate-ca.crt && \
        update-ca-certificates && \
        echo "Corporate certificate installed from build arg"; \
    else \
        echo "No corporate certificate provided in build arg"; \
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
FROM alpine:3.18

# Accept build args for package repositories
ARG APK_MAIN_REPO
ARG APK_COMMUNITY_REPO

# Accept build arg for corporate certificate content (passed from builder stage)
ARG CORPORATE_CA_CERT

# Upstream Ref
ARG GIT_REFERENCE
LABEL git-ref="$GIT_REFERENCE" \
      org.opencontainers.image.source="$GIT_REFERENCE"

# Configure Alpine repositories if custom ones are provided
RUN if [ -n "$APK_MAIN_REPO" ] && [ -n "$APK_COMMUNITY_REPO" ]; then \
        echo "$APK_MAIN_REPO" > /etc/apk/repositories && \
        echo "$APK_COMMUNITY_REPO" >> /etc/apk/repositories; \
    fi

# Install ca-certificates first
RUN apk --no-cache add ca-certificates \
    && echo "curl bash helm"

# Install corporate certificate from build arg AFTER ca-certificates is installed
RUN if [ -n "$CORPORATE_CA_CERT" ]; then \
        echo "$CORPORATE_CA_CERT" > /usr/local/share/ca-certificates/corporate-ca.crt && \
        update-ca-certificates && \
        echo "Corporate certificate installed from build arg"; \
    else \
        echo "No corporate certificate provided in build arg"; \
    fi
RUN addgroup -S user && adduser -S user -G user
WORKDIR /home/user

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /workspace/operator /home/user/operator
RUN chmod +x /home/user/operator

# Copy resolved Helm chart (without broken symlinks)
COPY --from=builder /workspace/helm-resolved /home/user/helm/git-change-operator

USER user
RUN echo ${GIT_REFERENCE} > git_reference

ENTRYPOINT ["/home/user/operator"]
