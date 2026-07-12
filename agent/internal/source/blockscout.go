package source

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// BlockscoutSource is a SourceAdapter backed by a Blockscout explorer's v2 REST
// API — a fundamentally different verification method (an independent indexer,
// not an RPC node), so it adds genuine trust-root diversity to consensus. It
// does not report chain heads, so it abstains from the finality vote.
type BlockscoutSource struct {
	name    string
	baseURL string // e.g. https://base.blockscout.com
	http    *http.Client
}

func NewBlockscoutSource(name, baseURL string, hc *http.Client) *BlockscoutSource {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &BlockscoutSource{name: name, baseURL: strings.TrimRight(baseURL, "/"), http: hc}
}

func (s *BlockscoutSource) Name() string { return s.name }

type bsTx struct {
	BlockNumber *uint64 `json:"block_number"`
	Position    *uint   `json:"position"`
	Status      string  `json:"status"` // "ok" | "error"
	Result      string  `json:"result"` // "success" | ...
	From        struct {
		Hash string `json:"hash"`
	} `json:"from"`
	To struct {
		Hash string `json:"hash"`
	} `json:"to"`
}

type bsBlock struct {
	Hash string `json:"hash"`
}

// Fetch reads the tx from Blockscout, then its block to obtain the block hash,
// so it can vote on the same (blockNumber, blockHash, status) agreement key as
// the RPC sources. A 404 means the tx isn't indexed → Found:false.
func (s *BlockscoutSource) Fetch(ctx context.Context, txHash string) (Receipt, error) {
	var tx bsTx
	code, err := s.getJSON(ctx, fmt.Sprintf("%s/api/v2/transactions/%s", s.baseURL, txHash), &tx)
	if err != nil {
		return Receipt{}, err
	}
	if code == http.StatusNotFound || tx.BlockNumber == nil {
		return Receipt{Found: false}, nil
	}
	if code != http.StatusOK {
		return Receipt{}, fmt.Errorf("%s: tx http %d", s.name, code)
	}

	out := Receipt{
		Found:       true,
		BlockNumber: *tx.BlockNumber,
		From:        strings.ToLower(tx.From.Hash),
		To:          strings.ToLower(tx.To.Hash),
	}
	if tx.Position != nil {
		out.TxIndex = *tx.Position
	}
	if tx.Status == "ok" || tx.Result == "success" {
		out.Status = 1
	}

	var blk bsBlock
	if code, err := s.getJSON(ctx, fmt.Sprintf("%s/api/v2/blocks/%d", s.baseURL, *tx.BlockNumber), &blk); err == nil && code == http.StatusOK {
		out.BlockHash = strings.ToLower(blk.Hash)
	}
	return out, nil
}

func (s *BlockscoutSource) getJSON(ctx context.Context, url string, v any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", s.name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return resp.StatusCode, nil
	}
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return resp.StatusCode, fmt.Errorf("%s: decode: %w", s.name, err)
	}
	return resp.StatusCode, nil
}
