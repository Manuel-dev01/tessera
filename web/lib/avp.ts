import canonicalize from "canonicalize";
import { verifyMessage } from "ethers";
import type { Proof } from "./types";

// verifyProofSignature recomputes the exact preimage the Go signer used
// (proof minus `attestation`, RFC 8785 canonical) and recovers the EIP-191
// signer in-browser. Mirrors /schema and internal/proof/CanonicalBytes.
export function verifyProofSignature(proof: Proof): { ok: boolean; recovered: string } {
  const { attestation, ...unsigned } = proof;
  const bytes = canonicalize(unsigned) as string;
  let recovered = "";
  try {
    recovered = verifyMessage(bytes, attestation.signature);
  } catch {
    return { ok: false, recovered: "" };
  }
  return {
    ok: recovered.toLowerCase() === attestation.signer.toLowerCase(),
    recovered,
  };
}
