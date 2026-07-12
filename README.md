# Tessera

**The fact layer for the agent economy.**

[![CI](https://github.com/Manuel-dev01/tessera/actions/workflows/ci.yml/badge.svg)](https://github.com/Manuel-dev01/tessera/actions/workflows/ci.yml)

**▶ Watch the demo:** https://www.youtube.com/watch?v=2f5mS_z1aR8

Tessera is an agent-native on-chain data verification oracle. It runs as a paid,
callable service on **Base**: one autonomous agent hires it to confirm that a
specific on-chain event occurred, and it returns a consensus-backed,
cryptographically signed **AgenticVerificationProof (AVP)**. That proof is the
machine-verifiable receipt one agent hands another as trustless evidence of an
on-chain fact. The counterparty verifies it locally, in a few lines of code,
with no call back to Tessera and no trust in Tessera required.

A *tessera hospitalis* was a token split between two parties in the Roman world,
where each half proved the bond was real. Tessera issues the digital equivalent.

Built for the CROO Agent Hackathon.

## Contents

- [Live](#live)
- [Try it in two minutes](#try-it-in-two-minutes)
- [How it works](#how-it-works)
- [Architecture](#architecture)
- [The AgenticVerificationProof](#the-agenticverificationproof)
- [Verify a proof yourself](#verify-a-proof-yourself)
- [Run it locally](#run-it-locally)
- [Project structure](#project-structure)
- [Build status](#build-status)
- [Deployment and CI](#deployment-and-ci)
- [Security](#security)

## Live

| Surface | URL |
|---|---|
| Demo video | https://www.youtube.com/watch?v=2f5mS_z1aR8 |
| Operator console | https://tessera-console.vercel.app/console |
| Landing page | https://tessera-console.vercel.app |
| Verification API | https://tessera-api-production-2b6c.up.railway.app |
| npm package | [`@olanuel-tessera/avp`](https://www.npmjs.com/package/@olanuel-tessera/avp) |
| Go package | `github.com/Manuel-dev01/tessera/sdk/go` |
| TesseraBond (Base) | [`0x69D095fb49bcE5735d48710Eb8dD6F94aD72fF85`](https://basescan.org/address/0x69D095fb49bcE5735d48710Eb8dD6F94aD72fF85) |

## Try it in two minutes

Three ways, from zero setup to a full local stack. Start with A.

### A. Drive the live console (no setup)

1. Open the console: **https://tessera-console.vercel.app/console**
2. In **TX HASH**, paste any recent successful Base transaction. If you need one,
   grab a Transfer from [Basescan](https://basescan.org), or run
   `./demo/pick-tx.sh` to print a ready one (finalized, successful, and recent
   enough to carry a merkle proof).
3. Click **run consensus**. Watch the 11 independent sources attest live and the
   quorum resolve.
4. Click **view proof**. You now have a signed AVP. Look for the green
   **signature verified in-browser** line: your browser recovered the oracle's
   address from the signature locally, with no call back to Tessera.
5. Optional: **hand off** the proof to anchor it on-chain against the honesty
   bond, then open the **proof map**.

> The console verifies real Base transactions against the live oracle. A proof
> is only marked verified when a supermajority of sources agree and the block is
> finalized, so freshly-mined (not-yet-final) transactions return a signed
> `verified: false`, which is itself a valid result.

### B. One command, end to end

Drives the live oracle through a full verification and then verifies the returned
proof independently with the SDK. No credentials.

```bash
git clone https://github.com/Manuel-dev01/tessera && cd tessera
./demo/demo.sh                 # auto-picks a finalized Base transaction
./demo/demo.sh 0x<base-tx>     # or verify a specific one
```

It prints the oracle health, the signed proof (consensus, finality, inclusion
proof, bond, signature), and a final independent check that reads
`signature: VALID` and `inclusion: VALID`.

### C. Verify a proof with the published SDK

```bash
npm install @olanuel-tessera/avp
```

```js
import { verifyAVP } from "@olanuel-tessera/avp";
import { verifyInclusion } from "@olanuel-tessera/avp/inclusion";

const { ok, signer } = verifyAVP(proof);        // recovers the EIP-191 signer, no network
const inclusion = await verifyInclusion(proof);  // checks the transactions-trie proof
```

## How it works

A caller submits a transaction to check:

```json
{ "chainId": 8453, "address": "0x...", "txHash": "0x...", "blockNumber": 12345 }
```

Tessera then does five things, each in service of one principle: **verification
must not require trusting the verifier.**

- **Multi-source consensus.** It queries 11 independent, method-diverse sources
  at once (10 distinct-operator Base RPC endpoints plus a block-explorer indexer)
  and signs only when a dynamic supermajority agrees on the block and status. No
  single node is a point of trust.
- **Finality gate.** It signs `verified: true` only after the block is finalized,
  so a reorg can never quietly undo a signed fact.
- **EIP-191 signature.** The proof is signed over its RFC 8785 canonical form.
  Any verifier recomputes the same bytes and recovers the signer locally.
- **Transactions-trie inclusion proof.** The proof carries a Merkle-Patricia
  inclusion proof, standalone cryptographic evidence that the transaction sits at
  its index in the block, checkable against the block header itself.
- **On-chain honesty bond.** High-value proofs are anchored to a USDC stake in
  `TesseraBond`. Anyone can challenge a fraudulent proof and the stake is slashed
  to them, with no oracle and no arbiter.

A signed negative (`verified: false`) is a first-class result: a caller learns as
much from a signed "this transaction is not in that block" as from a confirmation.

## Architecture

```
   caller agent
        │  hires (over CAP)  /  POST verify
        ▼
┌─────────────────────── Tessera provider (Go) ───────────────────────┐
│                                                                      │
│  consensus engine ──▶ 11 sources ──▶ Base RPCs + explorer            │
│        │  agreed (blockHash, status)                                 │
│        ▼                                                             │
│  finality gate ─────▶ requires block ≤ finalized                     │
│        ▼                                                             │
│  verify + sign ─────▶ EIP-191 over RFC 8785 canonical JSON           │
│        ▼                                                             │
│  merkle prover ─────▶ transactions-trie inclusion proof              │
│        ▼                                                             │
│  bond client ───────▶ anchor to TesseraBond (Base)                   │
│        ▼                                                             │
│  AgenticVerificationProof (signed)                                   │
└──────────────────────────────┬───────────────────────────────────────┘
                               │ hand off
                               ▼
                       counterparty agent
                  verifies locally (SDK / in-browser),
                  no trust in Tessera
```

The verification stack is a Go service; the honesty bond is a Solidity contract
on Base; the operator console is a Next.js app; the proof format ships as an open
schema with reference verifiers in JS and Go. For the full component breakdown,
the request lifecycle, the trust model, the canonicalization and signing scheme,
and the on-chain slashing mechanism, see **[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)**.

## The AgenticVerificationProof

The proof format is published as a proposed open schema for the CROO ecosystem,
not a Tessera-only format. See [schema/](schema/) for the formal JSON Schema, the
field semantics, and the signing scheme.

```json
{
  "schemaVersion": "avp/1.0",
  "verified": true,
  "chainId": 8453,
  "blockNumber": 48511342,
  "blockHash": "0x...",
  "txHash": "0x...",
  "txIndex": 168,
  "consensus": { "sources": 11, "responders": 11, "agreed": 11, "quorum": 9, "agreedSources": ["..."] },
  "finality": { "finalized": true, "finalizedBlock": 48515000, "confirmations": 3658 },
  "merkleProof": { "type": "transactionsTrie", "transactionsRoot": "0x...", "txIndex": 168, "key": "0x...", "nodes": ["0x..."], "leaf": "0x..." },
  "attestation": { "signer": "0x...", "scheme": "EIP-191", "signature": "0x..." },
  "bond": { "contract": "0x...", "stakedUSDC": "0.50", "challengeWindowSec": 3600, "proofId": "0x...", "anchored": false, "anchorTx": null },
  "reason": null,
  "issuedAt": 1720000000
}
```

The signature covers every field except `attestation`. A valid signature vouches
for the whole proof, including a `verified: false` verdict.

## Verify a proof yourself

Reference verifiers ship in [sdk/](sdk/) for JavaScript and Go. Both are
standalone and are tested against the same real oracle-signed proof, proving the
canonical byte form reproduces identically across implementations.

```ts
// npm install @olanuel-tessera/avp
import { verifyAVP } from "@olanuel-tessera/avp";
import { verifyInclusion } from "@olanuel-tessera/avp/inclusion";
const { ok, signer } = verifyAVP(proof);
const inclusion = await verifyInclusion(proof);
```

```go
// go get github.com/Manuel-dev01/tessera/sdk/go
import avp "github.com/Manuel-dev01/tessera/sdk/go"
r, _ := avp.Verify(proofJSON, true)
ok, _ := avp.VerifyProofInclusion(proofJSON, nil)
```

## Run it locally

Two processes: the Go verification API and the Next.js web app.

```bash
# terminal 1: the verification API (real consensus, signing, bond)
cd agent
BOND_ENABLED=true BOND_CONTRACT=0x69D095fb49bcE5735d48710Eb8dD6F94aD72fF85 \
  go run ./cmd/console-api            # serves http://localhost:8787

# terminal 2: the console and landing page
cd web
npm install && npm run dev            # serves http://localhost:3000/console
```

Copy `.env.example` to `.env` and fill in the oracle key for signing. The
contracts test suite runs with `cd contracts && forge test`.

## Project structure

```
schema/     AgenticVerificationProof v1, the open standard (JSON Schema + spec)
sdk/        Reference verifiers: JS @olanuel-tessera/avp and Go
agent/      Go CAP provider + console API: consensus, signing, bond client, merkle prover, watchtower
contracts/  Foundry project: TesseraBond.sol with the standing bond + EIP-2935 slashing
web/        Next.js landing page and operator console, wired to the agent
demo/       One-command end-to-end demo script
docs/       Architecture documentation
```

## Build status

| Phase | Scope | State |
|---|---|---|
| 0 | CAP handshake, order round-trip | done, proven live on CROO |
| 1 | Single-source verification and EIP-191 signing | done |
| 2 | Multi-source consensus and finality gate | done |
| 3 | Standing bond, on-chain anchoring, trustless slashing | done, slashed live on Base |
| 4 | Landing page and operator console | done, deployed |
| 5 | Open standard and reference verifier SDKs | done, published to npm and Go |
| 6 | Transactions-trie inclusion proof | done, live in every proof |
| 7 | Continuous integration across all stacks | done, green |
| 8 | End-to-end demo | done |

## Deployment and CI

The web app is hosted on Vercel and the verification API on Railway. Both deploy
automatically on push to `main`: Vercel through its native Git integration, and
the API through a GitHub Action. Continuous integration runs the Go tests, the
SDK tests, and the Foundry suite on every push and pull request. The SDK is
published to npm automatically when a `sdk/js/v*` tag is pushed. See
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md#deployment-topology) for the topology.

## Security

Secrets are never committed. The `.env` file is git-ignored from the first
commit. Payment and escrow are handled entirely by CAP, so Tessera builds no
second payment escrow. TesseraBond is a separate provider honesty stake, not a
payment channel.

## License

MIT.
