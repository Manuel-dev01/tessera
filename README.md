# Tessera

**The fact layer for the agent economy.**

[![CI](https://github.com/Manuel-dev01/tessera/actions/workflows/ci.yml/badge.svg)](https://github.com/Manuel-dev01/tessera/actions/workflows/ci.yml)

Tessera is an agent-native on-chain data verification oracle. It runs as a paid,
callable service on **Base**: another agent hires it to confirm that a specific
on-chain event occurred, and it returns a consensus-backed, cryptographically
signed **AgenticVerificationProof (AVP)**. That proof is the machine-verifiable
receipt one agent hands another as trustless evidence of an on-chain fact.

A *tessera hospitalis* was a token split between two parties in the Roman world,
where each half proved the bond was real. Tessera issues the digital equivalent.

Built for the CROO Agent Hackathon.

## Live

| Surface | URL |
|---|---|
| Operator console | https://tessera-console.vercel.app/console |
| Landing page | https://tessera-console.vercel.app |
| Verification API | https://tessera-api-production-2b6c.up.railway.app |
| npm package | [`@olanuel-tessera/avp`](https://www.npmjs.com/package/@olanuel-tessera/avp) |
| Go package | `github.com/Manuel-dev01/tessera/sdk/go` |

## What it does

A caller submits a transaction to check:

```json
{ "chainId": 8453, "address": "0x...", "txHash": "0x...", "blockNumber": 12345 }
```

Tessera queries eleven independent, method-diverse data sources at once, requires
a supermajority to agree, confirms the block is finalized, and returns a signed
AVP. The proof states whether the fact is true, records the consensus and
finality that back it, carries a transactions-trie inclusion proof, advertises an
on-chain honesty bond, and is signed so any counterparty can verify it without
trusting Tessera. A signed negative (`verified: false`) is a first-class result:
a caller learns as much from a signed "this transaction is not in that block" as
from a signed confirmation.

## Quick start

Two ways to reproduce the system, ordered by setup cost. The first needs
nothing but Node.

### 1. Verify a live proof with the published SDK

```bash
mkdir tessera-check && cd tessera-check && npm init -y >/dev/null
npm install @olanuel-tessera/avp
```

```js
// check.mjs
import { verifyAVP } from "@olanuel-tessera/avp";
import { verifyInclusion } from "@olanuel-tessera/avp/inclusion";

const res = await fetch(
  "https://tessera-api-production-2b6c.up.railway.app/api/verify",
  { method: "POST", headers: { "content-type": "application/json" },
    body: JSON.stringify({ txHash: "0x<a-finalized-base-tx>" }) },
);
const proof = await res.json();

console.log("signature:", verifyAVP(proof).ok);          // recovers the oracle signer
console.log("inclusion:", (await verifyInclusion(proof)).ok); // checks the trie proof
```

`verifyAVP` recomputes the canonical bytes and recovers the EIP-191 signer with no
network access. `verifyInclusion` checks the transactions-trie proof. Both return
`true` for a genuine proof and `false` for any tampered field.

### 2. Run the full stack locally

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

Paste a Base transaction hash into the console and watch the eleven sources
attest over server-sent events, then verify the signature in the browser.

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
  "consensus": { "sources": 11, "responders": 11, "agreed": 9, "quorum": 9, "agreedSources": ["..."] },
  "finality": { "finalized": true, "finalizedBlock": 48515000, "confirmations": 3658 },
  "merkleProof": { "type": "transactionsTrie", "transactionsRoot": "0x...", "txIndex": 168, "key": "0x...", "nodes": ["0x..."], "leaf": "0x..." },
  "attestation": { "signer": "0x...", "scheme": "EIP-191", "signature": "0x..." },
  "bond": { "contract": "0x...", "stakedUSDC": "0.50", "challengeWindowSec": 3600, "proofId": "0x...", "anchored": false, "anchorTx": null },
  "reason": null,
  "issuedAt": 1720000000
}
```

The signature covers every field except `attestation`, serialized as RFC 8785
canonical JSON and signed with EIP-191. A valid signature vouches for the whole
proof, including a `verified: false` verdict.

## How verification works

**Multi-source consensus.** Tessera queries eleven independent sources at once:
ten distinct-operator Base RPC endpoints plus a block-explorer indexer. Each
source that answers in time votes with its view of `(blockNumber, blockHash,
status)`. A source that errors or times out abstains. The signed value is the one
a dynamic quorum of responders agrees on, where quorum is a supermajority of the
sources that actually answered, with an absolute floor. Independence of the
sources is the real security parameter.

**Finality gate.** A block that is merely included can still be reorganized out.
Before signing `verified: true`, a quorum of sources must confirm the block is at
or under the chain's finalized tag. A consensus-valid but not-yet-final
transaction returns a signed `verified: false` with a reason, which protects
anything that later relies on the proof, such as a bond.

**Transactions-trie inclusion proof.** A block header commits to its
transactions through `transactionsRoot`, the root of a Merkle-Patricia trie. When
available, the proof carries a standalone inclusion proof for the transaction,
checkable against that root with no trust in the oracle. It is type-agnostic,
built from raw consensus-encoded transaction bytes, so OP-stack deposit
transactions (type `0x7E`, always index 0 on Base) need no special handling.

## Honesty bond and slashing

Every verified proof advertises a standing USDC bond in
[TesseraBond](contracts/src/TesseraBond.sol), deployed on Base at
`0x69D095fb49bcE5735d48710Eb8dD6F94aD72fF85`. High-value proofs are additionally
anchored on-chain, which earmarks part of the bond. Anyone can then call
`challenge(proofId)`. The contract reads the block's canonical hash on-chain,
using EIP-2935 with a `blockhash` fallback, and if the proof's claimed hash
differs, the stake is slashed to the challenger. There is no oracle and no
arbiter in the loop.

```bash
cd contracts && forge test         # full suite, including slashing a lying oracle
```

On Base mainnet, a deliberately fraudulent anchor was slashed by a real
profit-seeking searcher in the next block, roughly two seconds, before Tessera's
own challenger could act. Fraud is punished by permissionless actors in seconds.
An honest oracle never carries a wrong hash, so its proofs are safe.

## Hire it over CAP

Tessera is a real CAP provider. Payment and escrow are handled entirely by CAP,
which locks the caller's USDC in CAPVault and settles on delivery. The provider
is transport-agnostic, so the full order lifecycle runs offline with no
credentials for development and against live CAP in production.

```bash
# offline: simulate the full CAP order lifecycle with the real verifier
cd agent && TRANSPORT=loopback go run ./cmd/provider

# live: run online, then a second agent hires it (CAP forbids self-hire)
cd agent && TRANSPORT=croo go run ./cmd/provider
cd agent && go run ./cmd/requester -tx 0x<base-tx> -block <n> -addr 0x<involved-addr>
```

## Verify a proof yourself

Reference verifiers ship in [sdk/](sdk/) for JavaScript and Go. Both are
standalone, tested against the same real oracle-signed proof, and prove the
canonical byte form reproduces identically across implementations.

```ts
import { verifyAVP } from "@olanuel-tessera/avp";
import { verifyInclusion } from "@olanuel-tessera/avp/inclusion";
const { ok, signer } = verifyAVP(proof);
const inclusion = await verifyInclusion(proof);
```

```go
import avp "github.com/Manuel-dev01/tessera/sdk/go"
r, _ := avp.Verify(proofJSON, true)
ok, _ := avp.VerifyProofInclusion(proofJSON, nil)
```

## Repo layout

```
schema/     AgenticVerificationProof v1, the open standard
sdk/        Reference verifiers: JS @olanuel-tessera/avp and Go
agent/      Go CAP provider and console API: consensus, signing, bond client, merkle prover, watchtower
contracts/  Foundry project: TesseraBond.sol with the standing bond and EIP-2935 slashing
web/        Next.js landing page and operator console, wired to the agent
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
| 8 | End-to-end demo and runbook | done |

## Deployment and CI

The web app is hosted on Vercel and the verification API on Railway. Both deploy
automatically on push to `main`: Vercel through its native Git integration, and
the API through a GitHub Action. Continuous integration runs the Go tests, the
SDK tests, and the Foundry suite on every push and pull request. The SDK is
published to npm automatically when a `sdk/js/v*` tag is pushed.

## Security

Secrets are never committed. The `.env` file is git-ignored from the first
commit. Payment and escrow are handled entirely by CAP, so Tessera builds no
second payment escrow. TesseraBond is a separate provider honesty stake, not a
payment channel.

## License

MIT.
