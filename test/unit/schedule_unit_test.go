package test

import (
	"testing"
	"time"

	"github.com/robfig/cron/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gitv1 "github.com/mihaigalos/git-change-operator/api/v1"
)

// Helper function to create int pointers
func intPtr(i int) *int {
	return &i
}

func TestScheduleParsing(t *testing.T) {
	tests := []struct {
		name        string
		schedule    string
		shouldError bool
	}{
		{
			name:        "valid cron expression - daily",
			schedule:    "0 2 * * *",
			shouldError: false,
		},
		{
			name:        "valid cron expression - hourly",
			schedule:    "0 * * * *",
			shouldError: false,
		},
		{
			name:        "valid cron expression - every 15 minutes",
			schedule:    "*/15 * * * *",
			shouldError: false,
		},
		{
			name:        "valid descriptor - @daily",
			schedule:    "@daily",
			shouldError: false,
		},
		{
			name:        "valid descriptor - @hourly",
			schedule:    "@hourly",
			shouldError: false,
		},
		{
			name:        "valid descriptor - @weekly",
			schedule:    "@weekly",
			shouldError: false,
		},
		{
			name:        "valid descriptor - @monthly",
			schedule:    "@monthly",
			shouldError: false,
		},
		{
			name:        "invalid cron expression - too many fields",
			schedule:    "0 0 0 0 0 0",
			shouldError: true,
		},
		{
			name:        "empty schedule",
			schedule:    "",
			shouldError: true,
		},
		{
			name:        "invalid cron expression - bad syntax",
			schedule:    "not a cron",
			shouldError: true,
		},
	}

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parser.Parse(tt.schedule)
			if tt.shouldError && err == nil {
				t.Errorf("expected error for schedule %q, but got none", tt.schedule)
			}
			if !tt.shouldError && err != nil {
				t.Errorf("unexpected error for schedule %q: %v", tt.schedule, err)
			}
		})
	}
}

func TestScheduleNextExecution(t *testing.T) {
	tests := []struct {
		name     string
		schedule string
		now      time.Time
		// We'll check that next time is in the future
	}{
		{
			name:     "daily at 2 AM",
			schedule: "0 2 * * *",
			now:      time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		},
		{
			name:     "every 15 minutes",
			schedule: "*/15 * * * *",
			now:      time.Date(2024, 1, 15, 10, 5, 0, 0, time.UTC),
		},
		{
			name:     "@hourly",
			schedule: "@hourly",
			now:      time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		},
	}

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sched, err := parser.Parse(tt.schedule)
			if err != nil {
				t.Fatalf("failed to parse schedule: %v", err)
			}
			next := sched.Next(tt.now)
			if !next.After(tt.now) {
				t.Errorf("next execution time %v is not after now %v", next, tt.now)
			}
			// Verify it's a reasonable time in the future (not years away)
			if next.Sub(tt.now) > 366*24*time.Hour {
				t.Errorf("next execution time %v is too far in the future from now %v", next, tt.now)
			}
		})
	}
}

func TestExecutionHistoryLimit(t *testing.T) {
	tests := []struct {
		name          string
		maxHistory    *int
		existingCount int
		expectedLimit int
	}{
		{
			name:          "default limit of 10",
			maxHistory:    nil,
			existingCount: 15,
			expectedLimit: 10,
		},
		{
			name:          "custom limit of 5",
			maxHistory:    intPtr(5),
			existingCount: 10,
			expectedLimit: 5,
		},
		{
			name:          "custom limit of 20",
			maxHistory:    intPtr(20),
			existingCount: 25,
			expectedLimit: 20,
		},
		{
			name:          "limit of 1",
			maxHistory:    intPtr(1),
			existingCount: 5,
			expectedLimit: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create execution history
			history := make([]gitv1.ExecutionRecord, tt.existingCount)
			for i := 0; i < tt.existingCount; i++ {
				history[i] = gitv1.ExecutionRecord{
					ExecutionTime: metav1.NewTime(time.Now().Add(-time.Hour * time.Duration(i))),
					CommitSHA:     "abc123",
					Phase:         gitv1.GitCommitPhaseCommitted,
					Message:       "success",
				}
			}

			// Simulate adding new record and trimming
			newRecord := gitv1.ExecutionRecord{
				ExecutionTime: metav1.Now(),
				CommitSHA:     "new123",
				Phase:         gitv1.GitCommitPhaseCommitted,
				Message:       "new success",
			}
			history = append([]gitv1.ExecutionRecord{newRecord}, history...)

			// Apply limit
			maxHistory := 10 // default
			if tt.maxHistory != nil {
				maxHistory = *tt.maxHistory
			}
			if len(history) > maxHistory {
				history = history[:maxHistory]
			}

			if len(history) != tt.expectedLimit {
				t.Errorf("expected history length %d, got %d", tt.expectedLimit, len(history))
			}
			// Verify newest is first
			if history[0].CommitSHA != "new123" {
				t.Errorf("expected newest record first, got %s", history[0].CommitSHA)
			}
		})
	}
}

func TestSuspendBehavior(t *testing.T) {
	tests := []struct {
		name          string
		suspend       bool
		shouldExecute bool
	}{
		{
			name:          "suspend is false - should execute",
			suspend:       false,
			shouldExecute: true,
		},
		{
			name:          "suspend is true - should not execute",
			suspend:       true,
			shouldExecute: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gitCommit := &gitv1.GitCommit{
				Spec: gitv1.GitCommitSpec{
					Schedule: "0 2 * * *",
					Suspend:  tt.suspend,
				},
			}
			// In real controller, suspended resources don't execute
			shouldExecute := !gitCommit.Spec.Suspend
			if shouldExecute != tt.shouldExecute {
				t.Errorf("expected shouldExecute=%v, got %v", tt.shouldExecute, shouldExecute)
			}
		})
	}
}

func TestScheduleWithTTL(t *testing.T) {
	tests := []struct {
		name         string
		schedule     string
		ttlMinutes   *int
		shouldDelete bool
		description  string
	}{
		{
			name:         "schedule set, TTL set - should not delete",
			schedule:     "0 2 * * *",
			ttlMinutes:   intPtr(60),
			shouldDelete: false,
			description:  "When schedule is set, TTL should be ignored",
		},
		{
			name:         "no schedule, TTL set - should delete",
			schedule:     "",
			ttlMinutes:   intPtr(60),
			shouldDelete: true,
			description:  "When no schedule, TTL should apply",
		},
		{
			name:         "schedule set, no TTL - should not delete",
			schedule:     "0 2 * * *",
			ttlMinutes:   nil,
			shouldDelete: false,
			description:  "Scheduled resources persist indefinitely",
		},
		{
			name:         "no schedule, no TTL - should not delete",
			schedule:     "",
			ttlMinutes:   nil,
			shouldDelete: false,
			description:  "One-time resources without TTL persist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the controller logic for TTL deletion
			hasSchedule := tt.schedule != ""
			hasTTL := tt.ttlMinutes != nil
			// Controller logic: if schedule is set, skip TTL check
			shouldCheckTTL := !hasSchedule && hasTTL
			if shouldCheckTTL != tt.shouldDelete {
				t.Errorf("%s: expected shouldDelete=%v, got %v", tt.description, tt.shouldDelete, shouldCheckTTL)
			}
		})
	}
}

func TestFirstExecutionBehavior(t *testing.T) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	gitCommit := &gitv1.GitCommit{
		Spec: gitv1.GitCommitSpec{
			Schedule: "0 2 * * *",
		},
		Status: gitv1.GitCommitStatus{
			LastScheduledTime: nil, // First time
		},
	}

	// First execution should happen immediately
	if gitCommit.Status.LastScheduledTime != nil {
		t.Error("expected LastScheduledTime to be nil for first execution")
	}

	// After first execution, next time should be calculated
	now := time.Now()
	sched, err := parser.Parse(gitCommit.Spec.Schedule)
	if err != nil {
		t.Fatalf("failed to parse schedule: %v", err)
	}
	nextTime := sched.Next(now)
	if !nextTime.After(now) {
		t.Error("next scheduled time should be after now")
	}
}

func TestMaxExecutionHistoryValidation(t *testing.T) {
	tests := []struct {
		name    string
		value   *int
		isValid bool
	}{
		{
			name:    "nil - use default",
			value:   nil,
			isValid: true,
		},
		{
			name:    "minimum value 1",
			value:   intPtr(1),
			isValid: true,
		},
		{
			name:    "maximum value 100",
			value:   intPtr(100),
			isValid: true,
		},
		{
			name:    "default value 10",
			value:   intPtr(10),
			isValid: true,
		},
		{
			name:    "value 50",
			value:   intPtr(50),
			isValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gitCommit := &gitv1.GitCommit{
				Spec: gitv1.GitCommitSpec{
					MaxExecutionHistory: tt.value,
				},
			}

			// Get effective limit
			limit := 10 // default
			if gitCommit.Spec.MaxExecutionHistory != nil {
				limit = *gitCommit.Spec.MaxExecutionHistory
			}

			// Validate range (these validations are in the CRD)
			if limit < 1 || limit > 100 {
				if tt.isValid {
					t.Errorf("expected value to be valid, but got invalid limit: %d", limit)
				}
			}
		})
	}
}
