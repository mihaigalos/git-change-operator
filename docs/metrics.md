# Metrics Documentation

The Git Change Operator provides comprehensive Prometheus metrics for monitoring REST API interactions and overall system health. These metrics help you understand the performance, reliability, and behavior of your REST API integrations.

## Available Metrics

### 1. REST API Request Metrics

#### `gitchange_rest_api_requests_total`
- **Type**: Counter
- **Description**: Total number of REST API requests made
- **Labels**: 
  - `controller`: The controller type (gitcommit or pullrequest)
  - `url`: The API endpoint URL
  - `method`: HTTP method (GET, POST, etc.)
  - `status_code`: HTTP response status code or "error" for failed requests

```promql
# Total requests by controller
sum by (controller) (gitchange_rest_api_requests_total)

# Error rate per API endpoint
rate(gitchange_rest_api_requests_total{status_code="error"}[5m]) / rate(gitchange_rest_api_requests_total[5m])

# Requests by status code
sum by (status_code) (gitchange_rest_api_requests_total)
```

#### `gitchange_rest_api_request_duration_seconds`
- **Type**: Histogram
- **Description**: Duration of REST API requests in seconds
- **Labels**: 
  - `controller`: The controller type (gitcommit or pullrequest)
  - `url`: The API endpoint URL
  - `method`: HTTP method (GET, POST, etc.)

```promql
# 95th percentile response time
histogram_quantile(0.95, rate(gitchange_rest_api_request_duration_seconds_bucket[5m]))

# Average response time by endpoint
rate(gitchange_rest_api_request_duration_seconds_sum[5m]) / rate(gitchange_rest_api_request_duration_seconds_count[5m])

# Slow requests (>1 second)
increase(gitchange_rest_api_request_duration_seconds_bucket{le="1.0"}[5m])
```

#### `gitchange_rest_api_response_size_bytes`
- **Type**: Histogram
- **Description**: Size of REST API responses in bytes
- **Labels**: 
  - `controller`: The controller type (gitcommit or pullrequest)
  - `url`: The API endpoint URL

```promql
# 95th percentile response size
histogram_quantile(0.95, rate(gitchange_rest_api_response_size_bytes_bucket[5m]))

# Large responses (>100KB)
increase(gitchange_rest_api_response_size_bytes_bucket{le="100000"}[5m])
```

### 2. Condition Check Metrics

#### `gitchange_rest_api_condition_checks_total`
- **Type**: Counter
- **Description**: Total number of REST API condition checks
- **Labels**: 
  - `controller`: The controller type (gitcommit or pullrequest)
  - `condition_result`: Result of the condition check
    - `success`: All conditions passed
    - `http_status_failed`: HTTP status code condition failed
    - `json_condition_failed`: JSON field condition failed

```promql
# Success rate of condition checks
rate(gitchange_rest_api_condition_checks_total{condition_result="success"}[5m]) / rate(gitchange_rest_api_condition_checks_total[5m])

# Failed conditions by type
sum by (condition_result) (gitchange_rest_api_condition_checks_total{condition_result!="success"})
```

### 3. JSON Parsing Metrics

#### `gitchange_rest_api_json_parsing_errors_total`
- **Type**: Counter
- **Description**: Total number of JSON parsing errors during REST API processing
- **Labels**: 
  - `controller`: The controller type (gitcommit or pullrequest)
  - `error_type`: Type of JSON parsing error
    - `processing_failed`: General JSON processing failure
    - `condition_field_extraction_failed`: Failed to extract condition field
    - `data_field_extraction_failed`: Failed to extract data fields

```promql
# JSON parsing error rate
rate(gitchange_rest_api_json_parsing_errors_total[5m])

# Errors by type
sum by (error_type) (gitchange_rest_api_json_parsing_errors_total)
```

## Example Grafana Dashboards

### REST API Overview Dashboard

```json
{
  "dashboard": {
    "title": "Git Change Operator - REST API Monitoring",
    "panels": [
      {
        "title": "API Request Rate",
        "targets": [
          {
            "expr": "sum(rate(gitchange_rest_api_requests_total[5m])) by (controller)",
            "legendFormat": "{{controller}}"
          }
        ],
        "type": "graph"
      },
      {
        "title": "API Response Times",
        "targets": [
          {
            "expr": "histogram_quantile(0.95, rate(gitchange_rest_api_request_duration_seconds_bucket[5m]))",
            "legendFormat": "95th percentile"
          },
          {
            "expr": "histogram_quantile(0.50, rate(gitchange_rest_api_request_duration_seconds_bucket[5m]))",
            "legendFormat": "median"
          }
        ],
        "type": "graph"
      },
      {
        "title": "Condition Check Success Rate",
        "targets": [
          {
            "expr": "rate(gitchange_rest_api_condition_checks_total{condition_result=\"success\"}[5m]) / rate(gitchange_rest_api_condition_checks_total[5m]) * 100",
            "legendFormat": "Success Rate %"
          }
        ],
        "type": "stat"
      }
    ]
  }
}
```

## Alerting Rules

### Critical Alerts

```yaml
groups:
  - name: git-change-operator.rules
    rules:
      # High API error rate
      - alert: GitChangeOperatorHighErrorRate
        expr: rate(gitchange_rest_api_requests_total{status_code="error"}[5m]) / rate(gitchange_rest_api_requests_total[5m]) > 0.1
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "High API error rate detected"
          description: "Git Change Operator API error rate is {{ $value | humanizePercentage }} for controller {{ $labels.controller }}"

      # High response times
      - alert: GitChangeOperatorSlowAPI
        expr: histogram_quantile(0.95, rate(gitchange_rest_api_request_duration_seconds_bucket[5m])) > 5
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "Slow API responses detected"
          description: "95th percentile API response time is {{ $value }}s"

      # JSON parsing failures
      - alert: GitChangeOperatorJSONParsingErrors
        expr: increase(gitchange_rest_api_json_parsing_errors_total[5m]) > 0
        for: 1m
        labels:
          severity: warning
        annotations:
          summary: "JSON parsing errors detected"
          description: "JSON parsing errors for {{ $labels.error_type }} in {{ $labels.controller }}"

      # Condition check failures
      - alert: GitChangeOperatorConditionCheckFailures
        expr: rate(gitchange_rest_api_condition_checks_total{condition_result!="success"}[5m]) > 0.05
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "Condition check failures detected"
          description: "Condition checks failing at {{ $value | humanizePercentage }} rate"
```

## Integration with Your Monitoring Stack

### Service Monitor for Prometheus Operator

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: git-change-operator-metrics
  namespace: git-change-operator-system
spec:
  selector:
    matchLabels:
      control-plane: controller-manager
  endpoints:
  - port: https
    scheme: https
    tlsConfig:
      insecureSkipVerify: true
    path: /metrics
```

### Accessing Metrics

The metrics are exposed on the controller manager's metrics port (default: 8443) at the `/metrics` endpoint. Make sure your Prometheus instance can scrape this endpoint.

**Example scrape configuration:**
```yaml
scrape_configs:
  - job_name: 'git-change-operator'
    kubernetes_sd_configs:
      - role: endpoints
        namespaces:
          names:
          - git-change-operator-system
    relabel_configs:
      - source_labels: [__meta_kubernetes_service_name]
        action: keep
        regex: git-change-operator-controller-manager-metrics-service
```

## Troubleshooting

### Common Issues

1. **No metrics visible**: Ensure the metrics port is accessible and not blocked by network policies
2. **High error rates**: Check API endpoint configuration and authentication
3. **JSON parsing errors**: Verify CEL expressions and API response format
4. **Slow responses**: Check API endpoint performance and timeout configuration

### Debug Queries

```promql
# Check if metrics are being recorded
up{job="git-change-operator"}

# Recent API calls
increase(gitchange_rest_api_requests_total[1h])

# Error details by status code
sum by (status_code, url) (gitchange_rest_api_requests_total{status_code!~"2.."})
```