#!/usr/bin/env bash
# Tessera end-to-end demo — no credentials required.
#
# Drives the LIVE deployed oracle through a full verification and then
# independently verifies the returned proof with the @tessera/avp SDK:
#
#   1. health check          (11 sources, oracle signer, bond)
#   2. pick a finalized tx    (auto, or pass one as $1)
#   3. hire Tessera           (POST /api/verify -> signed AVP)
#   4. show the proof         (consensus, finality, merkleProof, bond, signature)
#   5. verify independently   (EIP-191 signature + transactions-trie inclusion)
#
# Usage:  ./demo/demo.sh [txHash]
# Env:    TESSERA_API  (default: the hosted Railway API)
#         BASE_RPC     (default: https://mainnet.base.org)
set -euo pipefail
export PYTHONIOENCODING=utf-8

API="${TESSERA_API:-https://tessera-api-production-2b6c.up.railway.app}"
RPC="${BASE_RPC:-https://mainnet.base.org}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
R="$ROOT/demo/render.py"

say()  { printf '\n\033[1;32m== %s\033[0m\n' "$*"; }
info() { printf '   %s\n' "$*"; }
need() { command -v "$1" >/dev/null 2>&1 || { echo "missing dependency: $1"; exit 1; }; }
need curl; need python; need node

say "1. health -- is the oracle online?"
curl -s --max-time 15 "$API/api/health" | python "$R" health

TX="${1:-}"
if [ -z "$TX" ]; then
  say "2. pick a finalized Base tx (auto)"
  HEAD=$(curl -s --max-time 12 -X POST "$RPC" -H 'content-type: application/json' \
    --data '{"jsonrpc":"2.0","id":1,"method":"eth_getBlockByNumber","params":["finalized",false]}' | python "$R" head)
  BN=$((HEAD - 3000)); HX=$(python -c "print(hex($BN))")
  TX=$(curl -s --max-time 12 -X POST "$RPC" -H 'content-type: application/json' \
    --data "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"eth_getBlockByNumber\",\"params\":[\"$HX\",true]}" | python "$R" pick)
  info "block $BN, tx $TX"
else
  say "2. tx (provided)"; info "$TX"
fi

say "3. hire Tessera -- POST /api/verify"
curl -s --max-time 90 -X POST "$API/api/verify" -H 'content-type: application/json' \
  -d "{\"txHash\":\"$TX\"}" > "$ROOT/demo/last-proof.json"
info "signed proof saved to demo/last-proof.json"

say "4. the AgenticVerificationProof"
python "$R" proof < "$ROOT/demo/last-proof.json"

say "5. verify independently with @tessera/avp"
( cd "$ROOT/sdk/js"
  [ -d node_modules ] || npm install --silent >/dev/null 2>&1
  [ -f dist/index.js ] || npm run build --silent >/dev/null 2>&1
  node --input-type=module -e '
import { verifyAVP } from "./dist/index.js";
import { verifyInclusion } from "./dist/inclusion.js";
import { readFileSync } from "node:fs";
const p = JSON.parse(readFileSync(process.argv[1], "utf8"));
const sig = verifyAVP(p);
console.log(`   signature : ${sig.ok ? "VALID" : "INVALID"}  (recovered ${sig.recovered})`);
if (p.merkleProof) {
  const inc = await verifyInclusion(p);
  console.log(`   inclusion : ${inc.ok ? "VALID" : "INVALID"}  ${inc.reason ?? ""}`);
} else {
  console.log("   inclusion : (no merkleProof on this proof)");
}
' "$ROOT/demo/last-proof.json" )

say "done -- a signed, consensus-backed, independently-verified on-chain fact."
