package jsonpath

import (
	"testing"
)

func TestExtractValue(t *testing.T) {
	// Test with your example JSON response
	jsonData := `{"status":"success","data":{"resultType":"scalar","result":[1759433836.397,"24.450000000004366"]}}`

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "extract status",
			path:     "status",
			expected: "success",
		},
		{
			name:     "extract result first element",
			path:     "data.result[0]",
			expected: "1759433836.397",
		},
		{
			name:     "extract result second element",
			path:     "data.result[1]",
			expected: "24.450000000004366",
		},
		{
			name:     "extract resultType",
			path:     "data.resultType",
			expected: "scalar",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := ExtractValue([]byte(jsonData), test.path)
			if err != nil {
				t.Fatalf("ExtractValue failed: %v", err)
			}
			if result != test.expected {
				t.Errorf("Expected %s, got %s", test.expected, result)
			}
		})
	}
}

func TestExtractMultipleValues(t *testing.T) {
	jsonData := `{"status":"success","data":{"resultType":"scalar","result":[1759433836.397,"24.450000000004366"]}}`

	paths := []string{"data.result[0]", "data.result[1]"}
	expected := []string{"1759433836.397", "24.450000000004366"}

	result, err := ExtractMultipleValues([]byte(jsonData), paths)
	if err != nil {
		t.Fatalf("ExtractMultipleValues failed: %v", err)
	}

	if len(result) != len(expected) {
		t.Fatalf("Expected %d results, got %d", len(expected), len(result))
	}

	for i, exp := range expected {
		if result[i] != exp {
			t.Errorf("Expected result[%d] = %s, got %s", i, exp, result[i])
		}
	}
}

func TestComplexPaths(t *testing.T) {
	jsonData := `{
		"nested": {
			"array": [
				{"field": "value1"},
				{"field": "value2"}
			],
			"simple": "test"
		}
	}`

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "nested simple field",
			path:     "nested.simple",
			expected: "test",
		},
		{
			name:     "nested array element field",
			path:     "nested.array[0].field",
			expected: "value1",
		},
		{
			name:     "nested array second element field",
			path:     "nested.array[1].field",
			expected: "value2",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := ExtractValue([]byte(jsonData), test.path)
			if err != nil {
				t.Fatalf("ExtractValue failed: %v", err)
			}
			if result != test.expected {
				t.Errorf("Expected %s, got %s", test.expected, result)
			}
		})
	}
}
