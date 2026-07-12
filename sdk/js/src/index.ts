import * as canonicalizeModule from "canonicalize";
import { verifyMessage, getAddress } from "ethers";

// `canonicalize` is a CommonJS default-function export; under NodeNext the
// callable lives on `.default`. Resolve it robustly across interop shapes.
const canonicalize = ((canonicalizeModule as unknown as { default?: unknown }).default ??
  canonicalizeModule) as (value: unknown) => string | undefined;

/** The only AVP version this library validates. */
export const AVP_VERSION = "avp/1.0";

export interface AVPAttestation {
  signer: string;
  scheme: string; // "EIP-191"
  signature: string;
}

export interface AVPConsensus {
  sources: number;
  responders: number;
  agreed: number;
  quorum: number;
  agreedSources: string[];
}

export interface AVPFinality {
  finalized: boolean;
  finalizedBlock: number;
  confirmations: number;
}

/** An AgenticVerificationProof v1 object. Unknown additive fields are tolerated. */
export interface AVProof {
  schemaVersion: string;
  verified: boolean;
  chainId: number;
  blockNumber: number | null;
  blockHash: string | null;
  txHash: string;
  txIndex: number | null;
  consensus: AVPConsensus;
  finality: AVPFinality | null;
  merkleProof: unknown | null;
  attestation: AVPAttestation;
  bond: unknown | null;
  reason: string | null;
  issuedAt: number;
  [k: string]: unknown;
}

export interface VerifyResult {
  /** True only when the recovered address equals attestation.signer. */
  ok: boolean;
  /** The address recovered from the signature (empty string on recovery error). */
  recovered: string;
  /** The address the proof claims signed it. */
  signer: string;
  /** Set when ok is false: why. */
  reason?: string;
}

/**
 * recoverSigner recomputes the exact preimage the issuer signed — the proof
 * with its `attestation` member removed, serialized as RFC 8785 canonical JSON —
 * and recovers the EIP-191 (`personal_sign`) signer address.
 *
 * Byte-exactness is load-bearing: this MUST reproduce the issuer's canonical
 * bytes or recovery yields the wrong address.
 */
export function recoverSigner(proof: AVProof): string {
  const { attestation, ...unsigned } = proof;
  const bytes = canonicalize(unsigned);
  if (bytes === undefined) throw new Error("canonicalize returned undefined");
  return verifyMessage(bytes, (attestation as AVPAttestation).signature);
}

/**
 * verifyAVP checks a proof's signature (and, by default, its schema version).
 * Returns ok=true only when the recovered signer matches `attestation.signer`.
 * A valid signature vouches for EVERY other field — including a `verified:false`
 * verdict, which is a first-class, equally-trustworthy result.
 */
export function verifyAVP(
  proof: AVProof,
  opts: { requireVersion?: boolean } = {}
): VerifyResult {
  const requireVersion = opts.requireVersion ?? true;
  const claimed = proof?.attestation?.signer ?? "";
  if (requireVersion && proof?.schemaVersion !== AVP_VERSION) {
    return { ok: false, recovered: "", signer: claimed, reason: `unknown AVP version: ${proof?.schemaVersion}` };
  }
  let recovered = "";
  try {
    recovered = recoverSigner(proof);
  } catch (e) {
    return { ok: false, recovered: "", signer: claimed, reason: `recover failed: ${(e as Error).message}` };
  }
  let ok = false;
  try {
    ok = getAddress(recovered) === getAddress(claimed);
  } catch {
    ok = recovered.toLowerCase() === claimed.toLowerCase();
  }
  return ok ? { ok, recovered, signer: claimed } : { ok, recovered, signer: claimed, reason: `recovered ${recovered} != signer ${claimed}` };
}

/** assertAVP throws unless the proof's signature is valid. */
export function assertAVP(proof: AVProof, opts?: { requireVersion?: boolean }): void {
  const r = verifyAVP(proof, opts);
  if (!r.ok) throw new Error(`AVP verification failed: ${r.reason ?? "signature mismatch"}`);
}
