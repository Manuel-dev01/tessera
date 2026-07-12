import { MerklePatriciaTrie, verifyMPTWithMerkleProof } from "@ethereumjs/mpt";
import { hexToBytes } from "@ethereumjs/util";
import { keccak256 } from "ethers";

// This module is a SEPARATE entry point (`@tessera/avp/inclusion`) so the core
// signature verifier stays free of the Merkle-Patricia trie dependency. Import it
// only when you need to check the transactions-trie inclusion proof.

export interface AVPMerkleProof {
  type: string;
  transactionsRoot: string;
  txIndex: number;
  key: string;
  nodes: string[];
  leaf: string;
}

export interface InclusionInput {
  txHash: string;
  merkleProof: AVPMerkleProof | null;
}

export interface InclusionResult {
  ok: boolean;
  reason?: string;
}

/**
 * verifyInclusion checks a proof's transactions-trie inclusion proof: that the
 * proof nodes resolve, under the transactions root, to a leaf at RLP(txIndex)
 * whose keccak256 equals `txHash`.
 *
 * Pass `trustedRoot` (the block header's real transactionsRoot) for a FULLY
 * TRUSTLESS check — cryptographic proof the tx is in that block. Omit it to check
 * against the proof's own root, which relies on the AVP signature vouching for the
 * root (so also run `verifyAVP`).
 */
export async function verifyInclusion(
  proof: InclusionInput,
  opts: { trustedRoot?: string } = {}
): Promise<InclusionResult> {
  const mp = proof.merkleProof;
  if (!mp) return { ok: false, reason: "proof carries no merkleProof" };
  try {
    const root = hexToBytes((opts.trustedRoot ?? mp.transactionsRoot) as `0x${string}`);
    const key = hexToBytes(mp.key as `0x${string}`);
    const nodes = mp.nodes.map((n) => hexToBytes(n as `0x${string}`));
    const value = await verifyMPTWithMerkleProof(new MerklePatriciaTrie(), root, key, nodes);
    if (!value) return { ok: false, reason: `no tx at index ${mp.txIndex} under root` };
    const leafHash = keccak256(value);
    if (leafHash.toLowerCase() !== proof.txHash.toLowerCase()) {
      return { ok: false, reason: `proven leaf hashes to ${leafHash}, not txHash ${proof.txHash}` };
    }
    return { ok: true };
  } catch (e) {
    return { ok: false, reason: (e as Error).message };
  }
}
