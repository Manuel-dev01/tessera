package avp

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

// The merkle fixture is a real signed proof that carries a transactions-trie
// inclusion proof; verifying it proves the SDK reproduces the trie verification.
func TestInclusion_RealProof(t *testing.T) {
	b, err := os.ReadFile("testdata/proof.merkle.json")
	if err != nil {
		t.Fatalf("read merkle fixture: %v", err)
	}
	// Signature must still verify with merkleProof present in the signed bytes.
	if r, err := Verify(b, true); err != nil || !r.OK {
		t.Fatalf("signature: ok=%v err=%v reason=%s", r.OK, err, r.Reason)
	}
	// Inclusion must verify against the proof's own root.
	ok, err := VerifyProofInclusion(b, nil)
	if err != nil || !ok {
		t.Fatalf("inclusion: ok=%v err=%v", ok, err)
	}
}

func TestInclusion_RejectsWrongTxHash(t *testing.T) {
	b, _ := os.ReadFile("testdata/proof.merkle.json")
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	m["txHash"] = "0x" + "11" + common.Bytes2Hex(common.HexToHash("0x0").Bytes())[2:] // arbitrary wrong hash
	bad, _ := json.Marshal(m)
	ok, _ := VerifyProofInclusion(bad, nil)
	if ok {
		t.Fatal("expected inclusion to reject a mismatched txHash")
	}
}

func TestInclusion_RejectsWrongTrustedRoot(t *testing.T) {
	b, _ := os.ReadFile("testdata/proof.merkle.json")
	bad := common.HexToHash("0xdeadbeef00000000000000000000000000000000000000000000000000000000")
	ok, _ := VerifyProofInclusion(b, &bad)
	if ok {
		t.Fatal("expected inclusion to fail against a wrong trusted root")
	}
}
