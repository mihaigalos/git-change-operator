package test

import (
	"testing"

	gitv1 "github.com/mihaigalos/git-change-operator/api/v1"
	"github.com/mihaigalos/git-change-operator/pkg/encryption"
	"github.com/stretchr/testify/require"
)

func TestPullRequestEncryptionUtils(t *testing.T) {
	// Test case: PullRequest with encryption enabled
	encryptionConfig := &gitv1.Encryption{
		Enabled: true,
		Recipients: []gitv1.Recipient{
			{
				Type: gitv1.RecipientTypeSSH,
				SecretRef: &gitv1.SecretRef{
					Name: "ssh-keys",
					Key:  "id_rsa.pub",
				},
			},
		},
	}

	// Test that file should be encrypted
	t.Run("shouldEncryptFile", func(t *testing.T) {
		should := encryption.ShouldEncryptFile("secret.yaml", encryptionConfig)
		require.True(t, should)

		// Already encrypted files should not be encrypted again
		shouldNot := encryption.ShouldEncryptFile("secret.yaml.age", encryptionConfig)
		require.False(t, shouldNot)

		// Nil config should return false
		shouldNotNil := encryption.ShouldEncryptFile("secret.yaml", nil)
		require.False(t, shouldNotNil)

		// Disabled encryption should return false
		disabledConfig := &gitv1.Encryption{Enabled: false}
		shouldNotDisabled := encryption.ShouldEncryptFile("secret.yaml", disabledConfig)
		require.False(t, shouldNotDisabled)
	})

	// Test encrypted file path generation
	t.Run("getEncryptedFilePath", func(t *testing.T) {
		encryptedPath := encryption.GetEncryptedFilePath("secret.yaml", encryptionConfig)
		require.Equal(t, "secret.yaml.age", encryptedPath)

		// Test with path
		encryptedPathWithDir := encryption.GetEncryptedFilePath("config/secret.yaml", encryptionConfig)
		require.Equal(t, "config/secret.yaml.age", encryptedPathWithDir)

		// Test with custom extension
		customConfig := &gitv1.Encryption{
			Enabled:       true,
			FileExtension: ".encrypted",
		}
		customPath := encryption.GetEncryptedFilePath("secret.yaml", customConfig)
		require.Equal(t, "secret.yaml.encrypted", customPath)
	})
}
