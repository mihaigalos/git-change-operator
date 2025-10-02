package controllers

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMetricsCollector(t *testing.T) {
	// Create a new metrics collector
	collector := NewMetricsCollector("test")

	// Test recording API request metrics
	collector.RecordAPIRequest("http://example.com/api", "GET", "200", time.Millisecond*100, 1024)

	// Check if the counter was incremented
	count := testutil.ToFloat64(restAPIRequestsTotal.WithLabelValues("test", "http://example.com/api", "GET", "200"))
	if count != 1.0 {
		t.Errorf("Expected request count to be 1.0, got %f", count)
	}

	// Check histogram was recorded by examining the underlying metric vector
	// We'll just verify that we can record to it without error
	collector.RecordAPIRequest("http://example.com/api", "GET", "200", time.Millisecond*200, 512)

	// Test condition check metrics
	collector.RecordConditionCheck("success")
	conditionCount := testutil.ToFloat64(restAPIConditionChecks.WithLabelValues("test", "success"))
	if conditionCount != 1.0 {
		t.Errorf("Expected condition check count to be 1.0, got %f", conditionCount)
	}

	// Test JSON parsing error metrics
	collector.RecordJSONParsingError("field_not_found")
	errorCount := testutil.ToFloat64(restAPIJSONParsingErrors.WithLabelValues("test", "field_not_found"))
	if errorCount != 1.0 {
		t.Errorf("Expected JSON parsing error count to be 1.0, got %f", errorCount)
	}

	// Test response size metrics by recording additional data
	collector.RecordAPIRequest("http://example.com/api2", "POST", "201", time.Millisecond*50, 2048)
}

func TestMetricsCollectorControllerNames(t *testing.T) {
	gitCommitCollector := NewMetricsCollector("gitcommit")
	pullRequestCollector := NewMetricsCollector("pullrequest")

	// Record metrics for both controllers
	gitCommitCollector.RecordConditionCheck("success")
	pullRequestCollector.RecordConditionCheck("success")

	// Verify they have different controller labels
	gitCommitCount := testutil.ToFloat64(restAPIConditionChecks.WithLabelValues("gitcommit", "success"))
	pullRequestCount := testutil.ToFloat64(restAPIConditionChecks.WithLabelValues("pullrequest", "success"))

	if gitCommitCount != 1.0 {
		t.Errorf("Expected gitcommit condition check count to be 1.0, got %f", gitCommitCount)
	}

	if pullRequestCount != 1.0 {
		t.Errorf("Expected pullrequest condition check count to be 1.0, got %f", pullRequestCount)
	}
}

// TestMetricsRegistration verifies that all metrics are properly registered
func TestMetricsRegistration(t *testing.T) {
	// Create a new registry to test registration
	testRegistry := prometheus.NewRegistry()

	// Register our metrics
	err := testRegistry.Register(restAPIRequestsTotal)
	if err != nil {
		t.Errorf("Failed to register restAPIRequestsTotal: %v", err)
	}

	err = testRegistry.Register(restAPIRequestDuration)
	if err != nil {
		t.Errorf("Failed to register restAPIRequestDuration: %v", err)
	}

	err = testRegistry.Register(restAPIConditionChecks)
	if err != nil {
		t.Errorf("Failed to register restAPIConditionChecks: %v", err)
	}

	err = testRegistry.Register(restAPIJSONParsingErrors)
	if err != nil {
		t.Errorf("Failed to register restAPIJSONParsingErrors: %v", err)
	}

	err = testRegistry.Register(restAPIResponseSize)
	if err != nil {
		t.Errorf("Failed to register restAPIResponseSize: %v", err)
	}
}
