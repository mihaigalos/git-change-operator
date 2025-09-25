package encryption

import (
	"bytes"
	"fmt"
	"io"

	"filippo.io/age"
	"filippo.io/age/agessh"
	gitev1 "github.com/mihaigalos/git-change-operator/api/v1"
)

// Decryptor handles age-based decryption
type Decryptor struct {
	identities []age.Identity
}

// NewDecryptorFromIdentities creates a new Decryptor from age.Identity objects
func NewDecryptorFromIdentities(identities []age.Identity) (*Decryptor, error) {
	if len(identities) == 0 {
		return nil, fmt.Errorf("no identities provided")
	}

	return &Decryptor{
		identities: identities,
	}, nil
}

// NewDecryptor creates a new Decryptor from identity specifications
func NewDecryptor(identities []gitev1.Recipient) (*Decryptor, error) {
	var ageIdentities []age.Identity

	for _, identity := range identities {
		var identityValue string

		if identity.Value != "" {
			identityValue = identity.Value
		} else if identity.SecretRef != nil {
			// TODO: Implement secret resolution
			return nil, fmt.Errorf("secret resolution not implemented yet for identity")
		} else {
			return nil, fmt.Errorf("identity must have either value or secretRef")
		}

		switch identity.Type {
		case gitev1.RecipientTypeAge:
			i, err := age.ParseX25519Identity(identityValue)
			if err != nil {
				return nil, fmt.Errorf("failed to parse age identity: %w", err)
			}
			ageIdentities = append(ageIdentities, i)

		case gitev1.RecipientTypeSSH:
			i, err := agessh.ParseIdentity([]byte(identityValue))
			if err != nil {
				return nil, fmt.Errorf("failed to parse SSH identity: %w", err)
			}
			ageIdentities = append(ageIdentities, i)

		case gitev1.RecipientTypePassphrase:
			i, err := age.NewScryptIdentity(identityValue)
			if err != nil {
				return nil, fmt.Errorf("failed to create passphrase identity: %w", err)
			}
			ageIdentities = append(ageIdentities, i)

		default:
			return nil, fmt.Errorf("unsupported identity type: %s", identity.Type)
		}
	}

	if len(ageIdentities) == 0 {
		return nil, fmt.Errorf("no valid identities provided")
	}

	return &Decryptor{
		identities: ageIdentities,
	}, nil
}

// Decrypt decrypts the given encrypted data and returns the plaintext bytes
func (d *Decryptor) Decrypt(encryptedData []byte) ([]byte, error) {
	r, err := age.Decrypt(bytes.NewReader(encryptedData), d.identities...)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		return nil, fmt.Errorf("failed to read decrypted data: %w", err)
	}

	return buf.Bytes(), nil
}

// ParseSSHIdentities parses SSH private key data and returns age identities
func ParseSSHIdentities(privateKeyData []byte) ([]age.Identity, error) {
	identity, err := agessh.ParseIdentity(privateKeyData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SSH identity: %w", err)
	}
	return []age.Identity{identity}, nil
}

// NewScryptIdentity creates a new scrypt-based identity from a passphrase
func NewScryptIdentity(passphrase string) (age.Identity, error) {
	identity, err := age.NewScryptIdentity(passphrase)
	if err != nil {
		return nil, fmt.Errorf("failed to create scrypt identity: %w", err)
	}
	return identity, nil
}
