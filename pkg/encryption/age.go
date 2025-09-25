package encryption

import (
	"bytes"
	"fmt"
	"strings"

	"filippo.io/age"
	"filippo.io/age/agessh"
	gitev1 "github.com/mihaigalos/git-change-operator/api/v1"
)

// Encryptor handles age-based encryption
type Encryptor struct {
	recipients []age.Recipient
}

// NewEncryptor creates a new Encryptor from recipient specifications
func NewEncryptor(recipients []gitev1.Recipient) (*Encryptor, error) {
	var ageRecipients []age.Recipient

	for _, recipient := range recipients {
		var recipientValue string

		// Get the actual recipient value (either from Value or SecretRef)
		if recipient.Value != "" {
			recipientValue = recipient.Value
		} else if recipient.SecretRef != nil {
			// TODO: Implement secret resolution in controller
			return nil, fmt.Errorf("secret resolution not implemented yet for recipient")
		} else {
			return nil, fmt.Errorf("recipient must have either value or secretRef")
		}

		// Parse recipient based on type
		switch recipient.Type {
		case gitev1.RecipientTypeAge:
			r, err := age.ParseX25519Recipient(recipientValue)
			if err != nil {
				return nil, fmt.Errorf("failed to parse age recipient: %w", err)
			}
			ageRecipients = append(ageRecipients, r)

		case gitev1.RecipientTypeSSH:
			r, err := agessh.ParseRecipient(recipientValue)
			if err != nil {
				return nil, fmt.Errorf("failed to parse SSH recipient: %w", err)
			}
			ageRecipients = append(ageRecipients, r)

		case gitev1.RecipientTypePassphrase:
			r, err := age.NewScryptRecipient(recipientValue)
			if err != nil {
				return nil, fmt.Errorf("failed to create passphrase recipient: %w", err)
			}
			ageRecipients = append(ageRecipients, r)

		default:
			return nil, fmt.Errorf("unsupported recipient type: %s", recipient.Type)
		}
	}

	if len(ageRecipients) == 0 {
		return nil, fmt.Errorf("no valid recipients provided")
	}

	return &Encryptor{
		recipients: ageRecipients,
	}, nil
}

// Encrypt encrypts the given data and returns the encrypted bytes
func (e *Encryptor) Encrypt(data []byte) ([]byte, error) {
	var buf bytes.Buffer

	w, err := age.Encrypt(&buf, e.recipients...)
	if err != nil {
		return nil, fmt.Errorf("failed to create age writer: %w", err)
	}

	if _, err := w.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write data: %w", err)
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("failed to close age writer: %w", err)
	}

	return buf.Bytes(), nil
}

// GetFileExtension returns the appropriate file extension for encrypted files
func GetFileExtension(config *gitev1.Encryption) string {
	if config != nil && config.FileExtension != "" {
		return config.FileExtension
	}
	return ".age"
}

// ShouldEncryptFile determines if a file should be encrypted based on the path
func ShouldEncryptFile(path string, config *gitev1.Encryption) bool {
	if config == nil || !config.Enabled {
		return false
	}

	// Don't double-encrypt already encrypted files
	ext := GetFileExtension(config)
	return !strings.HasSuffix(path, ext)
}

// GetEncryptedFilePath returns the path for the encrypted version of a file
func GetEncryptedFilePath(originalPath string, config *gitev1.Encryption) string {
	ext := GetFileExtension(config)
	return originalPath + ext
}
