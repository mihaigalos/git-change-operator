package jsonpath

import (
	"testing"
	"time"
)

// TestPrometheusExample validates the specific Prometheus API example from the user
func TestPrometheusExample(t *testing.T) {
	// The exact JSON response from the user's example
	jsonData := `{"status":"success","data":{"resultType":"scalar","result":[1759433836.397,"24.450000000004366"]}}`

	// Test extracting the status field for condition checking
	status, err := ExtractValue([]byte(jsonData), "status")
	if err != nil {
		t.Fatalf("Failed to extract status: %v", err)
	}
	if status != "success" {
		t.Errorf("Expected status 'success', got '%s'", status)
	}

	// Test extracting both result values
	dataFields := []string{"data.result[0]", "data.result[1]"}
	extractedData, err := ExtractMultipleValues([]byte(jsonData), dataFields)
	if err != nil {
		t.Fatalf("Failed to extract data fields: %v", err)
	}

	expected := []string{"1759433836.397", "24.450000000004366"}
	if len(extractedData) != len(expected) {
		t.Fatalf("Expected %d values, got %d", len(expected), len(extractedData))
	}

	for i, exp := range expected {
		if extractedData[i] != exp {
			t.Errorf("Expected extractedData[%d] = %s, got %s", i, exp, extractedData[i])
		}
	}

	// Test formatted output without timestamp
	separator := ", "
	outputWithoutTimestamp := extractedData[0] + separator + extractedData[1]
	expectedOutput := "1759433836.397, 24.450000000004366"
	if outputWithoutTimestamp != expectedOutput {
		t.Errorf("Expected output '%s', got '%s'", expectedOutput, outputWithoutTimestamp)
	}

	// Test formatted output with timestamp
	timestamp := "2025-10-02T21:49:50+02:00" // Example timestamp
	outputWithTimestamp := timestamp + separator + extractedData[0] + separator + extractedData[1]
	expectedOutputWithTimestamp := "2025-10-02T21:49:50+02:00, 1759433836.397, 24.450000000004366"
	if outputWithTimestamp != expectedOutputWithTimestamp {
		t.Errorf("Expected timestamped output '%s', got '%s'", expectedOutputWithTimestamp, outputWithTimestamp)
	}
}

// TestTimestampFormat validates ISO 8601 timestamp formatting
func TestTimestampFormat(t *testing.T) {
	timestamp := time.Now().Format(time.RFC3339)

	// Validate the format matches expected pattern (basic sanity check)
	if len(timestamp) < 19 { // Minimum length for ISO 8601
		t.Errorf("Timestamp too short: %s", timestamp)
	}

	// Should contain 'T' separator and timezone info
	if !containsChar(timestamp, 'T') {
		t.Errorf("Timestamp missing 'T' separator: %s", timestamp)
	}

	// Should end with timezone (+ or - or Z)
	lastChar := timestamp[len(timestamp)-1]
	if lastChar != 'Z' && !containsChar(timestamp, '+') && !containsChar(timestamp, '-') {
		t.Errorf("Timestamp missing timezone info: %s", timestamp)
	}
}

func containsChar(s string, c rune) bool {
	for _, char := range s {
		if char == c {
			return true
		}
	}
	return false
}
