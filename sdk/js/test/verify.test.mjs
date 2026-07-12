import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
import { verifyAVP, recoverSigner, assertAVP, AVP_VERSION } from "../dist/index.js";

const here = dirname(fileURLToPath(import.meta.url));
const proof = JSON.parse(readFileSync(join(here, "fixtures", "proof.verified.json"), "utf8"));

test("verifies a real Go-signed proof (cross-implementation)", () => {
  const r = verifyAVP(proof);
  assert.equal(r.ok, true, r.reason);
  assert.equal(r.recovered.toLowerCase(), proof.attestation.signer.toLowerCase());
});

test("recoverSigner returns the oracle address", () => {
  assert.equal(recoverSigner(proof).toLowerCase(), proof.attestation.signer.toLowerCase());
});

test("rejects a tampered signed field (blockNumber)", () => {
  const t = structuredClone(proof);
  t.blockNumber = proof.blockNumber + 1;
  assert.equal(verifyAVP(t).ok, false);
});

test("rejects a flipped verdict", () => {
  const t = structuredClone(proof);
  t.verified = !proof.verified;
  assert.equal(verifyAVP(t).ok, false);
});

test("rejects an unknown schema version", () => {
  const t = structuredClone(proof);
  t.schemaVersion = "avp/2.0";
  const r = verifyAVP(t);
  assert.equal(r.ok, false);
  assert.match(r.reason ?? "", /unknown AVP version/);
});

test("assertAVP passes on a valid proof and throws on tamper", () => {
  assert.doesNotThrow(() => assertAVP(proof));
  const t = structuredClone(proof);
  t.reason = "injected lie";
  assert.throws(() => assertAVP(t));
});

test("AVP_VERSION is avp/1.0", () => {
  assert.equal(AVP_VERSION, "avp/1.0");
});
