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

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager main.go

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]