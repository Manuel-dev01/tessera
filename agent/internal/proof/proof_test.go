package proof

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

// TestEchoProofMatchesSchema validates that the Phase 0 echo proof conforms to
// the published /schema/agentic-verification-proof-v1.json. Producer and the open
// standard must never drift.
func TestEchoProofMatchesSchema(t *testing.T) {
	schemaPath, err := filepath.Abs(filepath.Join("..", "..", "..", "schema", "agentic-verification-proof-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}

	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("avp.json", bytes.NewReader(schemaBytes)); err != nil {
		t.Fatalf("add schema: %v", err)
	}
	sch, err := compiler.Compile("avp.json")
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}

	p := BuildEcho(8453, "0x"+strings.Repeat("ab", 32), 12000000, 1720000000)
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if err := sch.Validate(doc); err != nil {
		t.Fatalf("echo proof does not conform to AVP v1 schema:\n%v", err)
	}
}

// TestCanonicalBytesExcludesAttestationAndSorts checks the signing preimage: the
// attestation is stripped and keys are lexicographically ordered.
func TestCanonicalBytesExcludesAttestationAndSorts(t *testing.T) {
	p := BuildEcho(8453, "0x"+strings.Repeat("cd", 32), 42, 1720000000)
	b, err := CanonicalBytes(p)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if strings.Contains(s, "attestation") {
		t.Fatalf("canonical bytes must not contain attestation: %s", s)
	}
	// Top-level keys must be sorted: blockHash < blockNumber < chainId < ...
	if !strings.HasPrefix(s, `{"blockHash":`) {
		t.Fatalf("canonical bytes not key-sorted, got prefix: %.40s", s)
	}
	// Must be compact (no insignificant whitespace).
	if strings.ContainsAny(s, "\n\t") || strings.Contains(s, ": ") {
		t.Fatalf("canonical bytes must be whitespace-free: %q", s)
	}
}
