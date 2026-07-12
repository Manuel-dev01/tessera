// TS mirror of the Go AVP proof + API payloads.

export interface Consensus {
  sources: number;
  responders: number;
  agreed: number;
  quorum: number;
  agreedSources: string[] | null;
}

export interface Finality {
  finalized: boolean;
  finalizedBlock: number;
  confirmations: number;
}

export interface Attestation {
  signer: string;
  scheme: string;
  signature: string;
}

export interface Bond {
  contract: string;
  stakedUSDC: string;
  challengeWindowSec: number;
  proofId: string;
  anchored: boolean;
  anchorTx?: string | null;
}

export interface Proof {
  schemaVersion: string;
  verified: boolean;
  chainId: number;
  blockNumber: number | null;
  blockHash: string | null;
  txHash: string;
  txIndex: number | null;
  consensus: Consensus;
  finality: Finality | null;
  merkleProof: unknown | null;
  attestation: Attestation;
  bond: Bond | null;
  reason: string | null;
  issuedAt: number;
}

export interface Health {
  chainId: number;
  signer: string;
  sources: string[];
  sourceCount: number;
  minResponders: number;
  quorumFraction: number;
  oracleEth: string;
  bondContract: string;
  bondEnabled: boolean;
  bondAnchorReady: boolean;
}

export interface TxPreview {
  txHash: string;
  blockNumber: number;
  blockHash: string;
  from: string;
  to: string;
  status: number;
  logs: { index: number; address: string; topic0: string }[];
}

export interface SourceReport {
  name: string;
  ok: boolean;
  found: boolean;
  blockNumber: number;
  blockHash: string;
  status: number;
  ms: number;
}

export interface ConsensusSummary {
  reached: boolean;
  sources: number;
  responders: number;
  agreed: number;
  quorum: number;
  agreedSources: string[] | null;
  finalOk: boolean;
  finality: Finality | null;
  reason: string | null;
}

export interface StoredProof {
  proofId: string;
  claim: string;
  verified: boolean;
  proof: Proof;
  issuedAt: number;
}
