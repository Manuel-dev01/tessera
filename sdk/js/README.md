# @tessera/avp

Verify an **AgenticVerificationProof v1** (AVP) — the signed, consensus-backed
on-chain fact receipt issued by [Tessera](https://github.com/Manuel-dev01/tessera).
Zero trust in Tessera required: given only the proof JSON, this recomputes the
RFC 8785 canonical preimage and recovers the EIP-191 signer.

A valid signature vouches for **every** field — including a `verified: false`
verdict, which is an equally trustworthy, first-class result.

```bash
npm install @tessera/avp
```

## Verify in 3 lines

```ts
import { verifyAVP } from "@tessera/avp";

const { ok, recovered, signer } = verifyAVP(proof);
if (ok) console.log(`signer ${signer} vouches for this fact`);
```

`verifyAVP` returns `{ ok, recovered, signer, reason? }`. `ok` is `true` only
when the address recovered from the signature equals `attestation.signer`. It
also rejects unknown `schemaVersion`s by default (`{ requireVersion: false }` to
skip).

## API

| Export | Signature | Purpose |
|---|---|---|
| `verifyAVP(proof, opts?)` | `(AVProof, { requireVersion?: boolean }) => VerifyResult` | Full check: version + signature. |
| `recoverSigner(proof)` | `(AVProof) => string` | Just recover the EIP-191 signer address. |
| `assertAVP(proof, opts?)` | `(AVProof) => void` | Throws unless the signature is valid. |
| `AVP_VERSION` | `"avp/1.0"` | The version this library validates. |

## How it works

1. Remove the `attestation` member from the proof object.
2. Serialize the rest to **RFC 8785** canonical JSON (`canonicalize`).
3. Recover the **EIP-191** (`personal_sign`) signer over those bytes (`ethers`).
4. Compare to `attestation.signer`.

Byte-exactness is load-bearing: the canonical bytes must match what the issuer
signed. This library reproduces the reference issuer's canonicalizer exactly and
is tested against a **real proof signed by the live Tessera oracle**.

See the [AVP spec](https://github.com/Manuel-dev01/tessera/blob/main/schema/agentic-verification-proof-v1.md).
