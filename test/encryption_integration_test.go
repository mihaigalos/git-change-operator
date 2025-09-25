package test

import (
	"os"
	"strings"
	"testing"

	"github.com/mihaigalos/git-change-operator/pkg/encryption"
	gitev1 "github.com/mihaigalos/git-change-operator/api/v1"
	"filippo.io/age"
	"filippo.io/age/agessh"
)

// Test keys and passphrases for testing
const (
	testPassphrase = "test-passphrase-123"
)

// generateTestKeys creates a valid age key pair for testing
func generateTestKeys(t *testing.T) (privateKey string, publicKey string) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("Failed to generate test identity: %v", err)
	}
	
	return identity.String(), identity.Recipient().String()
}

func TestEncryptDecryptWithAgeKey(t *testing.T) {
	content := []byte("Hello, World! This is a test message.")

	// Generate test keys
	testPrivateKey, testPublicKey := generateTestKeys(t)

	// Test with age key recipient
	recipients := []gitev1.Recipient{
		{
			Type:  gitev1.RecipientTypeAge,
			Value: testPublicKey,
		},
	}

	encryptor, err := encryption.NewEncryptor(recipients)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	encryptedContent, err := encryptor.Encrypt(content)
	if err != nil {
		t.Fatalf("Failed to encrypt content: %v", err)
	}

	// Verify content is encrypted (should not be plaintext)
	if strings.Contains(string(encryptedContent), string(content)) {
		t.Error("Encrypted content contains plaintext - encryption may have failed")
	}

	// Test decryption
	identities := []age.Identity{}
	privateKey, err := age.ParseX25519Identity(testPrivateKey)
	if err != nil {
		t.Fatalf("Failed to parse private key: %v", err)
	}
	identities = append(identities, privateKey)

	decryptor, err := encryption.NewDecryptorFromIdentities(identities)
	if err != nil {
		t.Fatalf("Failed to create decryptor: %v", err)
	}

	decryptedContent, err := decryptor.Decrypt(encryptedContent)
	if err != nil {
		t.Fatalf("Failed to decrypt content: %v", err)
	}

	if string(decryptedContent) != string(content) {
		t.Errorf("Decrypted content does not match original. Got: %s, Want: %s", string(decryptedContent), string(content))
	}
}

func TestEncryptDecryptWithPassphrase(t *testing.T) {
	content := []byte("Secret message encrypted with passphrase")

	// Test with passphrase recipient
	recipients := []gitev1.Recipient{
		{
			Type:  gitev1.RecipientTypePassphrase,
			Value: testPassphrase,
		},
	}

	encryptor, err := encryption.NewEncryptor(recipients)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	encryptedContent, err := encryptor.Encrypt(content)
	if err != nil {
		t.Fatalf("Failed to encrypt content: %v", err)
	}

	// Verify content is encrypted
	if strings.Contains(string(encryptedContent), string(content)) {
		t.Error("Encrypted content contains plaintext - encryption may have failed")
	}

	// Test decryption
	passphraseIdentity, err := age.NewScryptIdentity(testPassphrase)
	if err != nil {
		t.Fatalf("Failed to create passphrase identity: %v", err)
	}
	identities := []age.Identity{passphraseIdentity}

	decryptor, err := encryption.NewDecryptorFromIdentities(identities)
	if err != nil {
		t.Fatalf("Failed to create decryptor: %v", err)
	}

	decryptedContent, err := decryptor.Decrypt(encryptedContent)
	if err != nil {
		t.Fatalf("Failed to decrypt content: %v", err)
	}

	if string(decryptedContent) != string(content) {
		t.Errorf("Decrypted content does not match original. Got: %s, Want: %s", string(decryptedContent), string(content))
	}
}

func TestEncryptWithMultipleRecipients(t *testing.T) {
	content := []byte("Message for multiple recipients")

	// Generate test keys
	testPrivateKey, testPublicKey := generateTestKeys(t)
	_, testPublicKey2 := generateTestKeys(t)

	// Test with multiple age recipients (age supports multiple keys of same type)
	recipients := []gitev1.Recipient{
		{
			Type:  gitev1.RecipientTypeAge,
			Value: testPublicKey,
		},
		{
			Type:  gitev1.RecipientTypeAge,
			Value: testPublicKey2,
		},
	}

	encryptor, err := encryption.NewEncryptor(recipients)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	encryptedContent, err := encryptor.Encrypt(content)
	if err != nil {
		t.Fatalf("Failed to encrypt content: %v", err)
	}

	// Test decryption with age key
	identities1 := []age.Identity{}
	privateKey, err := age.ParseX25519Identity(testPrivateKey)
	if err != nil {
		t.Fatalf("Failed to parse private key: %v", err)
	}
	identities1 = append(identities1, privateKey)

	decryptor1, err := encryption.NewDecryptorFromIdentities(identities1)
	if err != nil {
		t.Fatalf("Failed to create decryptor: %v", err)
	}

	decryptedContent1, err := decryptor1.Decrypt(encryptedContent)
	if err != nil {
		t.Fatalf("Failed to decrypt content with age key: %v", err)
	}

	if string(decryptedContent1) != string(content) {
		t.Errorf("Decrypted content (age) does not match original. Got: %s, Want: %s", string(decryptedContent1), string(content))
	}

	// Note: Since we're using two age recipients, we can decrypt with the first key only
	// The age library encrypts to all recipients, so any valid key can decrypt
}

func TestEncryptWithMixedRecipientTypes(t *testing.T) {
	// Test that the age library correctly rejects incompatible recipient types
	// This is expected behavior - age doesn't support mixing different types
	
	// Generate test keys
	_, testPublicKey := generateTestKeys(t)

	// Test with mixed recipients (age + passphrase) - this should fail
	recipients := []gitev1.Recipient{
		{
			Type:  gitev1.RecipientTypeAge,
			Value: testPublicKey,
		},
		{
			Type:  gitev1.RecipientTypePassphrase,
			Value: testPassphrase,
		},
	}

	encryptor, err := encryption.NewEncryptor(recipients)
	if err != nil {
		// This is actually expected - our API allows it but age library rejects it
		// In practice, users should use consistent recipient types
		t.Logf("Expected: age library rejects mixed recipient types: %v", err)
		return
	}

	content := []byte("Message for mixed recipients")
	_, err = encryptor.Encrypt(content)
	if err != nil && strings.Contains(err.Error(), "incompatible") {
		// This is the expected behavior
		t.Logf("Expected: encryption fails with incompatible recipients: %v", err)
		return
	}

	if err == nil {
		t.Error("Expected error when encrypting with incompatible recipient types, but encryption succeeded")
	} else {
		t.Errorf("Unexpected error type: %v", err)
	}
}

func TestNewEncryptorErrors(t *testing.T) {
	// Generate test keys
	_, testPublicKey := generateTestKeys(t)

	tests := []struct {
		name       string
		recipients []gitev1.Recipient
		shouldErr  bool
	}{
		{
			name:       "empty recipients",
			recipients: []gitev1.Recipient{},
			shouldErr:  true,
		},
		{
			name: "invalid age key",
			recipients: []gitev1.Recipient{
				{
					Type:  gitev1.RecipientTypeAge,
					Value: "invalid-age-key",
				},
			},
			shouldErr: true,
		},
		{
			name: "unknown recipient type",
			recipients: []gitev1.Recipient{
				{
					Type:  "unknown",
					Value: "some-value",
				},
			},
			shouldErr: true,
		},
		{
			name: "valid age key",
			recipients: []gitev1.Recipient{
				{
					Type:  gitev1.RecipientTypeAge,
					Value: testPublicKey,
				},
			},
			shouldErr: false,
		},
		{
			name: "valid passphrase",
			recipients: []gitev1.Recipient{
				{
					Type:  gitev1.RecipientTypePassphrase,
					Value: testPassphrase,
				},
			},
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := encryption.NewEncryptor(tt.recipients)
			if tt.shouldErr && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestSSHKeySupport(t *testing.T) {
	content := []byte("SSH encrypted message")

	// Read the public key from test resources
	publicKeyBytes, err := os.ReadFile("resources/id_rsa_4096.pub")
	if err != nil {
		t.Fatalf("Failed to read public key: %v", err)
	}

	// Parse SSH public key
	sshPublicKey := strings.TrimSpace(string(publicKeyBytes))

	// Test encryption with SSH key
	recipients := []gitev1.Recipient{
		{
			Type:  gitev1.RecipientTypeSSH,
			Value: sshPublicKey,
		},
	}

	encryptor, err := encryption.NewEncryptor(recipients)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	encryptedContent, err := encryptor.Encrypt(content)
	if err != nil {
		t.Fatalf("Failed to encrypt content: %v", err)
	}

	// Verify content is encrypted
	if strings.Contains(string(encryptedContent), string(content)) {
		t.Error("Encrypted content contains plaintext - encryption may have failed")
	}

	// Read private key
	privateKeyBytes, err := os.ReadFile("resources/id_rsa_4096")
	if err != nil {
		t.Fatalf("Failed to read private key: %v", err)
	}

	// Parse SSH private key
	sshIdentity, err := agessh.ParseIdentity(privateKeyBytes)
	if err != nil {
		t.Fatalf("Failed to parse SSH private key: %v", err)
	}

	// Create identities array
	identities := []age.Identity{sshIdentity}

	decryptor, err := encryption.NewDecryptorFromIdentities(identities)
	if err != nil {
		t.Fatalf("Failed to create decryptor: %v", err)
	}

	decryptedContent, err := decryptor.Decrypt(encryptedContent)
	if err != nil {
		t.Fatalf("Failed to decrypt content: %v", err)
	}

	if string(decryptedContent) != string(content) {
		t.Errorf("Decrypted content does not match original. Got: %s, Want: %s", string(decryptedContent), string(content))
	}
}