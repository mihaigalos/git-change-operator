#----------------------------------------------------------------------------------------------
FROM golang:1.21 AS builder

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
COPY controllers/ controllers/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -a -o manager main.go && chmod +x manager

#----------------------------------------------------------------------------------------------
FROM alpine:3.18

# Accept build args for package repositories
ARG APK_MAIN_REPO
ARG APK_COMMUNITY_REPO

# Accept build arg for corporate certificate content (passed from builder stage)
ARG CORPORATE_CA_CERT

# Configure Alpine repositories if custom ones are provided
RUN if [ -n "$APK_MAIN_REPO" ] && [ -n "$APK_COMMUNITY_REPO" ]; then \
        echo "$APK_MAIN_REPO" > /etc/apk/repositories && \
        echo "$APK_COMMUNITY_REPO" >> /etc/apk/repositories; \
    fi

# Install ca-certificates first
RUN apk --no-cache add ca-certificates curl

# Install corporate certificate from build arg AFTER ca-certificates is installed
RUN if [ -n "$CORPORATE_CA_CERT" ]; then \
        echo "$CORPORATE_CA_CERT" > /usr/local/share/ca-certificates/corporate-ca.crt && \
        update-ca-certificates && \
        echo "Corporate certificate installed from build arg"; \
    else \
        echo "No corporate certificate provided in build arg"; \
    fi
WORKDIR /
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /workspace/manager /manager
RUN chmod +x /manager
ENTRYPOINT ["/manager"]