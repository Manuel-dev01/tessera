# Tessera demo — shot-by-shot script

A ~3 minute run. Every shot lists the screen, the exact action, and the
voiceover. Money shots are marked with a star. There is a 60 second cut at the
bottom.

Links:
- Console: https://tessera-console.vercel.app/console
- Landing: https://tessera-console.vercel.app
- Bond contract on Base: 0x69D095fb49bcE5735d48710Eb8dD6F94aD72fF85

## Pre-flight (do this before you hit record)

1. **Pick a demo transaction.** You need a Base tx that is finalized (older than
   ~20 min) and successful, and recent enough for the merkle proof (younger than
   ~4 hours). Run this to print one that is ready:
   ```bash
   ./demo/pick-tx.sh     # prints a finalized, successful, merkle-ready tx hash
   ```
   Copy the hash. Verify it once in the console first so the first take is warm.
2. **Open three things:** the console (`/console`), a terminal, and a Basescan
   tab for the bond contract (address above).
3. **Warm the API.** Load `/console` once so the health call and first stream are
   cached. Screen at 1440 wide for desktop, or 390 for a mobile cut.
4. **Know your fallback.** It is a live oracle. If a stream is slow, you have the
   terminal one-liner (`./demo/demo.sh`) as a backup that proves the same thing.

---

## Shot 1 — The problem (0:00 to 0:18)

- **Screen:** Landing page hero (`/`). Let the network animation move.
- **Action:** Slow scroll from the hero to the "A proof-half travels between two
  agents" diagram.
- **VO:** "Agents are starting to transact with each other on-chain. But when one
  agent claims a payment landed or an event fired, the other agent has no way to
  trust that claim without doing all the work itself. Tessera is the fact layer
  that fixes this. You hire it to verify an on-chain event, and it hands back a
  signed proof any other agent can check."

## Shot 2 — Hire the oracle (0:18 to 0:32)

- **Screen:** Console, Compose stage (`/console`).
- **Action:** Paste the demo tx hash into TX HASH. The contract, block, sender,
  and status chips auto-fill. Click **run consensus**.
- **VO:** "Here is the console. I paste one Base transaction hash. Tessera pulls
  the on-chain facts, and now it goes to consensus."

## Shot 3 — Consensus, live (0:32 to 0:52) *

- **Screen:** Consensus stage. The verifier sphere; the attestation log filling
  in; the quorum bar.
- **Action:** Let it run. Sources tick from "polling" to "agreed" one by one. The
  header climbs to 11 of 11, quorum 9, finalized.
- **VO:** "This is real. Eleven independent data sources, different operators,
  different methods, each checking the chain and voting. Tessera only signs when
  a supermajority agrees on the block and status, and only after the block is
  finalized so a reorg can never undo it." Click **view proof**.

## Shot 4 — The proof, verified without trusting Tessera (0:52 to 1:22) *

- **Screen:** Proof stage. The AVP fields, then the attestation line.
- **Action:** Point at `consensus`, `finality`, and the signature. Then point at
  the green line: **signature verified in-browser**, and the recovered signer.
- **VO:** "Out comes an AgenticVerificationProof. Consensus, finality, an
  EIP-191 signature. And here is the whole point. My browser just recomputed the
  canonical proof and recovered the signer's address from the signature, locally,
  with no call back to Tessera. If this checks out, the oracle vouches for every
  field, and I did not have to trust the oracle to know that."

## Shot 5 — The inclusion proof (1:22 to 1:40)

- **Screen:** Proof stage. Click **copy JSON** or **download**, then show the
  JSON (in an editor or the downloaded file) and scroll to `merkleProof`.
- **VO:** "It goes deeper. The proof also carries a transactions-trie inclusion
  proof. That is standalone cryptographic evidence that the transaction really
  sits at that index in the block, checkable against the block header itself,
  even if you throw away everything else Tessera told you."

## Shot 6 — Hand off and bond (1:40 to 2:02)

- **Screen:** Handoff stage.
- **Action:** Type a counterparty (a DID or an address). Click **anchor and
  confirm handoff**. Show the success screen and the Basescan anchor link.
- **VO:** "To hand the proof to a counterparty, Tessera can also stake a bond on
  it. It anchors the proof on-chain against a USDC honesty stake. Now the claim
  is not just signed, it is backed by money."

## Shot 7 — Fraud dies in one block (2:02 to 2:28) *

- **Screen:** Basescan. Open the real slash transaction:
  https://basescan.org/tx/0xac42c89488e83ecc7dc5b6c1804fb596cddf673c3159e83d6a9243b657d13612
  (the searcher's `challenge` call, in the block right after our fraudulent anchor).
- **Action:** Show that it is a `challenge` on the bond contract and that the
  stake moved to the challenger.
- **VO:** "And here is what makes the bond real. We deliberately anchored a
  fraudulent proof on Base mainnet, one with a wrong block hash. A profit-seeking
  searcher challenged and slashed it in the very next block, about two seconds,
  and took the stake. No oracle, no arbiter, no committee. A lie costs money and
  gets punished by anyone watching, instantly. So an honest oracle's proofs,
  which never carry a wrong hash, are safe, and a dishonest one does not last a
  block."

## Shot 8 — It is a standard, not an app (2:28 to 2:50) *

- **Screen:** Terminal.
- **Action:** Run:
  ```bash
  npm install @olanuel-tessera/avp
  ```
  then show three lines:
  ```js
  import { verifyAVP } from "@olanuel-tessera/avp";
  import { verifyInclusion } from "@olanuel-tessera/avp/inclusion";
  const { ok, signer } = verifyAVP(proof);
  ```
  Optionally run `./demo/demo.sh` and let it print `signature: VALID` and
  `inclusion: VALID`.
- **VO:** "The proof format is a published open standard, with verifiers on npm
  and in Go. Any agent, in any stack, can install it and check a Tessera proof in
  a few lines. This is infrastructure other agents build on, not a one-off app."

## Shot 9 — Close (2:50 to 3:00)

- **Screen:** Back to the landing hero, or the console proof map with nodes
  orbiting.
- **VO:** "Multi-source consensus, finality, a cryptographic inclusion proof, an
  on-chain bond, and an open verifier. That is Tessera. The fact layer for the
  agent economy."

---

## The 60 second cut

1. **0:00 to 0:08** Landing hero. VO: the trust problem in one line.
2. **0:08 to 0:24** Console: paste hash, run consensus, eleven sources agree.
3. **0:24 to 0:40** Proof: point at "signature verified in-browser." VO: verified
   without trusting Tessera.
4. **0:40 to 0:52** Basescan: the fraud anchor slashed in the next block.
5. **0:52 to 1:00** Terminal: `npm install @olanuel-tessera/avp`, `signature:
   VALID`. VO: it is a standard, not an app.

---

## Delivery notes

- Let the consensus stream breathe. The eleven sources ticking to "agreed" is the
  most convincing shot in the whole demo. Do not cut it short.
- The single strongest line is "verified in-browser, with no call back to
  Tessera." Land it slowly.
- The slash story is the credibility close. Have the Basescan tab pre-loaded so
  there is no dead air.
- Anchoring spends a small amount of real ETH from the oracle wallet each time.
  Anchor once for the take. If the wallet is low, skip the live anchor in Shot 6
  and go straight to the already-slashed transaction in Shot 7.
- Keep it calm and specific. The product is impressive on its own. No hype needed.
