// Package sign produces EIP-191 attestations over the canonical form of an AVP
// proof, using Tessera's oracle key. The signature covers proof.CanonicalBytes
// (the proof with `attestation` removed, RFC 8785 canonical), so any verifier can
// recompute the same preimage and recover the signer.
package sign

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"tessera/agent/internal/proof"
)

// Signer holds the oracle key and its derived address.
type Signer struct {
	key  *ecdsa.PrivateKey
	addr common.Address
}

// NewSigner parses a hex-encoded secp256k1 private key (with or without 0x).
func NewSigner(hexKey string) (*Signer, error) {
	k, err := crypto.HexToECDSA(strings.TrimPrefix(strings.TrimSpace(hexKey), "0x"))
	if err != nil {
		return nil, fmt.Errorf("parse oracle key: %w", err)
	}
	return &Signer{key: k, addr: crypto.PubkeyToAddress(k.PublicKey)}, nil
}

// Address returns the checksummed oracle signer address.
func (s *Signer) Address() string { return s.addr.Hex() }

// Addr returns the signer address as a common.Address.
func (s *Signer) Addr() common.Address { return s.addr }

// Sign fills p.Attestation with an EIP-191 signature over the canonical proof.
// It signs both positive and negative proofs — a signed negative is valid output.
func (s *Signer) Sign(p *proof.Proof) error {
	preimage, err := proof.CanonicalBytes(*p)
	if err != nil {
		return fmt.Errorf("canonicalize: %w", err)
	}
	hash := accounts.TextHash(preimage) // "\x19Ethereum Signed Message:\n" + len + msg, keccak256
	sig, err := crypto.Sign(hash, s.key)
	if err != nil {
		return fmt.Errorf("sign: %w", err)
	}
	sig[64] += 27 // recovery id 0/1 -> 27/28 so ethers verifyMessage recovers
	p.Attestation = proof.Attestation{
		Signer:    s.addr.Hex(),
		Scheme:    "EIP-191",
		Signature: "0x" + hex.EncodeToString(sig),
	}
	return nil
}
