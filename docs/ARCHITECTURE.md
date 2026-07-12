# Architecture

This document describes how Tessera is built: the components, the end-to-end
request lifecycle, the proof format and its signing scheme, the trust model, the
on-chain bond and slashing mechanism, and the deployment topology.

For what Tessera is and how to run it, see the [README](../README.md). For the
formal proof format, see [schema/](../schema).

## Contents

- [System overview](#system-overview)
- [Components](#components)
- [The verification pipeline](#the-verification-pipeline)
- [The AVP object and signing scheme](#the-avp-object-and-signing-scheme)
- [Trust model](#trust-model)
- [On-chain bond and slashing](#on-chain-bond-and-slashing)
- [Deployment topology](#deployment-topology)
- [Technology choices](#technology-choices)

## System overview

Tessera turns a request about an on-chain event into a signed, portable proof
that any party can verify without trusting Tessera. The verification stack is a
Go service. It reaches consensus across many independent data sources, gates on
finality, signs the result, attaches a cryptographic inclusion proof, and can
back the proof with an on-chain honesty stake. The proof format is an open schema
with reference verifiers in two languages.

```
   caller agent
        │  hires over CAP  (live)   or   POST /api/verify  (console/API)
        ▼
┌──────────────────────── Tessera provider (Go) ─────────────────────────┐
│                                                                         │
│  consensus.Engine ─▶ 11 SourceAdapters ─▶ Base RPCs + block explorer    │
│        │  agree on (blockNumber, blockHash, status), dynamic quorum     │
│        ▼                                                                │
│  finality gate ────▶ quorum must confirm block ≤ finalized head         │
│        ▼                                                                │
│  verify.Verifier ──▶ applies the caller's claim to the agreed facts     │
│        ▼                                                                │
│  sign.Signer ──────▶ EIP-191 over RFC 8785 canonical JSON               │
│        ▼                                                                │
│  merkleproof ──────▶ transactions-trie inclusion proof (best effort)    │
│        ▼                                                                │
│  bond.Client ──────▶ advertise / anchor to TesseraBond on Base          │
│        ▼                                                                │
│  proof.Proof  (AgenticVerificationProof, signed)                        │
└────────────────────────────────┬────────────────────────────────────────┘
                                 │ delivered over CAP, or streamed to the console
                                 ▼
                         counterparty agent
              verifies with the SDK or in-browser, no trust in Tessera
                (signature, finality, consensus, inclusion, bond)
```

## Components

### `agent/` (Go)

The verification stack. It is transport-agnostic: the same order-handling core
runs behind the live CAP transport, an in-memory loopback for offline demos, and
the HTTP API the console uses.

Entry points (`agent/cmd/`):

| Command | Purpose |
|---|---|
| `console-api` | HTTP + SSE API the web console drives (health, tx preview, live verify stream, anchor, proofs). |
| `provider` | The CAP provider service. `TRANSPORT=croo` runs live; `TRANSPORT=loopback` simulates the full order lifecycle offline. |
| `requester` | A second agent that hires the provider over live CAP (CAP forbids self-hire). |
| `verify` | Verifies a single transaction offline, no CAP and no payment. |
| `watchtower` | A permissionless fraud enforcer: detects a fraudulent anchor and challenges it on-chain. |

Internal packages (`agent/internal/`):

| Package | Responsibility |
|---|---|
| `consensus` | Fans out to the source pool, collects votes, computes the dynamic quorum and finality, streams per-source reports. |
| `source` | `SourceAdapter` implementations: keyless Base RPC sources, a Blockscout explorer source, an optional Basescan source. All abstain-safe. |
| `verify` | Applies the caller's claim (block, sender, status, optional event and log index) to the consensus-agreed facts. |
| `sign` | Produces the EIP-191 attestation over the canonical proof. |
| `proof` | The AVP struct, the canonicalizers (`CanonicalBytes`, `CanonicalCoreBytes`), and the builders. |
| `merkleproof` | Builds and verifies the transactions-trie inclusion proof from raw block bytes. |
| `bond` | Go binding for `TesseraBond`: anchor, challenge, read canonical hash. |
| `handler` | The order-handling core (`AssembleProof`), shared by the CAP handler and the HTTP API. |
| `api` | The HTTP + SSE server for the console. |
| `app` | Central construction of the engine, verifier, signer, bond, and merkle prover from config. |
| `config` | Environment configuration; secrets load from `.env`, never from code. |
| `cap` | Transport abstraction: the live CROO transport and the loopback harness. |

### `contracts/` (Solidity, Foundry)

`TesseraBond.sol` is a self-contained honesty-bond contract on Base. It holds a
standing USDC bond, lets the oracle anchor a per-proof earmark, and lets anyone
challenge a fraudulent anchor. Challenges are resolved trustlessly on-chain (see
[On-chain bond and slashing](#on-chain-bond-and-slashing)). The suite covers the
happy path, false-proof slashing, window expiry, double-slash prevention, and
reentrancy.

### `web/` (Next.js, TypeScript)

`app/page.tsx` is the landing page. `app/console/page.tsx` is the operator
console: a four-stage pipeline (Compose, Consensus, Proof, Handoff) plus a proof
map. It drives the agent through the HTTP API, streams the 11 sources live over
server-sent events, verifies the signature in-browser with `lib/avp.ts`, and
anchors proofs on-chain. `lib/useNarrow.ts` drives the responsive layout.

### `sdk/` (JS + Go)

Standalone verifiers for the AVP format. `sdk/js` is the published npm package
`@olanuel-tessera/avp` (signature verification plus an `/inclusion` entry point
for the merkle proof). `sdk/go` is the Go module. Both are tested against the same
real oracle-signed proof.

### `schema/`

The AgenticVerificationProof v1 specification: a formal JSON Schema, the field
semantics, the RFC 8785 canonicalization and EIP-191 signing scheme, and a short
"verify a proof" snippet.

## The verification pipeline

A single request flows through the stack as follows.

1. **Compose.** The caller submits `{ chainId, address, txHash, blockNumber }`,
   with optional `eventSignature` and `logIndex`. In the console, pasting a tx
   hash calls `GET /api/tx` to auto-fill the contract, block, sender, and status.
2. **Consensus.** `consensus.Engine` queries all sources concurrently. Each
   source that answers in time votes with its view of `(blockNumber, blockHash,
   status)`; a source that errors or times out abstains. Sources are grouped by
   agreement, and the winning group must meet a dynamic quorum: `ceil(fraction ×
   responders)`, default fraction 9/11, never below a `minResponders` floor. If
   too few sources answer, Tessera refuses to sign.
3. **Finality gate.** A quorum of head-reporting sources must confirm the block
   is at or under the chain's `finalized` tag. A consensus-valid but not-yet-final
   transaction yields a signed `verified: false` with a reason, protecting
   anything that later relies on the proof.
4. **Verify the claim.** `verify.Verifier` checks the caller's claim against the
   agreed facts: the transaction is in the claimed block, the receipt status is
   success, the address is involved (sender, recipient, or a log emitter), and if
   given, a matching event log exists at the claimed index.
5. **Sign.** `sign.Signer` serializes the proof to its RFC 8785 canonical form
   (with `attestation` removed) and signs those bytes with EIP-191.
6. **Inclusion proof.** `merkleproof` fetches the block's raw bytes and builds a
   transactions-trie inclusion proof for the transaction. This is best effort: if
   the source cannot provide raw block data, `merkleProof` is left null and the
   proof is still valid.
7. **Bond.** When bonding is enabled, the standing bond is advertised on every
   verified proof. High-value proofs can additionally be anchored on-chain, which
   earmarks part of the bond against the proof id.
8. **Handoff.** The signed proof is delivered: over CAP to the hiring agent, or
   over SSE to the console. On anchor, the server re-signs the proof so the
   updated bond fields remain covered by the signature.

## The AVP object and signing scheme

The proof is a flat JSON object (full field table in [schema/](../schema)). Two
canonical forms matter:

- **`CanonicalBytes`** is the proof with `attestation` removed, serialized as RFC
  8785 canonical JSON. This is the exact preimage the oracle signs with EIP-191
  and that every verifier recomputes. Byte-exactness is load-bearing: producer
  and consumer must agree on it or signatures will not validate. In particular,
  the canonicalizer does not HTML-escape `<`, `>`, or `&`, matching RFC 8785.
- **`CanonicalCoreBytes`** removes both `attestation` and `bond`. Its keccak256 is
  the `proofId`. The core identity must not change when a proof is bonded, and the
  bond field cannot contain a hash of itself. The signature still covers `bond`,
  so the bond claim is signed.

A valid signature vouches for every field, including a `verified: false` verdict.

## Trust model

Verification comes in levels, from cheapest to fully trustless:

1. **Signature.** Recompute the canonical bytes and recover the EIP-191 signer.
   If it matches `attestation.signer`, the oracle vouches for every field. This
   needs no chain access and is what the console and both SDKs do by default.
2. **Consensus and finality.** The proof records how many independent sources
   agreed and that the block was finalized. This is oracle-attested, covered by
   the signature.
3. **Inclusion, oracle-vouched.** The signature covers `merkleProof`, so checking
   that the proof nodes resolve under `transactionsRoot` to a leaf hashing to the
   txHash proves the oracle attests the transaction's inclusion.
4. **Inclusion, fully trustless.** Additionally fetch the block header and confirm
   `header.transactionsRoot == merkleProof.transactionsRoot`. Now inclusion is
   proven with no trust in the oracle at all.
5. **Economic.** The on-chain bond makes a false proof slashable by anyone. Trust
   is backstopped by money, not reputation.

## On-chain bond and slashing

`TesseraBond` holds a standing USDC bond. When a proof is anchored, the oracle
earmarks part of the bond against its `proofId`, recording the claimed block
number and block hash on-chain within a challenge window.

Anyone may then call `challenge(proofId)`. The contract reads the block's
**canonical** hash on-chain: via **EIP-2935** (the historical block-hash contract
at `0x0000F90827F1C53a10cb7A02335B175320002935`, which serves canonical hashes
about 4.5 hours back on Base) with a `blockhash()` fallback for very recent
blocks. If the anchored proof's claimed hash differs from the canonical hash, the
stake is slashed to the challenger. There is no oracle and no arbiter; the
contract is the sole arbiter, deterministically.

Refund and withdraw paths are non-blockable (a malicious hook must never be able
to lock funds), double-slash is prevented, and reentrancy is guarded.
`cmd/watchtower` is a permissionless enforcer that independently checks the
canonical hash and challenges fraud.

This was proven on Base mainnet: a deliberately fraudulent anchor was slashed by
a real profit-seeking searcher in the very next block, about two seconds, before
Tessera's own challenger could act. A lie costs money and is punished by anyone
watching, in seconds. An honest oracle, whose proofs never carry a wrong hash, is
safe.

## Deployment topology

```
GitHub (Manuel-dev01/tessera)
   │  push to main
   ├──▶ Vercel (native Git)     ──▶ web app        https://tessera-console.vercel.app
   ├──▶ GitHub Action           ──▶ Railway        https://tessera-api-production-...railway.app  (Go console-api)
   └──▶ GitHub Action (CI)      ──▶ go test, sdk tests, forge test   (every push and PR)

   push a sdk/js/v* tag ──▶ GitHub Action ──▶ npm publish @olanuel-tessera/avp

Base mainnet:  TesseraBond  0x69D095fb49bcE5735d48710Eb8dD6F94aD72fF85
```

- **Web** deploys on Vercel through its native Git integration; root directory
  `web`.
- **API** deploys on Railway from a Dockerfile via a GitHub Action; the container
  binds the injected `$PORT`.
- **CI** runs the Go build, vet, and tests, both SDK test suites, and the Foundry
  suite on every push and pull request.
- **Releases** publish the npm SDK when a `sdk/js/v*` tag is pushed.

## Technology choices

- **Go** for the verification stack: strong concurrency for the source fan-out,
  the mature go-ethereum libraries for RPC, ABI binding, EIP-191 signing, and the
  Merkle-Patricia trie used by the inclusion proof.
- **Base** as the settlement chain: low fees for the bond and anchor transactions,
  and CAP sponsors gas for the order flow.
- **Foundry** for the contract: fast tests, fork tests against real Base state,
  and a self-contained `TesseraBond` that depends only on `forge-std`.
- **Next.js** for the console: server-sent events for the live source stream, and
  `ethers` plus an RFC 8785 canonicalizer to verify signatures in the browser.
- **An open schema with two-language verifiers** so the proof is verifiable in any
  stack, which is what turns a format into infrastructure rather than a private
  detail.
