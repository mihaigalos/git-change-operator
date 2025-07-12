# Corporate Environment Setup

This guide explains how to configure the Git Change Operator for corporate environments with proxy servers, artifact repositories, and SSL inspection.

## Overview

Corporate environments often require special configuration for:
- Go module proxy servers (Artifactory, Nexus)
- SSL certificate handling for HTTPS inspection
- Package repository proxies (Alpine, Ubuntu)
- HTTP/HTTPS proxy settings

The Git Change Operator supports corporate environments through external configuration files that override public defaults without modifying tracked code.

## Quick Start

1. **Copy the corporate configuration template:**
   ```bash
   cp corporate-config.env.example corporate-config.env
   ```

2. **Edit `corporate-config.env` with your corporate settings:**
   ```bash
   # Edit the file to match your corporate environment
   vim corporate-config.env
   ```

3. **Source the configuration before building:**
   ```bash
   source corporate-config.env
   make build
   ```

## Configuration Files

### corporate-config.env (git-ignored)

This file contains all corporate-specific environment variables that override public defaults:

```bash
# Go proxy configuration
export GOPROXY="https://your-artifactory.company.com/artifactory/api/go/go-remote,direct"
export GOSUMDB="off"

# Corporate URLs for build tools
export SETUP_ENVTEST_INDEX="https://your-artifactory.company.com/artifactory/api/go/go-remote/kubernetes-sigs/controller-tools/HEAD/envtest-releases.yaml"

# Docker build arguments for corporate environments
export DOCKER_BUILD_ARGS="--build-arg GOPROXY=$GOPROXY --build-arg GOSUMDB=$GOSUMDB --build-arg APK_MAIN_REPO=https://your-artifactory.company.com/artifactory/api/alpine/alpine-proxy/edge/main --build-arg APK_COMMUNITY_REPO=https://your-artifactory.company.com/artifactory/api/alpine/alpine-proxy/edge/community"

# SSL certificate configuration
export SSL_CERT_FILE="/path/to/corporate-ca.pem"
export REQUESTS_CA_BUNDLE="/path/to/corporate-ca.pem"
export CURL_CA_BUNDLE="/path/to/corporate-ca.pem"
```

## Building with Corporate Configuration

### Go Builds

The Makefile automatically uses environment variables when set:

```bash
source corporate-config.env
make build
```

### Docker Builds

Use the Makefile target which automatically handles corporate configuration:

```bash
source corporate-config.env
make docker-build
```

### Testing

For integration tests in corporate environments:

```bash
source corporate-config.env
make test-integration
```

## Environment Variables Reference

### Go Configuration

| Variable | Purpose | Example |
|----------|---------|---------|
| `GOPROXY` | Go module proxy | `https://artifactory.company.com/artifactory/api/go/go-remote,direct` |
| `GOSUMDB` | Checksum database | `off` (disable for corporate proxies) |
| `GONOPROXY` | Bypass proxy for domains | `github.company.com,internal.company.com` |
| `GONOSUMDB` | Bypass sumdb for domains | `github.company.com` |

### Build Tools

| Variable | Purpose | Example |
|----------|---------|---------|
| `SETUP_ENVTEST_INDEX` | Kubernetes test binaries index | `https://artifactory.company.com/artifactory/api/go/go-remote/kubernetes-sigs/controller-tools/HEAD/envtest-releases.yaml` |

### Docker Build Arguments

| Variable | Purpose | Example |
|----------|---------|---------|
| `APK_MAIN_REPO` | Alpine main repository | `https://artifactory.company.com/artifactory/api/alpine/alpine-proxy/edge/main` |
| `APK_COMMUNITY_REPO` | Alpine community repository | `https://artifactory.company.com/artifactory/api/alpine/alpine-proxy/edge/community` |

### SSL Certificates

| Variable | Purpose | Example |
|----------|---------|---------|
| `SSL_CERT_FILE` | CA certificate bundle | `/etc/ssl/certs/corporate-ca.pem` |
| `REQUESTS_CA_BUNDLE` | Python requests CA bundle | `/etc/ssl/certs/corporate-ca.pem` |
| `CURL_CA_BUNDLE` | cURL CA bundle | `/etc/ssl/certs/corporate-ca.pem` |

## Troubleshooting

### Common Issues

1. **SSL Certificate Errors**
   ```
   x509: certificate signed by unknown authority
   ```
   **Solution:** Ensure your corporate CA certificate is properly configured in `SSL_CERT_FILE`.

2. **Go Module Download Failures**
   ```
   go: module example.com/module: Get "https://proxy.golang.org/example.com/module": 403 Forbidden
   ```
   **Solution:** Configure `GOPROXY` to use your corporate proxy and set `GOSUMDB=off`.

3. **Docker Build Failures**
   ```
   ERROR: Could not satisfy requirements index
   ```
   **Solution:** Ensure `APK_MAIN_REPO` and `APK_COMMUNITY_REPO` are set correctly for Alpine packages.


## Security Considerations

- **Never commit `corporate-config.env` to public-facing repos** - it's already ignored in `.gitignore`
- **Keep corporate URLs and certificates confidential**
- **Regularly update corporate CA certificates**
- **Use principle of least provilege for service accounts**

## Integration with CI/CD

For automated builds in corporate CI/CD systems:

1. **Store corporate configuration as secrets**
2. **Inject environment variables at build time**
3. **Use corporate-specific build agents with pre-configured proxies**

Example GitHub Actions (for internal repositories):

```yaml
- name: Configure Corporate Environment
  run: |
    echo "GOPROXY=${{ secrets.CORPORATE_GOPROXY }}" >> $GITHUB_ENV
    echo "GOSUMDB=off" >> $GITHUB_ENV
    
- name: Build with Corporate Config
  run: make build
```