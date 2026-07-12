# Tessera SDK — verify an AgenticVerificationProof

Reference verifiers for **AVP v1** in two languages. Both are standalone (they
depend only on a hashing + secp256k1 library) and are tested against the **same
real proof signed by the live Tessera oracle** — proving the canonical byte form
is reproduced identically across implementations.

| Package | Path | Install |
|---|---|---|
| JavaScript / TypeScript | [`js/`](js/) — `@tessera/avp` | `npm install @tessera/avp` |
| Go | [`go/`](go/) — `github.com/Manuel-dev01/tessera/sdk/go` | `go get github.com/Manuel-dev01/tessera/sdk/go` |

## Why an SDK

An AVP is only useful if a *counterparty agent* can check it without trusting the
issuer. Verification needs nothing but the proof JSON and a crypto library:

1. strip `attestation`,
2. canonicalize the rest per **RFC 8785**,
3. recover the **EIP-191** signer,
4. compare to `attestation.signer`.

A valid signature vouches for every other field, including a `verified: false`
verdict. See the [spec](../schema/agentic-verification-proof-v1.md).

## JS

```ts
import { verifyAVP } from "@tessera/avp";
const { ok, signer } = verifyAVP(proof);
```

## Go

```go
import avp "github.com/Manuel-dev01/tessera/sdk/go"

r, err := avp.Verify(proofJSON, true)
if err == nil && r.OK {
    fmt.Printf("signer %s vouches for this fact\n", r.Signer)
}
```

## Test

```bash
cd js && npm install && npm test     # builds, then verifies the live fixture
cd go && go test ./...               # same fixture, Go verifier
```

Both fixtures (`js/test/fixtures/proof.verified.json`,
`go/testdata/proof.verified.json`) are a byte-identical real proof pulled from
the deployed Tessera API — the Go oracle signed it, and both SDKs recover the
oracle address from it.
