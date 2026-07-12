# AgenticVerificationProof v1 (`avp/1.0`)

> A proposed open schema for the CROO agent ecosystem. Not Tessera-specific.

An **AgenticVerificationProof** (AVP) is a signed, consensus-backed attestation that a
specific on-chain event did, or did not, occur. It is the machine-verifiable receipt
one agent hands another as trustless evidence of an on-chain fact. Any agent can issue
one; any agent can verify one with nothing but the signer's address and a hashing
library.

A **signed negative** (`verified: false`) is a first-class result: a caller learns just
as much from a signed "this tx is not in that block" as from a signed confirmation.

- Formal schema: [`agentic-verification-proof-v1.json`](agentic-verification-proof-v1.json) (JSON Schema draft 2020-12)
- Version tag: `schemaVersion: "avp/1.0"`. Verifiers MUST reject unknown versions.

## Input (what a requester asks to have verified)

This is the CAP service **Requirements** schema, what the hiring agent submits.

```json
{
  "chainId": 8453,
  "address": "0x...",
  "txHash": "0x...",
  "blockNumber": 12345,
  "eventSignature": "Transfer(address,address,uint256)",
  "logIndex": 3
}
```

`eventSignature` and `logIndex` are optional; when present, the issuer additionally
confirms a matching log exists at that index.

### Verification semantics (Tessera reference issuer)

For `verified` to be `true`, all of the following must hold against the canonical chain:

- The tx exists and its receipt's block number equals the claimed `blockNumber`
  (else `reason: "tx not in claimed block N (actual M)"`).
- The receipt status is success; a reverted tx yields `verified:false`,
  `reason: "tx reverted (status 0)"`.
- **`address` is *involved***: it equals the tx sender (`from`), recipient (`to`),
  or the emitter of any log in the tx (case-insensitive). This is the intentionally
  permissive reading of "an address party to the event."
- If `eventSignature` is given, a log must exist whose `topics[0]` equals
  `keccak256(eventSignature)`. If `logIndex` is also given, that specific
  **block-level** log index must be the matching log.

A failure at any step produces a **signed** `verified:false` proof with `reason` set,
never an unsigned error.

### Consensus + finality (Tessera reference issuer)

- **Multi-source consensus.** Tessera queries N independent, method-diverse sources
  (many RPC operators + block-explorer indexers) concurrently. Each source that answers
  in time *votes* with its `(blockNumber, blockHash, status)`. A source that errors or
  times out simply abstains. The signed value is the one a **dynamic quorum** of
  *responders* agrees on. Quorum is `ceil(fraction * responders)` (default fraction 9/11),
  never below an absolute floor of 2. If fewer than `minResponders` answer, Tessera refuses
  to sign (`"insufficient live sources"`).
  - *Security note:* measuring quorum against responders (not a fixed configured count)
    favors liveness during outages, but would let an attacker who can force sources offline
    shrink the honest set. The `minResponders` floor and a high supermajority fraction bound
    that. Independence of the sources is the real security parameter.
- **Finality gate.** A block that is merely *included* can still be reorged out. Before
  signing `verified:true`, a quorum of head-reporting sources must confirm the block is at or
  under the chain's `finalized` tag (or ≥ a configured confirmation depth). The `finality`
  object records this. A consensus-valid but not-yet-final tx returns a signed
  `verified:false` with reason `"block N not finalized"`, protecting anything (e.g. a bond)
  that later relies on the proof.

## Output fields

| Field | Type | Meaning |
|---|---|---|
| `schemaVersion` | string | Always `"avp/1.0"`. |
| `verified` | bool | Confirmed by consensus. When `false`, `reason` is set. |
| `chainId` | int | Chain checked (Base = `8453`). |
| `blockNumber` | int \| null | Block containing the tx; null if not located. |
| `blockHash` | 0x-hash \| null | Canonical block hash agreed by consensus. |
| `txHash` | 0x-hash | The tx the requester asked about. |
| `txIndex` | int \| null | Position of the tx in its block. |
| `consensus` | object | `{sources, responders, agreed, quorum, agreedSources[]}`. Sources are queried concurrently; a dynamic quorum (supermajority of responders) must agree on `blockHash + status`. `verified` is only `true` when `agreed >= quorum`. |
| `finality` | object \| null | `{finalized, finalizedBlock, confirmations}`. `verified:true` requires the block to be final (or ≥ configured confirmations). Null when consensus was not reached. |
| `merkleProof` | object \| null | Transactions-trie inclusion proof: `{type:"transactionsTrie", transactionsRoot, txIndex, key, nodes[], leaf}`. Standalone cryptographic evidence the tx is at `txIndex` in the block, checkable against the header's `transactionsRoot`. Null when unavailable (e.g. no debug-capable source). See below. |
| `attestation` | object | `{signer, scheme:"EIP-191", signature}` over the canonical proof. |
| `bond` | object \| null | `{contract, stakedUSDC, challengeWindowSec, proofId, anchored, anchorTx}`. Advertises the oracle's standing USDC honesty stake; `anchored:true` (+ `anchorTx`) means this proof was individually committed on-chain and is trustlessly slashable. Null when bonding is disabled. |
| `reason` | string \| null | Why `verified` is false. Null on success. |
| `issuedAt` | int | Unix seconds when issued. |

## Transactions-trie inclusion proof (`merkleProof`)

A block header commits to its transactions via `transactionsRoot`, the root of a
Merkle-Patricia trie keyed by `RLP(txIndex)` with each transaction's *consensus
encoding* as the value. When present, `merkleProof` carries a standalone inclusion
proof for the verified tx:

| Field | Meaning |
|---|---|
| `type` | `"transactionsTrie"`. |
| `transactionsRoot` | The root this proof resolves to, equal to the block header's `transactionsRoot`. |
| `txIndex` | Position of the tx in the block. |
| `key` | `0x` RLP(txIndex), the trie key. |
| `nodes` | `0x` MPT proof nodes, root → leaf. |
| `leaf` | `0x` the tx's consensus encoding; `keccak256(leaf) == txHash`. |

It is **type-agnostic**: values are raw consensus encodings, so OP-stack deposit
transactions (type `0x7E`, always index 0 on Base) are handled with no special
casing. Two levels of trust:

- **Oracle-vouched**: the AVP signature covers `merkleProof`, so verifying the
  signature (above) plus checking `nodes` resolve under `transactionsRoot` to a
  leaf hashing to `txHash` proves the oracle attests the tx's inclusion.
- **Trustless**: additionally fetch the block header (by `blockHash`) and confirm
  `header.transactionsRoot == merkleProof.transactionsRoot`. Then the inclusion is
  proven with no trust in the oracle at all.

Reference verifiers ship in [`../sdk`](../sdk): Go `avp.VerifyInclusion(...)` and
JS `import { verifyInclusion } from "@olanuel-tessera/avp/inclusion"`.

## Signing scheme

The signature covers the **canonical byte form** of the proof object with the
`attestation` member removed.

1. Take the proof object, delete the `attestation` field.
2. Canonicalize to bytes per **RFC 8785 (JSON Canonicalization Scheme)**: object keys
   sorted lexicographically, no insignificant whitespace, canonical number and string
   forms, UTF-8.
3. Sign those bytes with **EIP-191** (`personal_sign`): the wallet prefixes
   `"\x19Ethereum Signed Message:\n" + len(bytes)` before hashing with kecc-256 and
   signing with the oracle key.
4. Place `{signer, scheme:"EIP-191", signature}` back into `attestation`.

> **Byte-exactness is load-bearing.** The verifier MUST recompute the identical
> canonical bytes. Any difference in key order, whitespace, or number formatting breaks
> the signature. Producer and consumer MUST both use RFC 8785.

### proofId / on-chain anchoring

`bond.proofId` is `keccak256` of the canonical proof with **both** `attestation` and `bond`
removed (the *core* fact identity, since a proof's identity must not change when it is bonded, and
`bond` cannot contain a hash of itself). The **signature** still covers `bond` (it strips only
`attestation`), so the bond claim is signed. When `anchored:true`, the oracle has earmarked part
of its standing stake against `proofId` in TesseraBond; anyone may call `challenge(proofId)` and,
if the claimed `blockHash` differs from the block's canonical hash (read on-chain via EIP-2935 /
`blockhash`), the stake is slashed to the challenger. No oracle or arbiter is involved.

## Verify a Tessera proof in ~10 lines (ethers v6)

```js
import { verifyMessage } from "ethers";
import canonicalize from "canonicalize"; // RFC 8785

export function verifyAVP(proof) {
  if (proof.schemaVersion !== "avp/1.0") throw new Error("unknown AVP version");
  const { attestation, ...unsigned } = proof;              // 1. strip attestation
  const bytes = canonicalize(unsigned);                    // 2. RFC 8785 canonical JSON
  const recovered = verifyMessage(bytes, attestation.signature); // 3. EIP-191 recover
  return recovered.toLowerCase() === attestation.signer.toLowerCase(); // 4. match signer
}
```

If `verifyAVP(proof)` returns `true`, the `signer` address vouches for every other field
in the proof, including a `verified: false` verdict.

> Reference verifiers for JS (`@olanuel-tessera/avp`) and Go (`github.com/Manuel-dev01/tessera/sdk/go`)
> live in [`../sdk`](../sdk). Both are tested against a real oracle-signed proof, proving the
> canonical byte form reproduces identically across implementations.

## Versioning

Breaking changes bump the tag (`avp/2.0`, and so on). Additive, backward-compatible fields may be
introduced within `avp/1.x` and MUST be ignored by older verifiers.
