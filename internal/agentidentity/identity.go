// Package agentidentity generates opaque identifiers for durable Orc agents
// and their individual live process instances.
package agentidentity

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"io"
	"strings"
)

const randomBytes = 16

var idEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewAgentID returns a durable identity that survives provider resumes.
func NewAgentID() (string, error) {
	return newID("a_", rand.Reader)
}

// NewInstanceID returns the identity of one live agent process launch.
func NewInstanceID() (string, error) {
	return newID("i_", rand.Reader)
}

func newID(prefix string, source io.Reader) (string, error) {
	data := make([]byte, randomBytes)
	if _, err := io.ReadFull(source, data); err != nil {
		return "", fmt.Errorf("generate %s identity: %w", strings.TrimSuffix(prefix, "_"), err)
	}
	return prefix + strings.ToLower(idEncoding.EncodeToString(data)), nil
}
