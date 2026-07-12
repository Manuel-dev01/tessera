import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
import { verifyInclusion } from "../dist/inclusion.js";
import { verifyAVP } from "../dist/index.js";

const here = dirname(fileURLToPath(import.meta.url));
const proof = JSON.parse(readFileSync(join(here, "fixtures", "proof.merkle.json"), "utf8"));

test("signature still verifies with merkleProof in the signed bytes", () => {
  assert.equal(verifyAVP(proof).ok, true);
});

test("verifies a real transactions-trie inclusion proof", async () => {
  const r = await verifyInclusion(proof);
  assert.equal(r.ok, true, r.reason);
});

test("full trustless check with the real transactionsRoot", async () => {
  const r = await verifyInclusion(proof, { trustedRoot: proof.merkleProof.transactionsRoot });
  assert.equal(r.ok, true, r.reason);
});

test("rejects a mismatched txHash", async () => {
  const t = structuredClone(proof);
  t.txHash = "0x1111111111111111111111111111111111111111111111111111111111111111";
  const r = await verifyInclusion(t);
  assert.equal(r.ok, false);
});

test("rejects a wrong trusted root", async () => {
  const r = await verifyInclusion(proof, { trustedRoot: "0xdeadbeef00000000000000000000000000000000000000000000000000000000" });
  assert.equal(r.ok, false);
});

test("reports absence of merkleProof", async () => {
  const t = structuredClone(proof);
  t.merkleProof = null;
  const r = await verifyInclusion(t);
  assert.equal(r.ok, false);
  assert.match(r.reason ?? "", /no merkleProof/);
});
