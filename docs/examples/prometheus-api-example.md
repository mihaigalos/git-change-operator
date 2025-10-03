# Example GitCommit with REST API JSON parsing for Prometheus metrics

This example shows how to use the new REST API integration with JSON parsing to:
1. Query a Prometheus API endpoint
2. Parse the JSON response 
3. Extract specific data fields
4. Create a commit with the formatted data

## Example API Response

Your API endpoint returns this JSON:
```json
{
  "status": "success",
  "data": {
    "resultType": "scalar", 
    "result": [1759433836.397, "24.450000000004366"]
  }
}
```

## GitCommit Configuration

### Basic Example (without timestamp)

```yaml
apiVersion: gco.galos.one/v1
kind: GitCommit
metadata:
  name: prometheus-data-commit
  namespace: default
spec:
  repository: "https://github.com/your-username/metrics-data.git"
  branch: "main"
  commitMessage: "Update power consumption metrics"
  authSecretRef: "git-auth-secret"
  
  # REST API configuration with JSON parsing
  restAPI:
    url: "https://cloud.galos.one/prometheus/api/v1/query?query=scalar(max(max_over_time(smartmeter%7Bkind%3D%22total_power%22%7D%5B1d%5D))-max(max_over_time(smartmeter%7Bkind%3D%22total_power%22%7D%5B1d%5D%20offset%201d)))"
    method: "GET"
    timeoutSeconds: 30
    
    # JSON response parsing configuration using CEL (Common Expression Language)
    responseParsing:
      # Only proceed if status is "success"
      condition: 'has(status) && status == "success"'
      
      # Extract both values from the result array and join them
      dataExpression: 'string(data.result[0]) + "," + string(data.result[1])'
      
      # Format output with comma-space separator
      outputFormat: 'string(data.result[0]) + ", " + string(data.result[1])'
  
  files:
    - path: "metrics/power-consumption.txt"
      useRestAPIData: true  # This file will contain the formatted API response
    
    - path: "metadata/last-update.yaml"
      content: |
        timestamp: "$(date -Iseconds)"
        source: "prometheus-smartmeter-api"
        description: "Daily power consumption delta"
```

### Advanced Example (with timestamp)

```yaml
apiVersion: gco.galos.one/v1
kind: GitCommit
metadata:
  name: prometheus-data-with-timestamp
  namespace: default
spec:
  repository: "https://github.com/your-username/metrics-data.git"
  branch: "main"
  commitMessage: "Update power consumption metrics with timestamp"
  authSecretRef: "git-auth-secret"
  
  restAPI:
    url: "https://cloud.galos.one/prometheus/api/v1/query?query=scalar(max(max_over_time(smartmeter%7Bkind%3D%22total_power%22%7D%5B1d%5D))-max(max_over_time(smartmeter%7Bkind%3D%22total_power%22%7D%5B1d%5D%20offset%201d)))"
    method: "GET"
    
    responseParsing:
      condition: 'has(status) && status == "success"'
      dataExpression: 'string(data.result[0]) + "," + string(data.result[1])'
      # Format output with timestamp prefix - CEL includes timestamp automatically when specified
      outputFormat: 'string(now) + ", " + string(data.result[0]) + ", " + string(data.result[1])'
  
  files:
    - path: "metrics/power-consumption-timestamped.txt" 
      useRestAPIData: true
      # Will contain: "2025-10-02T21:49:50+02:00, 1759433836.397, 24.450000000004366"
```

### Multiple Files Example

```yaml
apiVersion: gco.galos.one/v1
kind: GitCommit
metadata:
  name: prometheus-multi-file-commit
  namespace: default
spec:
  repository: "https://github.com/your-username/metrics-data.git"
  branch: "main"
  commitMessage: "Update power metrics (multiple formats)"
  authSecretRef: "git-auth-secret"
  
  restAPI:
    url: "https://cloud.galos.one/prometheus/api/v1/query?query=scalar(max(max_over_time(smartmeter%7Bkind%3D%22total_power%22%7D%5B1d%5D))-max(max_over_time(smartmeter%7Bkind%3D%22total_power%22%7D%5B1d%5D%20offset%201d)))"
    method: "GET"
    
    responseParsing:
      condition: 'has(status) && status == "success"'
      dataExpression: 'string(data.result[0]) + "," + string(data.result[1])'
      outputFormat: 'string(now) + ", " + string(data.result[0]) + ", " + string(data.result[1])'
  
  files:
    # CSV format with timestamp
    - path: "data/metrics.csv"
      useRestAPIData: true
      
    # Custom README file (static content)
    - path: "README.md"
      content: |
        # Power Consumption Metrics
        
        This repository contains automated power consumption data from smart meters.
        Data is updated automatically via GitCommit operator.
        
        ## Format
        - `data/metrics.csv`: Timestamped CSV format
        - `json/metrics.json`: JSON format with metadata
        
    # JSON format (manual content using static values for now)
    - path: "json/metrics.json"
      content: |
        {
          "timestamp": "auto-generated",
          "source": "prometheus-api",
          "metrics": "see CSV for actual values"
        }
```

## Authentication Setup

Create a secret for Git authentication:

```bash
kubectl create secret generic git-auth-secret \
  --from-literal=token=ghp_your_github_token_here
```

## Expected Results

### Without timestamp (`includeTimestamp: false`):
File content: `1759433836.397, 24.450000000004366`

### With timestamp (`includeTimestamp: true`):  
File content: `2025-10-02T21:49:50+02:00, 1759433836.397, 24.450000000004366`

## Monitoring

Check the GitCommit status to see REST API details:

```bash
# Check status
kubectl get gitcommit prometheus-data-commit -o yaml

# View extracted data
kubectl get gitcommit prometheus-data-commit -o jsonpath='{.status.restAPIStatus.extractedData}'

# View formatted output
kubectl get gitcommit prometheus-data-commit -o jsonpath='{.status.restAPIStatus.formattedOutput}'
```

## CEL Expression Examples

The new CEL-based response parsing supports powerful expressions:

```yaml
# Your response structure
# {
#   "status": "success",
#   "data": {
#     "resultType": "scalar",
#     "result": [1759433836.397, "24.450000000004366"]
#   }
# }

# CEL Condition Examples:
condition: 'has(status) && status == "success"'
condition: 'has(data) && has(data.result) && size(data.result) >= 2'

# CEL Data Expression Examples:
dataExpression: 'status'                    # "success"
dataExpression: 'data.resultType'           # "scalar"  
dataExpression: 'string(data.result[0])'    # "1759433836.397"
dataExpression: 'string(data.result[0]) + "," + string(data.result[1])'  # Combined values

# CEL Output Format Examples:
outputFormat: 'string(data.result[0]) + ", " + string(data.result[1])'
outputFormat: 'string(now) + ": " + data.resultType + " = " + string(data.result[0])'
```

## Error Handling

The operator will:
1. Check HTTP status code first (must be ≤ 399 by default)
2. Parse JSON and validate condition field (`status == "success"`)
3. Extract data fields 
4. Only create commit if all conditions are met
5. Store detailed status in `.status.restAPIStatus` for debugging