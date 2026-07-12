# CLAUDE.md — Tessera Oracle Agent

> Operating manual for Claude Code. Read this fully before writing any code.
> Tessera is an agent-native on-chain data verification oracle: a paid, callable
> CAP Provider that other agents hire to verify an on-chain event and returns a
> consensus-backed, cryptographically signed proof. Built for the CROO Agent
> Hackathon. Submissions close Jul 9.

---

## 0. NAME + ONE-LINER

**Tessera.** A *tessera hospitalis* was a token split between two parties in the
Roman world — each half proved the bond was real. Tessera issues the digital
equivalent: a signed proof-half one agent hands another as trustless evidence
that an on-chain fact is true.

Tagline: **"The fact layer for the agent economy."**

---

## 1. GROUND TRUTH — DO NOT INVENT (verified against official docs)

- Chain: **Base**. Settlement token: **USDC**. Gas for the CAP order flow is
  **sponsored by CROO** — agents do not need ETH to transact via CAP.
- CAP Go SDK: `github.com/CROO-Network/go-sdk`.
- Env vars (never hardcode; load from .env, never commit .env):
CROO_API_URL="https://api.croo.network"
CROO_WS_URL="wss://api.croo.network/ws"
CROO_SDK_KEY="croo_sk_..."
- CAP order lifecycle — this is the ONLY payment/escrow path. CAP locks the
  requester's USDC in CAPVault automatically. **Do NOT build a second payment
  escrow.** The bond contract (§6) is a SEPARATE, provider-side honesty stake.
Requester: NegotiateOrder → (WS: order_created) → PayOrder   [USDC locked]
Provider:  AcceptNegotiation → (WS: order_paid) → DeliverOrder
Requester: (WS: order_completed) → GetDelivery
- Agent registration, AA wallet, and DID are created in the **dashboard**
  (agent.croo.network), NOT in code. Code consumes the SDK_KEY only.
- A cleared order automatically writes a PTS (reputation) update to the DID.
- Stack layering: **ERC-8004** = identity/reputation; **ERC-8183** = commerce/
  escrow standard (reference impl `github.com/erc-8183/base-contracts`, Foundry).

> RULE: If you are unsure of a real SDK method name or signature, STOP and read
> the provider/requester examples in `github.com/CROO-Network/go-sdk` before
> writing. Never fabricate an SDK call. A fabricated method poisons the build.

---

## 2. STACK (matches the maintainer's toolchain — do not substitute)

| Layer | Tech |
|---|---|
| Provider service + orchestration | Go 1.22+ (CAP Go SDK) |
| Honesty-bond / slashing contract | Solidity + Foundry (on Base) |
| Demo dashboard | Next.js |

Monorepo, one repo:
/agent      Go provider service, verification core, consensus, signing
/contracts  Foundry project: TesseraBond.sol + tests
/web        Next.js demo dashboard
/schema     AgenticVerificationProof v1 (the open standard)

---

## 3. WHAT TESSERA DOES

**Service input schema** (the CAP Service "Requirements"):
```json
{ "chainId": 8453, "address": "0x...", "txHash": "0x...", "blockNumber": 12345,
  "eventSignature": "Transfer(address,address,uint256)", "logIndex": 3 }
```
`eventSignature` and `logIndex` are optional.

**Output** = `AgenticVerificationProof v1` (canonical schema lives in /schema):
```json
{
  "schemaVersion": "avp/1.0",
  "verified": true,
  "chainId": 8453,
  "blockNumber": 12345,
  "blockHash": "0x...",
  "txHash": "0x...",
  "txIndex": 7,
  "consensus": { "sources": 3, "agreed": 3, "quorum": 2 },
  "merkleProof": null,
  "attestation": { "signer": "0x...", "scheme": "EIP-191", "signature": "0x..." },
  "bond": { "contract": "0x...", "stakedUSDC": "0.50", "challengeWindowSec": 3600 },
  "reason": null,
  "issuedAt": 1720000000
}
```
On failure: `verified:false`, `reason` populated (e.g. `"tx not found in claimed block"`, `"quorum not reached: 1/3 sources agreed"`), and the proof is still signed (a signed *negative* is as useful to a caller as a signed positive).

---

## 4. BUILD PHASES — ship and verify each before the next

**Phase 0 — CAP handshake.** Monorepo init. Go provider using the real SDK
provider example. Agent goes Online, receives → accepts → delivers a trivial
echo order returning `{"verified": true}`. MUST round-trip before anything else.

**Phase 1 — Verification core (single source).** RPC client hits Base:
`eth_getTransactionReceipt` + `eth_getBlockByNumber`. Verify tx exists, is in
the claimed block, matches sender/status, and (if given) a matching log exists
at logIndex. Sign canonical JSON (RFC 8785-style) with the oracle key via
EIP-191. Deliverable: hire with a REAL Base txHash → correct signed proof.

**Phase 2 — Multi-source consensus.** `SourceAdapter` interface; THREE
implementations over three independent Base RPC providers. Concurrent query,
require **2-of-3 quorum** on blockHash + receipt status before signing. Config
flag `CONSENSUS_MODE=internal|croo`: default `internal`; `croo` swaps sources
for real CROO sub-agents ONLY if such a service is live in the store. Never
hard-depend on external agents existing. Deliverable: kill one source → still
verifies at 2/3; kill two → refuses to sign with a reason.

**Phase 3 — Honesty bond + slashing.** See §6.

**Phase 4 — Demo dashboard.** One Next.js screen: paste Base txHash → "Hire
Tessera ($0.005 USDC)" → live CAP lifecycle → render the proof, the 2-of-3
consensus, the EIP-191 signature (verified client-side), and bond/challenge
status. Copyable JSON. This is the demo-video centerpiece.

---

## 5. THE STANDARD (innovation differentiator — ship in-repo)

`/schema/agentic-verification-proof-v1.md` + `.json`:
- Formal JSON Schema for AVP v1, field semantics, signing scheme (EIP-191 over
  canonical JSON), and a "verify a Tessera proof in 10 lines" snippet.
- README frames it as a **proposed open schema for the CROO ecosystem**, not a
  Tessera-only format. This is what reads as a foundational builder, not a
  one-off app.

---

## 6. TESSERABOND.SOL (Foundry, Base, USDC)

- Oracle stakes USDC per proof (or a rolling bond); funds locked.
- Each issued proof commits a hash on-chain (referenceable by proofId).
- Challenger submits a fraud proof within `challengeWindow`; contract verifies
  counter-evidence **deterministically where possible** (e.g. claimed blockHash
  ≠ canonical) → slashes stake to challenger.
- Refund/withdraw path after the window is **non-blockable** (mirror ERC-8183's
  rule that refunds must never be gate-able by a hook — a malicious hook must
  not be able to lock funds forever).
- Study `github.com/erc-8183/base-contracts` for the Job/hook mental model, but
  keep TesseraBond minimal and self-contained. You do NOT need full 8183.
- Foundry tests: happy path, false-proof slashing, window expiry, double-slash
  prevention, reentrancy. Aim for high coverage.

---

## 7. KNOWN TRAPS (learned the hard way — don't rediscover these)

- **Tx-within-block Merkle proof is a transactions TRIE (RLP-encoded MPT), not a
  plain Merkle tree.** Verifying it is real work. `merkleProof` stays `null` in
  the MVP; it's a Phase-5 stretch. Ship consensus + signature first.
- **Consensus via store sub-agents can break at demo time** if no RPC-checker
  service is live. Default to internal RPC sources; the `croo` mode is a bonus,
  never the critical path.
- **Don't duplicate CAP's escrow.** CAP already locks the fee. TesseraBond is a
  separate provider honesty stake, not a payment channel.
- **Canonical JSON matters.** Sign the same canonical byte form the verifier
  recomputes, or signatures won't validate. Nail this in Phase 1.

---

## 8. GUARDRAILS

- Every phase runs and is demonstrable before the next begins.
- Correctness and a clean vertical slice beat feature breadth. A working
  Phase 0–2 + bond beats a broken everything.
- Never commit secrets. `.env` in `.gitignore` from commit 1.
- Never fabricate SDK method names (see §1).

---

## 9. FIRST ACTIONS FOR CLAUDE CODE

1. Read the provider + requester examples in `github.com/CROO-Network/go-sdk`.
   Summarize the real method signatures back before writing code.
2. Propose the exact monorepo file tree.
3. Build Phase 0. Report the round-trip result. Do NOT write Phase 1+ until
   Phase 0 round-trips.