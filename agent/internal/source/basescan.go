package source

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// BasescanSource is a SourceAdapter backed by the Etherscan v2 unified API
// (chainid=8453 for Base) — another independent indexer/trust root. Its
// eth_getTransactionReceipt proxy returns a full receipt (incl. logs), so it can
// vote on the agreement key and event-log checks. Requires an API key; when
// unconfigured it is simply not added to the pool. Abstains from finality.
type BasescanSource struct {
	name   string
	apiKey string
	http   *http.Client
}

func NewBasescanSource(name, apiKey string, hc *http.Client) *BasescanSource {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &BasescanSource{name: name, apiKey: apiKey, http: hc}
}

func (s *BasescanSource) Name() string { return s.name }

type esReceipt struct {
	BlockNumber      string `json:"blockNumber"`
	BlockHash        string `json:"blockHash"`
	TransactionIndex string `json:"transactionIndex"`
	Status           string `json:"status"`
	From             string `json:"from"`
	To               string `json:"to"`
	Logs             []struct {
		LogIndex string   `json:"logIndex"`
		Address  string   `json:"address"`
		Topics   []string `json:"topics"`
	} `json:"logs"`
}

func (s *BasescanSource) Fetch(ctx context.Context, txHash string) (Receipt, error) {
	q := url.Values{}
	q.Set("chainid", "8453")
	q.Set("module", "proxy")
	q.Set("action", "eth_getTransactionReceipt")
	q.Set("txhash", txHash)
	q.Set("apikey", s.apiKey)
	endpoint := "https://api.etherscan.io/v2/api?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Receipt{}, err
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return Receipt{}, fmt.Errorf("%s: %w", s.name, err)
	}
	defer resp.Body.Close()

	var body struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Receipt{}, fmt.Errorf("%s: decode: %w", s.name, err)
	}
	// A missing/failed lookup returns result null or a string message.
	if len(body.Result) == 0 || body.Result[0] != '{' {
		return Receipt{Found: false}, nil
	}

	var r esReceipt
	if err := json.Unmarshal(body.Result, &r); err != nil {
		return Receipt{}, fmt.Errorf("%s: receipt decode: %w", s.name, err)
	}

	out := Receipt{
		Found:       true,
		BlockNumber: hexU64(r.BlockNumber),
		BlockHash:   strings.ToLower(r.BlockHash),
		TxIndex:     uint(hexU64(r.TransactionIndex)),
		Status:      hexU64(r.Status),
		From:        strings.ToLower(r.From),
		To:          strings.ToLower(r.To),
	}
	for _, l := range r.Logs {
		lg := Log{Index: uint(hexU64(l.LogIndex)), Address: strings.ToLower(l.Address)}
		if len(l.Topics) > 0 {
			lg.Topic0 = strings.ToLower(l.Topics[0])
		}
		out.Logs = append(out.Logs, lg)
	}
	return out, nil
}

func hexU64(s string) uint64 {
	n, _ := strconv.ParseUint(strings.TrimPrefix(strings.TrimSpace(s), "0x"), 16, 64)
	return n
}
