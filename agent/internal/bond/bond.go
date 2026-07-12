// Package bond is a Go client for the TesseraBond contract on Base: anchoring
// proofs, challenging (slashing) fraudulent ones, and reading the standing bond.
// The oracle EOA (ORACLE_PRIVATE_KEY) signs these txs; Base gas is NOT
// CROO-sponsored, so that EOA must hold a little ETH on Base.
package bond

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	"tessera/agent/internal/proof"
)

// Minimal ABI covering just what Tessera calls.
const bondABI = `[
 {"type":"function","name":"deposit","stateMutability":"nonpayable","inputs":[{"name":"amount","type":"uint256"}],"outputs":[]},
 {"type":"function","name":"withdraw","stateMutability":"nonpayable","inputs":[{"name":"amount","type":"uint256"}],"outputs":[]},
 {"type":"function","name":"anchor","stateMutability":"nonpayable","inputs":[{"name":"proofId","type":"bytes32"},{"name":"blockNumber","type":"uint256"},{"name":"claimedBlockHash","type":"bytes32"},{"name":"slashAmount","type":"uint256"}],"outputs":[]},
 {"type":"function","name":"challenge","stateMutability":"nonpayable","inputs":[{"name":"proofId","type":"bytes32"}],"outputs":[]},
 {"type":"function","name":"free","stateMutability":"view","inputs":[{"name":"","type":"address"}],"outputs":[{"name":"","type":"uint256"}]},
 {"type":"function","name":"challengeWindow","stateMutability":"view","inputs":[],"outputs":[{"name":"","type":"uint256"}]},
 {"type":"function","name":"anchors","stateMutability":"view","inputs":[{"name":"","type":"bytes32"}],"outputs":[{"name":"oracle","type":"address"},{"name":"blockNumber","type":"uint256"},{"name":"claimedBlockHash","type":"bytes32"},{"name":"slashAmount","type":"uint256"},{"name":"unlockAt","type":"uint64"},{"name":"resolved","type":"bool"}]}
]`

// Client wraps a TesseraBond deployment.
type Client struct {
	ec       *ethclient.Client
	contract *bind.BoundContract
	address  common.Address
	key      *ecdsa.PrivateKey
	from     common.Address
	chainID  *big.Int
	parsed   abi.ABI
}

// New dials rpcURL and binds the contract at contractAddr, signing with hexKey.
func New(ctx context.Context, rpcURL, contractAddr, hexKey string) (*Client, error) {
	ec, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}
	chainID, err := ec.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("chainid: %w", err)
	}
	key, err := crypto.HexToECDSA(strings.TrimPrefix(strings.TrimSpace(hexKey), "0x"))
	if err != nil {
		return nil, fmt.Errorf("key: %w", err)
	}
	parsed, err := abi.JSON(strings.NewReader(bondABI))
	if err != nil {
		return nil, fmt.Errorf("abi: %w", err)
	}
	addr := common.HexToAddress(contractAddr)
	return &Client{
		ec:       ec,
		contract: bind.NewBoundContract(addr, parsed, ec, ec, ec),
		address:  addr,
		key:      key,
		from:     crypto.PubkeyToAddress(key.PublicKey),
		chainID:  chainID,
		parsed:   parsed,
	}, nil
}

func (c *Client) From() common.Address { return c.from }

// ProofID is keccak256 of the core canonical proof (attestation + bond removed).
func ProofID(p proof.Proof) ([32]byte, error) {
	core, err := proof.CanonicalCoreBytes(p)
	if err != nil {
		return [32]byte{}, err
	}
	return crypto.Keccak256Hash(core), nil
}

func (c *Client) opts(ctx context.Context) (*bind.TransactOpts, error) {
	o, err := bind.NewKeyedTransactorWithChainID(c.key, c.chainID)
	if err != nil {
		return nil, err
	}
	o.Context = ctx
	return o, nil
}

// Deposit tops up the standing bond (requires prior USDC approval).
func (c *Client) Deposit(ctx context.Context, amount *big.Int) (string, error) {
	return c.send(ctx, "deposit", amount)
}

// Anchor earmarks slashAmount against a proof. Returns the anchor tx hash.
func (c *Client) Anchor(ctx context.Context, proofID [32]byte, blockNumber, slashAmount *big.Int, claimedBlockHash [32]byte) (string, error) {
	return c.send(ctx, "anchor", proofID, blockNumber, claimedBlockHash, slashAmount)
}

// Challenge attempts to slash a fraudulent anchored proof. Returns the tx hash.
func (c *Client) Challenge(ctx context.Context, proofID [32]byte) (string, error) {
	return c.send(ctx, "challenge", proofID)
}

// Free reads an oracle's free (withdrawable) standing bond.
func (c *Client) Free(ctx context.Context, oracle common.Address) (*big.Int, error) {
	var out []any
	if err := c.contract.Call(&bind.CallOpts{Context: ctx}, &out, "free", oracle); err != nil {
		return nil, err
	}
	return out[0].(*big.Int), nil
}

// AnchorInfo is a stored per-proof anchor.
type AnchorInfo struct {
	Oracle           common.Address
	BlockNumber      *big.Int
	ClaimedBlockHash [32]byte
	SlashAmount      *big.Int
	UnlockAt         uint64
	Resolved         bool
	Exists           bool
}

// GetAnchor reads the anchor for proofID from chain.
func (c *Client) GetAnchor(ctx context.Context, proofID [32]byte) (AnchorInfo, error) {
	var out []any
	if err := c.contract.Call(&bind.CallOpts{Context: ctx}, &out, "anchors", proofID); err != nil {
		return AnchorInfo{}, err
	}
	a := AnchorInfo{
		Oracle:           out[0].(common.Address),
		BlockNumber:      out[1].(*big.Int),
		ClaimedBlockHash: out[2].([32]byte),
		SlashAmount:      out[3].(*big.Int),
		UnlockAt:         out[4].(uint64),
		Resolved:         out[5].(bool),
	}
	a.Exists = a.Oracle != (common.Address{})
	return a, nil
}

// CanonicalBlockHash independently reads the canonical hash of blockNumber from
// this client's RPC — the watchtower's own view, distinct from the contract's.
func (c *Client) CanonicalBlockHash(ctx context.Context, blockNumber *big.Int) ([32]byte, error) {
	h, err := c.ec.HeaderByNumber(ctx, blockNumber)
	if err != nil {
		return [32]byte{}, err
	}
	return h.Hash(), nil
}

// ChallengeWindow reads the configured window (seconds).
func (c *Client) ChallengeWindow(ctx context.Context) (*big.Int, error) {
	var out []any
	if err := c.contract.Call(&bind.CallOpts{Context: ctx}, &out, "challengeWindow"); err != nil {
		return nil, err
	}
	return out[0].(*big.Int), nil
}

// send submits a tx and waits for it to be mined, returning the tx hash.
func (c *Client) send(ctx context.Context, method string, args ...any) (string, error) {
	o, err := c.opts(ctx)
	if err != nil {
		return "", err
	}
	tx, err := c.contract.Transact(o, method, args...)
	if err != nil {
		return "", fmt.Errorf("%s: %w", method, err)
	}
	rcpt, err := bind.WaitMined(ctx, c.ec, tx)
	if err != nil {
		return tx.Hash().Hex(), fmt.Errorf("%s wait: %w", method, err)
	}
	if rcpt.Status != 1 {
		return tx.Hash().Hex(), fmt.Errorf("%s reverted (tx %s)", method, tx.Hash().Hex())
	}
	return tx.Hash().Hex(), nil
}
