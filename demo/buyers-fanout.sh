#!/usr/bin/env bash
# A2A fan-out: drive N distinct buyer agents, each hiring the live Tessera
# service, to build up unique-buyer-wallet and unique-counterparty breadth on
# CROO for judging.
#
# Each unique buyer wallet is a SEPARATELY REGISTERED CROO agent (its AA wallet +
# SDK key are created in the dashboard, never in code) that has been funded with a
# little USDC. This script consumes their SDK keys; it cannot mint wallets.
#
# Usage:
#   ./demo/buyers-fanout.sh <keys-file> [orders-per-buyer]
#
#   <keys-file>       one buyer SDK key per line (croo_sk_...). Blank lines and
#                     lines starting with # are ignored. Keep it gitignored
#                     (agent/buyers.txt and demo/buyers.txt already are).
#   orders-per-buyer  default 2.
#
# The Tessera provider must be ONLINE (it runs persistently on Railway as
# tessera-provider). Requires SERVICE_ID in ../.env (Tessera's service). Run from
# the repo root or the agent/ directory; it builds agent/req.exe if missing.
set -euo pipefail

KEYS_FILE="${1:?usage: buyers-fanout.sh <keys-file> [orders-per-buyer]}"
ORDERS_PER_BUYER="${2:-2}"

# Resolve the agent directory (this script lives in demo/).
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AGENT_DIR="$(cd "$HERE/.." && pwd)/agent"
cd "$AGENT_DIR"

if [[ ! -f ./req.exe ]]; then
  echo "building req.exe..."
  go build -o req.exe ./cmd/requester
fi

RPC="https://mainnet.base.org"

# pick_finalized_tx prints "BLOCK HASH FROM" for a normal (index>0) tx in the
# current finalized head, so every hire verifies a real, finalized Base tx.
pick_finalized_tx() {
  curl -s -X POST "$RPC" -H 'content-type: application/json' \
    --data '{"jsonrpc":"2.0","id":1,"method":"eth_getBlockByNumber","params":["finalized",true]}' \
  | python -c "
import sys,json
b=json.load(sys.stdin)['result']; bn=int(b['number'],16)
for t in b['transactions']:
    if int(t.get('transactionIndex','0x0'),16)>0:
        print(bn, t['hash'], t['from']); break
"
}

buyer_n=0
order_n=0
ids=()
while IFS= read -r key || [[ -n "$key" ]]; do
  key="$(echo "$key" | tr -d ' \r')"
  [[ -z "$key" || "$key" == \#* ]] && continue
  buyer_n=$((buyer_n+1))
  echo "── buyer #$buyer_n (${key:0:12}...) ─────────────────────────────"
  for ((i=1; i<=ORDERS_PER_BUYER; i++)); do
    read -r BLOCK HASH FROM < <(pick_finalized_tx)
    echo "  order $i/$ORDERS_PER_BUYER  tx=$HASH  block=$BLOCK"
    # Export the buyer key so the requester uses it (it wins over .env, which is
    # only loaded for non-existing vars). Each key is a distinct WS, no collision.
    if out=$(REQUESTER_SDK_KEY="$key" ./req.exe -tx "$HASH" -addr "$FROM" -block "$BLOCK" -timeout 220s 2>&1); then
      oid=$(echo "$out" | grep -oiE 'orderId=[a-f0-9-]+' | head -1 | cut -d= -f2)
      echo "    ✓ completed  order=$oid"
      order_n=$((order_n+1)); ids+=("$oid")
    else
      echo "    ! order failed (see below); continuing"
      echo "$out" | grep -iE 'error|reject' | tail -3 | sed 's/^/      /'
    fi
  done
done < "$KEYS_FILE"

echo "════════════════════════════════════════════════════════════════"
echo "fan-out done: $buyer_n unique buyer wallets, $order_n completed orders"
printf '  order ids: %s\n' "${ids[*]:-(none)}"
