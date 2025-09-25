package test

import (
	"testing"

	"github.com/mihaigalos/git-change-operator/pkg/encryption"
	gitev1 "github.com/mihaigalos/git-change-operator/api/v1"
)

func TestShouldEncryptFile(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		config         *gitev1.Encryption
		expectedResult bool
	}{
		{
			name:           "nil config",
			path:           "test.txt",
			config:         nil,
			expectedResult: false,
		},
		{
			name: "encryption disabled",
			path: "test.txt",
			config: &gitev1.Encryption{
				Enabled: false,
			},
			expectedResult: false,
		},
		{
			name: "encryption enabled - should encrypt",
			path: "test.txt",
			config: &gitev1.Encryption{
				Enabled: true,
			},
			expectedResult: true,
		},
		{
			name: "already encrypted file - should not encrypt",
			path: "test.txt.age",
			config: &gitev1.Encryption{
				Enabled: true,
			},
			expectedResult: false,
		},
		{
			name: "custom extension - should encrypt",
			path: "test.txt",
			config: &gitev1.Encryption{
				Enabled:       true,
				FileExtension: ".enc",
			},
			expectedResult: true,
		},
		{
			name: "custom extension - already encrypted",
			path: "test.txt.enc",
			config: &gitev1.Encryption{
				Enabled:       true,
				FileExtension: ".enc",
			},
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := encryption.ShouldEncryptFile(tt.path, tt.config)
			if result != tt.expectedResult {
				t.Errorf("ShouldEncryptFile() = %v, want %v", result, tt.expectedResult)
			}
		})
	}
}

func TestGetFileExtension(t *testing.T) {
	tests := []struct {
		name     string
		config   *gitev1.Encryption
		expected string
	}{
		{
			name:     "nil config",
			config:   nil,
			expected: ".age",
		},
		{
			name: "empty extension",
			config: &gitev1.Encryption{
				FileExtension: "",
			},
			expected: ".age",
		},
		{
			name: "custom extension",
			config: &gitev1.Encryption{
				FileExtension: ".enc",
			},
			expected: ".enc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := encryption.GetFileExtension(tt.config)
			if result != tt.expected {
				t.Errorf("GetFileExtension() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetEncryptedFilePath(t *testing.T) {
	tests := []struct {
		name         string
		originalPath string
		config       *gitev1.Encryption
		expected     string
	}{
		{
			name:         "default extension",
			originalPath: "test.txt",
			config: &gitev1.Encryption{
				Enabled: true,
			},
			expected: "test.txt.age",
		},
		{
			name:         "custom extension",
			originalPath: "test.yaml",
			config: &gitev1.Encryption{
				Enabled:       true,
				FileExtension: ".enc",
			},
			expected: "test.yaml.enc",
		},
		{
			name:         "path with directories",
			originalPath: "configs/app.yaml",
			config: &gitev1.Encryption{
				Enabled: true,
			},
			expected: "configs/app.yaml.age",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := encryption.GetEncryptedFilePath(tt.originalPath, tt.config)
			if result != tt.expected {
				t.Errorf("GetEncryptedFilePath() = %v, want %v", result, tt.expected)
			}
		})
	}
}