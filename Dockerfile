#----------------------------------------------------------------------------------------------
FROM golang:1.21 AS builder

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

# Copy corporate certificate and install it if provided
COPY zscaler.pe[m] /tmp/zscaler.pem
RUN if [ -f /tmp/zscaler.pem ]; then \
        apt-get update && apt-get install -y ca-certificates && \
        cp /tmp/zscaler.pem /usr/local/share/ca-certificates/zscaler.crt && \
        update-ca-certificates && \
        rm /tmp/zscaler.pem; \
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

# Configure Alpine repositories if custom ones are provided
RUN if [ -n "$APK_MAIN_REPO" ] && [ -n "$APK_COMMUNITY_REPO" ]; then \
        echo "$APK_MAIN_REPO" > /etc/apk/repositories && \
        echo "$APK_COMMUNITY_REPO" >> /etc/apk/repositories; \
    fi

RUN apk --no-cache add ca-certificates curl
WORKDIR /
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY zscaler.pe[m] /usr/local/share/ca-certificates/zscaler.crt
RUN update-ca-certificates 2>/dev/null || true
COPY --from=builder /workspace/manager /manager
RUN chmod +x /manager
ENTRYPOINT ["/manager"]