package cel

import (
	"testing"
)

func TestEvaluateCondition(t *testing.T) {
	evaluator, err := NewEvaluator()
	if err != nil {
		t.Fatalf("Failed to create evaluator: %v", err)
	}

	// Test data - Prometheus API response
	jsonData := []byte(`{"status":"success","data":{"resultType":"scalar","result":[1759433836.397,"24.450000000004366"]}}`)

	tests := []struct {
		name      string
		condition string
		expected  bool
		wantError bool
	}{
		{
			name:      "simple status check",
			condition: `response.status == "success"`,
			expected:  true,
			wantError: false,
		},
		{
			name:      "complex condition with size check",
			condition: `response.status == "success" && size(response.data.result) >= 2`,
			expected:  true,
			wantError: false,
		},
		{
			name:      "failing condition",
			condition: `response.status == "error"`,
			expected:  false,
			wantError: false,
		},
		{
			name:      "empty condition should pass",
			condition: "",
			expected:  true,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.EvaluateCondition(tt.condition, jsonData)
			if (err != nil) != tt.wantError {
				t.Errorf("EvaluateCondition() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if result != tt.expected {
				t.Errorf("EvaluateCondition() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestEvaluateDataExpression(t *testing.T) {
	evaluator, err := NewEvaluator()
	if err != nil {
		t.Fatalf("Failed to create evaluator: %v", err)
	}

	// Test data - Prometheus API response
	jsonData := []byte(`{"status":"success","data":{"resultType":"scalar","result":[1759433836.397,"24.450000000004366"]}}`)

	tests := []struct {
		name       string
		expression string
		expected   string
		wantError  bool
	}{
		{
			name:       "extract simple field",
			expression: `response.status`,
			expected:   `success`,
			wantError:  false,
		},
		{
			name:       "extract formatted string directly",
			expression: `string(response.data.result[0]) + "," + string(response.data.result[1])`,
			expected:   `1.759433836397e+09,24.450000000004366`,
			wantError:  false,
		},
		{
			name:       "extract first element",
			expression: `string(response.data.result[0])`,
			expected:   `1.759433836397e+09`,
			wantError:  false,
		},
		{
			name:       "empty expression",
			expression: "",
			expected:   "",
			wantError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.EvaluateDataExpression(tt.expression, jsonData)
			if (err != nil) != tt.wantError {
				t.Errorf("EvaluateDataExpression() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if result != tt.expected {
				t.Errorf("EvaluateDataExpression() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestProcessResponse(t *testing.T) {
	evaluator, err := NewEvaluator()
	if err != nil {
		t.Fatalf("Failed to create evaluator: %v", err)
	}

	// Test data - Prometheus API response
	jsonData := []byte(`{"status":"success","data":{"resultType":"scalar","result":[1759433836.397,"24.450000000004366"]}}`)

	req := ProcessRequest{
		Condition:      `response.status == "success" && size(response.data.result) >= 2`,
		DataExpression: `string(response.data.result[0]) + "," + string(response.data.result[1])`,
		OutputFormat:   `data`,
		ResponseData:   jsonData,
	}

	result, err := evaluator.ProcessResponse(req)
	if err != nil {
		t.Fatalf("ProcessResponse() error = %v", err)
	}

	if !result.ConditionMet {
		t.Error("Expected condition to be met")
	}

	expectedExtracted := `1.759433836397e+09,24.450000000004366`
	if result.ExtractedData != expectedExtracted {
		t.Errorf("ExtractedData = %v, want %v", result.ExtractedData, expectedExtracted)
	}

	expectedFormatted := `1.759433836397e+09,24.450000000004366`
	if result.FormattedOutput != expectedFormatted {
		t.Errorf("FormattedOutput = %v, want %v", result.FormattedOutput, expectedFormatted)
	}
}
