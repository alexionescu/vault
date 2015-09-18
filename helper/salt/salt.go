package salt

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"

	"github.com/hashicorp/vault/helper/uuid"
	"github.com/hashicorp/vault/logical"
)

const (
	// DefaultLocation is the path in the view we store our key salt
	// if no other path is provided.
	DefaultLocation = "salt"
)

// Salt is used to manage a persistent salt key which is used to
// hash values. This allows keys to be generated and recovered
// using the global salt. Primarily, this allows paths in the storage
// backend to be obfuscated if they may contain sensitive information.
type Salt struct {
	config    *Config
	salt      string
	generated bool
	hmac      hash.Hash
}

type HashFunc func([]byte) []byte

// Config is used to parameterize the Salt
type Config struct {
	// Location is the path in the storage backend for the
	// salt. Uses DefaultLocation if not specified.
	Location string

	// HashFunc is the hashing function to use for salting.
	// Defaults to SHA1 if not provided.
	HashFunc HashFunc

	// HMAC allows specification of a hash function to use for
	// the HMAC helpers
	HMAC func() hash.Hash
}

// NewSalt creates a new salt based on the configuration
func NewSalt(view logical.Storage, config *Config) (*Salt, error) {
	// Setup the configuration
	if config == nil {
		config = &Config{}
	}
	if config.Location == "" {
		config.Location = DefaultLocation
	}
	if config.HashFunc == nil {
		config.HashFunc = SHA256Hash
	}

	// Create the salt
	s := &Salt{
		config: config,
	}

	// Look for the salt
	raw, err := view.Get(config.Location)
	if err != nil {
		return nil, fmt.Errorf("failed to read salt: %v", err)
	}

	// Restore the salt if it exists
	if raw != nil {
		s.salt = string(raw.Value)
	}

	// Generate a new salt if necessary
	if s.salt == "" {
		s.salt = uuid.GenerateUUID()
		s.generated = true
		raw = &logical.StorageEntry{
			Key:   config.Location,
			Value: []byte(s.salt),
		}
		if err := view.Put(raw); err != nil {
			return nil, fmt.Errorf("failed to persist salt: %v", err)
		}
	}

	if config.HMAC != nil {
		s.hmac = hmac.New(config.HMAC, []byte(s.salt))
		if s.hmac == nil {
			return nil, fmt.Errorf("failed to instantiate HMAC function")
		}
	}

	return s, nil
}

// SaltID is used to apply a salt and hash function to an ID to make sure
// it is not reversible
func (s *Salt) SaltID(id string) string {
	return SaltID(s.salt, id, s.config.HashFunc)
}

// SaltIDandHMAC is used to apply a salt and hash function to an ID to make sure
// it is not reversible, with an additional HMAC
func (s *Salt) GetHMAC(id string) string {
	if s.hmac == nil {
		return ""
	}
	s.hmac.Reset()
	s.hmac.Write([]byte(id))
	return string(s.hmac.Sum(nil))
}

// DidGenerate returns if the underlying salt value was generated
// on initialization or if an existing salt value was loaded
func (s *Salt) DidGenerate() bool {
	return s.generated
}

// SaltID is used to apply a salt and hash function to an ID to make sure
// it is not reversible
func SaltID(salt, id string, hash HashFunc) string {
	comb := salt + id
	hashVal := hash([]byte(comb))
	return hex.EncodeToString(hashVal)
}

// SHA1Hash returns the SHA1 of the input
func SHA1Hash(inp []byte) []byte {
	hashed := sha1.Sum(inp)
	return hashed[:]
}

// SHA256Hash returns the SHA256 of the input
func SHA256Hash(inp []byte) []byte {
	hashed := sha256.Sum256(inp)
	return hashed[:]
}
