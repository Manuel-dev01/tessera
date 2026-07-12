package sign

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"

	"tessera/agent/internal/proof"
)

func TestSignRecover(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	hexKey := hex.EncodeToString(crypto.FromECDSA(key))
	signer, err := NewSigner(hexKey)
	if err != nil {
		t.Fatal(err)
	}

	// A negative proof — signed negatives are valid output and must verify too.
	reason := "tx reverted (status 0)"
	p := proof.Build(proof.Fields{
		ChainID: 8453, TxHash: "0x" + strings.Repeat("ab", 32),
		Verified: false, Reason: &reason, IssuedAt: 1720000000,
	})
	if err := signer.Sign(&p); err != nil {
		t.Fatal(err)
	}

	if p.Attestation.Scheme != "EIP-191" {
		t.Fatalf("scheme = %q", p.Attestation.Scheme)
	}
	if !strings.EqualFold(p.Attestation.Signer, signer.Address()) {
		t.Fatalf("signer %q != %q", p.Attestation.Signer, signer.Address())
	}

	// Recover the signer the way an EIP-191 verifier (e.g. ethers) would.
	preimage, err := proof.CanonicalBytes(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(preimage), "attestation") {
		t.Fatalf("preimage must exclude attestation: %s", preimage)
	}
	digest := accounts.TextHash(preimage)

	sig, err := hex.DecodeString(strings.TrimPrefix(p.Attestation.Signature, "0x"))
	if err != nil {
		t.Fatal(err)
	}
	if len(sig) != 65 {
		t.Fatalf("sig len = %d, want 65", len(sig))
	}
	if sig[64] != 27 && sig[64] != 28 {
		t.Fatalf("v = %d, want 27/28", sig[64])
	}
	sig[64] -= 27 // back to 0/1 for SigToPub

	pub, err := crypto.SigToPub(digest, sig)
	if err != nil {
		t.Fatal(err)
	}
	recovered := crypto.PubkeyToAddress(*pub)
	if !strings.EqualFold(recovered.Hex(), signer.Address()) {
		t.Fatalf("recovered %s != oracle %s", recovered.Hex(), signer.Address())
	}
}
