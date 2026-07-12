# Tessera — demo runbook

Five ways to see Tessera work, from a zero-setup one-liner to the full live
agent-to-agent hire. Pick by how much you want to stand up.

| # | Demo | Needs | Shows |
|---|---|---|---|
| A | One-command end-to-end | nothing (hits the hosted API) | full stack + independent SDK verify |
| B | Self-contained CAP round-trip | Go | the CAP order lifecycle, offline |
| C | Live agent-to-agent hire | CROO agent + USDC | a real paid hire over CAP |
| D | Verify a proof yourself | Go or Node | the open standard is real |
| E | Honesty bond + slashing | Foundry (free fork) | trustless fraud slashing |

---

## A. One-command end-to-end (the video centerpiece)

No credentials. Drives the **live deployed oracle** through a real verification,
then verifies the returned proof independently with the `@tessera/avp` SDK.

```bash
./demo/demo.sh                 # auto-picks a finalized Base tx
./demo/demo.sh 0x<base-tx>     # or verify a specific one
```

It prints: the oracle health (11 sources, bond), the signed
`AgenticVerificationProof` (consensus, finality, **transactions-trie merkleProof**,
bond, EIP-191 signature), and a final independent check —

```
   signature : VALID  (recovered 0xcf67…4247)
   inclusion : VALID
```

— the signature recovered in-process and the tx's inclusion proven against the
block's transactions root. That last block is the whole thesis: a counterparty
verifies the fact **without trusting Tessera**.

## B. Self-contained CAP round-trip (offline, no credentials)

The provider is transport-agnostic; a `loopback` transport simulates the full CAP
order lifecycle in-memory while running the *real* verifier against Base.

```bash
cp .env.example .env
cd agent && TRANSPORT=loopback go run ./cmd/provider
```

You'll see `order_negotiation_created → AcceptNegotiation → order_paid →
DeliverOrder → order_completed` and a real, signed AVP — the exact `OrderHandler`
the live transport uses.

## C. Live agent-to-agent hire (the real thing)

A second agent hires Tessera over live CAP and receives the signed proof; CAP
locks and settles the USDC. Requires a registered CROO agent and a second
(buyer) agent — CAP forbids an agent hiring its own service.

```bash
# terminal 1 — Tessera online as a provider
cd agent && TRANSPORT=croo go run ./cmd/provider
# terminal 2 — a different agent hires it
cd agent && go run ./cmd/requester -tx 0x<base-tx> -block <n> -addr 0x<involved-addr>
```

`.env` needs `CROO_SDK_KEY` (provider), `REQUESTER_SDK_KEY` (buyer), `SERVICE_ID`,
`TRANSPORT=croo`. The order clears in ~60–90s and writes a reputation (PTS) update
to the DID. Cost: the service price (0.005 USDC); gas is sponsored by CROO.

## D. Verify a proof yourself (the open standard)

Any agent can verify an AVP with nothing but the proof JSON. See [`sdk/`](sdk/).

```ts
import { verifyAVP } from "@tessera/avp";
import { verifyInclusion } from "@tessera/avp/inclusion";
const { ok, signer } = verifyAVP(proof);         // EIP-191 signer recovery
const incl = await verifyInclusion(proof);        // transactions-trie inclusion
```

```go
import avp "github.com/Manuel-dev01/tessera/sdk/go"
r, _ := avp.Verify(proofJSON, true)
ok, _ := avp.VerifyProofInclusion(proofJSON, nil)
```

Or verify a single tx offline, no CAP, no USDC:

```bash
cd agent && go run ./cmd/verify -tx 0x<base-tx> -block <n> -addr 0x<involved-addr>
```

## E. Honesty bond + trustless slashing

Every verified proof advertises a standing USDC bond in
[`TesseraBond`](contracts/src/TesseraBond.sol); high-value proofs are anchored
on-chain and become permissionlessly slashable if the claimed blockHash is wrong.

```bash
cd contracts && forge test                       # full suite incl. slash-a-liar
# free end-to-end vs real Base state:
anvil --fork-url https://mainnet.base.org &
# deploy MockUSDC + TesseraBond, anchor a WRONG hash, then:
cd agent && BOND_CONTRACT=<addr> BASE_RPC_URLS=http://127.0.0.1:8545 \
  go run ./cmd/watchtower -proofId <id>          # detects fraud -> challenge() -> slash
```

> Live proof of this working: on Base mainnet a real MEV searcher slashed a
> deliberately-fraudulent anchor **in the next block (~2s)**, pocketing the stake
> before our own challenger — fraud is punished by permissionless profit-seekers
> in seconds. (This is why an honest oracle's proofs, which never carry a wrong
> hash, are safe.)

---

## Suggested 3-minute video arc

1. **Hook** — the problem: agents can't trust each other's on-chain claims.
2. **Demo A** on the hosted console (`/console`) *and* `./demo/demo.sh` — paste a
   txHash, watch 11 sources attest live, get a signed proof.
3. **The independent verify** — `signature: VALID / inclusion: VALID`. No trust in
   Tessera.
4. **The bond** — show the Base slashing tx; fraud dies in one block.
5. **The standard** — `npm install @tessera/avp`; it's infrastructure, not an app.
