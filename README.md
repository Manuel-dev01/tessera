# Tessera — the fact layer for the agent economy

[![CI](https://github.com/Manuel-dev01/tessera/actions/workflows/ci.yml/badge.svg)](https://github.com/Manuel-dev01/tessera/actions/workflows/ci.yml)

Tessera is an agent-native on-chain data verification oracle. It is a paid, callable
**CAP Provider** on **Base**: other agents hire it to verify that a specific on-chain
event occurred, and it returns a consensus-backed, cryptographically signed
**`AgenticVerificationProof v1`** (AVP) — a signed proof-half one agent hands another
as trustless evidence that an on-chain fact is true.

> A *tessera hospitalis* was a token split between two parties in the Roman world —
> each half proved the bond was real. Tessera issues the digital equivalent.

Built for the CROO Agent Hackathon.

**See it work in one command** (no setup — hits the live oracle, then verifies the
proof independently): [`./demo/demo.sh`](demo/demo.sh). Full runbook in [`DEMO.md`](DEMO.md).

## Status

| Phase | What | State |
|---|---|---|
| 0 | CAP handshake — order round-trip echoing a proof | ✅ proven live on CROO |
| 1 | Real single-source verification + EIP-191 signing | ✅ done (cross-verified in JS) |
| 2 | Multi-source consensus (11 sources, dynamic quorum) + finality gate | ✅ done |
| 3 | `TesseraBond` standing bond + per-proof anchor + trustless (EIP-2935) slashing + watchtower | ✅ done |
| 4 | Landing page + fully-wired operator console (Next.js) | ✅ done |

## The open standard: AgenticVerificationProof v1

The proof format is published as a **proposed open schema for the CROO ecosystem**,
not a Tessera-only format. See [`schema/`](schema/):

- [`agentic-verification-proof-v1.json`](schema/agentic-verification-proof-v1.json) — formal JSON Schema
- [`agentic-verification-proof-v1.md`](schema/agentic-verification-proof-v1.md) — field semantics, EIP-191 signing scheme, "verify a proof in 10 lines"

## Repo layout

```
schema/     AgenticVerificationProof v1 — the open standard
sdk/        Reference verifiers (JS `@olanuel-tessera/avp` + Go) — "verify a proof in 3 lines"
agent/      Go CAP provider + console-api (verification core, consensus, signing, bond client, watchtower)
contracts/  Foundry: TesseraBond.sol + tests (standing bond + EIP-2935 trustless slashing)
web/        Next.js landing page + operator console (fully wired to the agent)
```

## Verify a proof (SDK)

A proof is only useful if a counterparty agent can check it **without trusting
Tessera**. The [`sdk/`](sdk/) ships standalone verifiers that need nothing but the
proof JSON — they recompute the RFC 8785 canonical bytes and recover the EIP-191
signer. Both are tested against the *same real proof signed by the live oracle*.

```ts
// JavaScript / TypeScript — npm install @olanuel-tessera/avp
import { verifyAVP } from "@olanuel-tessera/avp";
const { ok, signer } = verifyAVP(proof);   // ok === true → `signer` vouches for every field
```

```go
// Go — go get github.com/Manuel-dev01/tessera/sdk/go
r, _ := avp.Verify(proofJSON, true)        // r.OK === true → r.Signer vouches for every field
```

## Landing page + operator console (Phase 4)

A Next.js app in [`web/`](web/): a static landing page (`/`) and a **fully-wired operator
console** (`/console`) — no mock data. The console drives the real agent through an HTTP API
([`agent/cmd/console-api`](agent/cmd/console-api/main.go)): paste a Base txHash → watch the **11
real sources attest live over SSE** → a signed AVP with **in-browser EIP-191 verification** →
**anchor the proof on-chain** to TesseraBond → see it orbit the proof map.

```bash
# terminal 1 — the verification API (real consensus + signing + bond)
cd agent && BOND_ENABLED=true BOND_CONTRACT=0x69D095fb49bcE5735d48710Eb8dD6F94aD72fF85 \
  go run ./cmd/console-api          # :8787

# terminal 2 — the web app
cd web && npm install && npm run dev # http://localhost:3000  (/console)
```

API endpoints: `/api/health`, `/api/tx`, `/api/verify/stream` (SSE), `/api/verify`, `/api/anchor`,
`/api/proofs`. The console's signature check recomputes the RFC-8785 canonical form in-browser and
recovers the signer with ethers — the same bytes the Go oracle signed.

## Honesty bond & slashing (Phase 3)

Tessera keeps a **standing USDC bond** in [`TesseraBond`](contracts/src/TesseraBond.sol); every
verified proof advertises it. High-value proofs are additionally **anchored** on-chain, earmarking
part of the bond. Anyone can then `challenge(proofId)`: the contract reads the block's *canonical*
hash on-chain — via **EIP-2935** (history contract, ~4.5h window) with `blockhash()` fallback — and
if the proof's claimed hash differs, the stake is **slashed to the challenger**. Trustless: no
oracle, no arbiter. [`cmd/watchtower`](agent/cmd/watchtower/main.go) is the permissionless enforcer.

```bash
cd contracts && forge test          # full suite incl. slash-a-lying-oracle + reentrancy
# free end-to-end vs real Base state:
anvil --fork-url https://mainnet.base.org &
# deploy MockUSDC + TesseraBond, anchor a wrong hash, then:
cd agent && BOND_CONTRACT=<addr> BASE_RPC_URLS=http://127.0.0.1:8545 \
  go run ./cmd/watchtower -proofId <id>   # detects fraud -> challenge() -> slash
```

## Run the Phase 0 demo (no credentials needed)

The provider is built **transport-agnostic**: an in-memory `loopback` transport
simulates the full CAP order lifecycle so the round-trip is demonstrable without a
CROO dashboard account. The live `croo` transport uses the real CAP Go SDK and is
selected once you register an agent and set `CROO_SDK_KEY`.

```bash
cp .env.example .env          # loopback needs no edits
cd agent
go mod tidy
TRANSPORT=loopback go run ./cmd/provider
```

You should see the simulated lifecycle
(`negotiation_created → accept → order_paid → DeliverOrder → order_completed`) and a
real, signed AVP proof (the loopback now runs the Phase 1 verifier against Base).

### Verify a single tx offline (no CAP, no USDC)

```bash
cd agent
go run ./cmd/verify -tx 0x<base-tx> -block <n> -addr 0x<involved-address>
```

Prints a signed `AgenticVerificationProof`. A mismatched block/address/status yields a
signed `verified:false` proof with a `reason`. The EIP-191 signature is recoverable by
the 10-line JS snippet in [schema/](schema/agentic-verification-proof-v1.md).

### Going live (later)

1. Register the agent + a service at [agent.croo.network](https://agent.croo.network),
   pasting the AVP **input** schema (`{chainId, address, txHash, blockNumber, ...}`) as
   the service Requirements.
2. Put the real `CROO_SDK_KEY` in `.env` and set `TRANSPORT=croo`.
3. `go run ./cmd/provider`, then hire the agent — the order completes with the echo proof.

## Security

`.env` is gitignored from commit 1. Never commit secrets or the oracle private key.
Payment/escrow is handled entirely by CAP (CAPVault) — Tessera builds **no** second
payment escrow. `TesseraBond` (Phase 3) is a separate provider honesty stake.
