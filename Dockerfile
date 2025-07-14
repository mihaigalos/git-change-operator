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
COPY corporate-ca.pe[m] /tmp/corporate-ca.pem
RUN if [ -f /tmp/corporate-ca.pem ] && [ -s /tmp/corporate-ca.pem ]; then \
        apt-get update && apt-get install -y ca-certificates && \
        cp /tmp/corporate-ca.pem /usr/local/share/ca-certificates/corporate-ca.crt && \
        update-ca-certificates && \
        rm /tmp/corporate-ca.pem; \
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

# Copy and install corporate certificate BEFORE configuring repositories or running apk
COPY corporate-ca.pe[m] /usr/local/share/ca-certificates/corporate-ca.crt
RUN if [ -f /usr/local/share/ca-certificates/corporate-ca.crt ] && [ -s /usr/local/share/ca-certificates/corporate-ca.crt ]; then \
        update-ca-certificates; \
    else \
        echo "No corporate certificate provided or empty file"; \
    fi

# Configure Alpine repositories if custom ones are provided
RUN if [ -n "$APK_MAIN_REPO" ] && [ -n "$APK_COMMUNITY_REPO" ]; then \
        echo "$APK_MAIN_REPO" > /etc/apk/repositories && \
        echo "$APK_COMMUNITY_REPO" >> /etc/apk/repositories; \
    fi

RUN apk --no-cache add ca-certificates curl
WORKDIR /
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /workspace/manager /manager
RUN chmod +x /manager
ENTRYPOINT ["/manager"]