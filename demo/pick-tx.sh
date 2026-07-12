#!/usr/bin/env bash
# Prints a Base transaction hash that is ready to demo: finalized (so Tessera
# will sign verified:true), successful (status 1), has a recipient, and recent
# enough (~2500 blocks back) that debug_getRawBlock still serves it so the proof
# carries a merkleProof. Grab one right before recording.
set -euo pipefail
export PYTHONIOENCODING=utf-8

HEAD_RPC="https://mainnet.base.org"
RPC="${BASE_RPC:-https://base.drpc.org}"   # receipts RPC (avoid rate limits on the main one)

head=$(curl -s --max-time 10 -X POST "$HEAD_RPC" -H 'content-type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"eth_getBlockByNumber","params":["finalized",false]}' \
  | python -c "import sys,json;print(int(json.load(sys.stdin)['result']['number'],16))")
b=$((head - 2500))
hx=$(python -c "print(hex($b))")

hashes=$(curl -s --max-time 15 -X POST "$RPC" -H 'content-type: application/json' \
  --data "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"eth_getBlockByNumber\",\"params\":[\"$hx\",false]}" \
  | python -c "import sys,json;print(' '.join(json.load(sys.stdin)['result']['transactions'][1:15]))")

for h in $hashes; do
  st=$(curl -s --max-time 8 -X POST "$RPC" -H 'content-type: application/json' \
    --data "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"eth_getTransactionReceipt\",\"params\":[\"$h\"]}" \
    | python -c "import sys,json;r=json.load(sys.stdin).get('result') or {};print('ok' if r.get('status')=='0x1' and r.get('to') else 'no')" 2>/dev/null || echo no)
  if [ "$st" = "ok" ]; then
    echo "$h"
    exit 0
  fi
done

echo "no ready tx found in block $b; run again" >&2
exit 1
