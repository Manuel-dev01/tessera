// Package api exposes Tessera's verification stack over HTTP for the operator
// console: health, tx preview, live (SSE) consensus + signed proof, on-chain
// anchoring, and the session proof list. It reuses the exact same engine,
// verifier, signer, and bond client as the CAP provider.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"tessera/agent/internal/app"
	"tessera/agent/internal/bond"
	"tessera/agent/internal/config"
	"tessera/agent/internal/consensus"
	"tessera/agent/internal/handler"
	"tessera/agent/internal/proof"
	"tessera/agent/internal/sign"
	"tessera/agent/internal/verify"
)

// Server holds the shared verification stack + an in-memory session proof store.
type Server struct {
	cfg      *config.Config
	log      *slog.Logger
	engine   *consensus.Engine
	verifier *verify.Verifier
	signer   *sign.Signer
	bond     *bond.Client // nil when bonding disabled/unconfigured
	merkler  handler.MerkleProver // nil when merkle proofs disabled
	ec       *ethclient.Client
	rpcURL   string

	mu     sync.Mutex
	proofs []StoredProof // newest first
}

// StoredProof is a session-issued proof (for the proof map).
type StoredProof struct {
	ProofID  string      `json:"proofId"`
	Claim    string      `json:"claim"`
	Verified bool        `json:"verified"`
	Proof    proof.Proof `json:"proof"`
	IssuedAt int64       `json:"issuedAt"`
}

// New assembles the server from config.
func New(ctx context.Context, cfg *config.Config, log *slog.Logger) (*Server, error) {
	engine, err := app.BuildEngine(cfg, log)
	if err != nil {
		return nil, err
	}
	if cfg.OraclePrivateKey == "" {
		return nil, fmt.Errorf("ORACLE_PRIVATE_KEY required")
	}
	signer, err := sign.NewSigner(cfg.OraclePrivateKey)
	if err != nil {
		return nil, err
	}
	rpcURL := "https://mainnet.base.org"
	if len(cfg.BaseRPCURLs) > 0 {
		rpcURL = cfg.BaseRPCURLs[0]
	}
	ec, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return nil, fmt.Errorf("dial rpc: %w", err)
	}
	s := &Server{
		cfg: cfg, log: log, engine: engine, verifier: verify.New(),
		signer: signer, ec: ec, rpcURL: rpcURL,
		merkler: app.BuildMerkleProver(cfg, log),
	}
	if cfg.BondContract != "" && cfg.OraclePrivateKey != "" {
		if bc, err := bond.New(ctx, rpcURL, cfg.BondContract, cfg.OraclePrivateKey); err == nil {
			s.bond = bc
		} else {
			log.Warn("bond client unavailable", "err", err)
		}
	}
	return s, nil
}

// SignerAddress returns the oracle signer address (for startup logging).
func (s *Server) SignerAddress() string { return s.signer.Address() }

// Handler returns the CORS-wrapped mux.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/tx", s.handleTx)
	mux.HandleFunc("GET /api/verify/stream", s.handleVerifyStream)
	mux.HandleFunc("POST /api/verify", s.handleVerify)
	mux.HandleFunc("POST /api/anchor", s.handleAnchor)
	mux.HandleFunc("GET /api/proofs", s.handleProofs)
	return s.cors(mux)
}

func (s *Server) cors(next http.Handler) http.Handler {
	// WEB_ORIGIN is a comma-separated allow-list. The request Origin is echoed
	// back only if it matches an allowed entry exactly, or is a *.vercel.app
	// host (so Vercel preview deploys work without re-listing every URL).
	var allowed []string
	for _, o := range strings.Split(s.cfg.WebOrigin, ",") {
		if o = strings.TrimSpace(o); o != "" {
			allowed = append(allowed, o)
		}
	}
	if len(allowed) == 0 {
		allowed = []string{"http://localhost:3000"}
	}
	allow := func(origin string) bool {
		if origin == "" {
			return false
		}
		for _, a := range allowed {
			if a == "*" || a == origin {
				return true
			}
		}
		if host := strings.TrimPrefix(strings.TrimPrefix(origin, "https://"), "http://"); strings.HasSuffix(host, ".vercel.app") {
			return true
		}
		return false
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allow(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		} else {
			w.Header().Set("Access-Control-Allow-Origin", allowed[0])
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ---- health ----

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	var ethBal string
	if bal, err := s.ec.BalanceAt(r.Context(), s.signer.Addr(), nil); err == nil {
		ethBal = weiToEth(bal)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"chainId":         8453,
		"signer":          s.signer.Address(),
		"sources":         s.engine.SourceNames(),
		"sourceCount":     s.engine.Sources(),
		"minResponders":   s.engine.MinResponders(),
		"quorumFraction":  s.engine.QuorumFraction(),
		"oracleEth":       ethBal,
		"bondContract":    s.cfg.BondContract,
		"bondEnabled":     s.cfg.BondEnabled,
		"bondAnchorReady": s.bond != nil,
	})
}

// ---- tx preview (compose auto-fill) ----

func (s *Server) handleTx(w http.ResponseWriter, r *http.Request) {
	hash := strings.TrimSpace(r.URL.Query().Get("hash"))
	if hash == "" {
		writeErr(w, http.StatusBadRequest, "hash required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	h := common.HexToHash(hash)
	receipt, err := s.ec.TransactionReceipt(ctx, h)
	if err != nil {
		writeErr(w, http.StatusNotFound, "tx not found: "+err.Error())
		return
	}
	tx, _, err := s.ec.TransactionByHash(ctx, h)
	if err != nil {
		writeErr(w, http.StatusNotFound, "tx not found: "+err.Error())
		return
	}
	from := ""
	if f, err := s.ec.TransactionSender(ctx, tx, receipt.BlockHash, receipt.TransactionIndex); err == nil {
		from = strings.ToLower(f.Hex())
	}
	to := ""
	if tx.To() != nil {
		to = strings.ToLower(tx.To().Hex())
	}
	logs := make([]map[string]any, 0, len(receipt.Logs))
	for _, l := range receipt.Logs {
		t0 := ""
		if len(l.Topics) > 0 {
			t0 = strings.ToLower(l.Topics[0].Hex())
		}
		logs = append(logs, map[string]any{"index": l.Index, "address": strings.ToLower(l.Address.Hex()), "topic0": t0})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"txHash":      hash,
		"blockNumber": receipt.BlockNumber.Uint64(),
		"blockHash":   strings.ToLower(receipt.BlockHash.Hex()),
		"from":        from,
		"to":          to,
		"status":      receipt.Status,
		"logs":        logs,
	})
}

// ---- verify (SSE stream) ----

func (s *Server) handleVerifyStream(w http.ResponseWriter, r *http.Request) {
	in, claim, err := s.inputFromQuery(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	var wmu sync.Mutex
	send := func(event string, v any) {
		wmu.Lock()
		defer wmu.Unlock()
		b, _ := json.Marshal(v)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
		flusher.Flush()
	}

	ctx := r.Context()
	out := s.engine.FetchStream(ctx, in.TxHash, func(rep consensus.SourceReport) {
		send("source", rep)
	})
	send("consensus", map[string]any{
		"reached": out.Reached, "sources": out.Sources, "responders": out.Responders,
		"agreed": out.Agreed, "quorum": out.Quorum, "agreedSources": out.AgreedSources,
		"finalOk": out.FinalOK, "finality": out.Finality, "reason": out.Reason,
	})

	p, err := handler.AssembleProof(ctx, in, out, s.verifier, s.signer, s.bondOpts(), s.merkler)
	if err != nil {
		send("error", map[string]string{"error": err.Error()})
		return
	}
	s.store(p, claim)
	send("proof", p)
}

func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TxHash, Address, Event, Claim string
		BlockNumber                   int64
		LogIndex                      *int64
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "bad json")
		return
	}
	in, err := s.buildInput(r.Context(), body.TxHash, body.Address, body.Event, body.BlockNumber, body.LogIndex)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	out := s.engine.Fetch(r.Context(), in.TxHash)
	p, err := handler.AssembleProof(r.Context(), in, out, s.verifier, s.signer, s.bondOpts(), s.merkler)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.store(p, body.Claim)
	writeJSON(w, http.StatusOK, p)
}

// ---- anchor (Handoff) ----

func (s *Server) handleAnchor(w http.ResponseWriter, r *http.Request) {
	var body struct{ ProofID string }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "bad json")
		return
	}
	if s.bond == nil {
		writeErr(w, http.StatusBadRequest, "on-chain anchoring not configured (set BOND_CONTRACT + fund the oracle)")
		return
	}
	sp := s.find(body.ProofID)
	if sp == nil || sp.Proof.BlockNumber == nil || sp.Proof.BlockHash == nil {
		writeErr(w, http.StatusNotFound, "proof not found or missing block facts")
		return
	}
	slash, _ := new(big.Int).SetString(s.cfg.BondSlashUSDC, 10)
	var pid [32]byte
	copy(pid[:], common.HexToHash(body.ProofID).Bytes())
	blockNum := new(big.Int).SetUint64(uint64(*sp.Proof.BlockNumber))
	claimed := common.HexToHash(*sp.Proof.BlockHash)

	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	tx, err := s.bond.Anchor(ctx, pid, blockNum, slash, claimed)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "anchor failed: "+err.Error())
		return
	}
	// reflect anchored state into the stored proof (re-signed) + response
	updated := s.markAnchored(body.ProofID, tx)
	resp := map[string]any{
		"anchorTx": tx,
		"bond": map[string]any{
			"contract": s.cfg.BondContract, "stakedUSDC": s.cfg.BondStandingStake,
			"challengeWindowSec": s.cfg.ChallengeWindowSec, "proofId": body.ProofID,
			"anchored": true, "anchorTx": tx,
		},
	}
	if updated != nil {
		resp["proof"] = updated // re-signed proof, so it still verifies
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleProofs(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	writeJSON(w, http.StatusOK, s.proofs)
}

// ---- helpers ----

func (s *Server) bondOpts() handler.BondOpts {
	// Advertise the standing bond during verify; anchoring is a separate step.
	return handler.BondOpts{
		Enabled:            s.cfg.BondEnabled,
		Contract:           s.cfg.BondContract,
		StakedUSDC:         s.cfg.BondStandingStake,
		ChallengeWindowSec: s.cfg.ChallengeWindowSec,
	}
}

func (s *Server) inputFromQuery(r *http.Request) (verify.Input, string, error) {
	q := r.URL.Query()
	var logIndex *int64
	if v := q.Get("logIndex"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			logIndex = &n
		}
	}
	bn, _ := strconv.ParseInt(q.Get("blockNumber"), 10, 64)
	in, err := s.buildInput(r.Context(), q.Get("txHash"), q.Get("address"), q.Get("event"), bn, logIndex)
	return in, q.Get("claim"), err
}

// buildInput validates + auto-derives blockNumber from the receipt when absent.
func (s *Server) buildInput(ctx context.Context, txHash, address, event string, blockNumber int64, logIndex *int64) (verify.Input, error) {
	txHash = strings.TrimSpace(txHash)
	if txHash == "" {
		return verify.Input{}, fmt.Errorf("txHash required")
	}
	if blockNumber == 0 {
		cctx, cancel := context.WithTimeout(ctx, 12*time.Second)
		defer cancel()
		if rc, err := s.ec.TransactionReceipt(cctx, common.HexToHash(txHash)); err == nil {
			blockNumber = int64(rc.BlockNumber.Uint64())
		} else {
			return verify.Input{}, fmt.Errorf("could not resolve blockNumber: %v", err)
		}
	}
	in := verify.Input{ChainID: 8453, Address: strings.TrimSpace(address), TxHash: txHash, BlockNumber: blockNumber, LogIndex: logIndex}
	if e := strings.TrimSpace(event); e != "" {
		in.EventSignature = &e
	}
	return in, nil
}

func (s *Server) store(p proof.Proof, claim string) {
	pid := ""
	if p.Bond != nil {
		pid = p.Bond.ProofID
	}
	sp := StoredProof{ProofID: pid, Claim: claim, Verified: p.Verified, Proof: p, IssuedAt: p.IssuedAt}
	s.mu.Lock()
	s.proofs = append([]StoredProof{sp}, s.proofs...)
	if len(s.proofs) > 50 {
		s.proofs = s.proofs[:50]
	}
	s.mu.Unlock()
}

func (s *Server) find(proofID string) *StoredProof {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.proofs {
		if strings.EqualFold(s.proofs[i].ProofID, proofID) {
			return &s.proofs[i]
		}
	}
	return nil
}

// markAnchored records the anchor on the stored proof and RE-SIGNS it. The AVP
// signature covers the `bond` object (only `attestation` is stripped from the
// preimage), so flipping anchored/anchorTx without re-signing would invalidate the
// proof's own signature. Returns the updated, re-signed proof (nil if not found).
func (s *Server) markAnchored(proofID, tx string) *proof.Proof {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.proofs {
		if strings.EqualFold(s.proofs[i].ProofID, proofID) && s.proofs[i].Proof.Bond != nil {
			s.proofs[i].Proof.Bond.Anchored = true
			s.proofs[i].Proof.Bond.AnchorTx = &tx
			if err := s.signer.Sign(&s.proofs[i].Proof); err != nil {
				s.log.Warn("re-sign after anchor failed", "err", err)
			}
			p := s.proofs[i].Proof
			return &p
		}
	}
	return nil
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func weiToEth(w *big.Int) string {
	f := new(big.Float).Quo(new(big.Float).SetInt(w), big.NewFloat(1e18))
	return f.Text('f', 6)
}
