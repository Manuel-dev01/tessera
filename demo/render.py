"""Pretty-print helper for demo.sh. Reads JSON on stdin; mode chosen by argv[1]."""
import sys, json

mode = sys.argv[1] if len(sys.argv) > 1 else ""
d = json.load(sys.stdin)

if mode == "health":
    print(f"   signer   : {d['signer']}")
    print(f"   sources  : {d['sourceCount']} ({d['quorumFraction']:.2f} quorum, min {d['minResponders']})")
    print(f"   bond     : {d['bondContract']} (enabled={d['bondEnabled']})")
    print(f"   oracleEth: {d.get('oracleEth', '?')}")

elif mode == "pick":
    # stdin is a full block (eth_getBlockByNumber, full=true); print a middle tx with a `to`.
    b = d["result"]
    txs = [x for x in b["transactions"] if x.get("to")]
    print(txs[len(txs) // 2]["hash"])

elif mode == "head":
    print(int(d["result"]["number"], 16))

elif mode == "proof":
    p = d
    c = p["consensus"]
    f = p.get("finality") or {}
    mp = p.get("merkleProof")
    b = p.get("bond") or {}
    mp_str = "null" if not mp else f"{mp['type']} ({len(mp['nodes'])} nodes, root {mp['transactionsRoot'][:14]}...)"
    print(f"   verified   : {p['verified']}   reason: {p.get('reason')}")
    print(f"   block      : {p['blockNumber']}  txIndex {p['txIndex']}")
    print(f"   consensus  : {c['agreed']}/{c['sources']} agreed, quorum {c['quorum']}, {c['responders']} responders")
    print(f"   finality   : finalized={f.get('finalized')} confirmations={f.get('confirmations')}")
    print(f"   merkleProof: {mp_str}")
    print(f"   bond       : {b.get('stakedUSDC', '-')} USDC @ {b.get('contract', '-')}")
    print(f"   signature  : {p['attestation']['scheme']} by {p['attestation']['signer']}")
