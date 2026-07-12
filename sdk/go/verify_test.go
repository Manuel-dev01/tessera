package avp

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func loadProof(t *testing.T) map[string]any {
	t.Helper()
	b, err := os.ReadFile("testdata/proof.verified.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return m
}

func mustJSON(t *testing.T, m map[string]any) []byte {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

// The fixture is a real proof signed by the Go oracle and served over the live
// API — verifying it here proves the standalone SDK reproduces the issuer's
// canonical bytes exactly.
func TestVerify_RealSignedProof(t *testing.T) {
	m := loadProof(t)
	r, err := Verify(mustJSON(t, m), true)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !r.OK {
		t.Fatalf("expected OK, got reason: %s", r.Reason)
	}
	want, _ := m["attestation"].(map[string]any)["signer"].(string)
	if !strings.EqualFold(r.Recovered, want) {
		t.Fatalf("recovered %s != signer %s", r.Recovered, want)
	}
}

func TestVerify_RejectsTamperedBlockNumber(t *testing.T) {
	m := loadProof(t)
	m["blockNumber"] = m["blockNumber"].(float64) + 1
	r, _ := Verify(mustJSON(t, m), true)
	if r.OK {
		t.Fatal("expected tampered proof to fail verification")
	}
}

func TestVerify_RejectsFlippedVerdict(t *testing.T) {
	m := loadProof(t)
	m["verified"] = !m["verified"].(bool)
	r, _ := Verify(mustJSON(t, m), true)
	if r.OK {
		t.Fatal("expected flipped verdict to fail verification")
	}
}

func TestVerify_RejectsUnknownVersion(t *testing.T) {
	m := loadProof(t)
	m["schemaVersion"] = "avp/2.0"
	r, _ := Verify(mustJSON(t, m), true)
	if r.OK || !strings.Contains(r.Reason, "unknown AVP version") {
		t.Fatalf("expected version rejection, got OK=%v reason=%s", r.OK, r.Reason)
	}
}
