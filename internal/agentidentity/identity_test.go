package agentidentity

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestNewIDIsStableForKnownEntropy(t *testing.T) {
	got, err := newID("a_", bytes.NewReader(make([]byte, randomBytes)))
	if err != nil {
		t.Fatal(err)
	}
	if got != "a_aaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("newID = %q", got)
	}
}

func TestNewIDReportsEntropyFailure(t *testing.T) {
	_, err := newID("i_", failingReader{})
	if err == nil || !strings.Contains(err.Error(), "generate i identity") {
		t.Fatalf("newID error = %v", err)
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("no entropy") }
