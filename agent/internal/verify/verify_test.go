package verify

import (
	"strings"
	"testing"

	"tessera/agent/internal/source"
)

func strp(s string) *string { return &s }
func i64p(i int64) *int64   { return &i }

// transferTopic0 is keccak256("Transfer(address,address,uint256)").
const transferTopic0 = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"

func TestVerify(t *testing.T) {
	base := source.Receipt{
		Found:       true,
		BlockNumber: 48316558,
		BlockHash:   "0xabc",
		TxIndex:     7,
		Status:      1,
		From:        "0x1111111111111111111111111111111111111111",
		To:          "0x2222222222222222222222222222222222222222",
		Logs: []source.Log{
			{Index: 3, Address: "0x3333333333333333333333333333333333333333", Topic0: transferTopic0},
		},
	}
	in := Input{ChainID: 8453, TxHash: "0xdead", BlockNumber: 48316558, Address: base.From}

	tests := []struct {
		name       string
		mut        func(*Input, *source.Receipt)
		wantVerify bool
		reasonHas  string
	}{
		{"happy path (sender)", nil, true, ""},
		{"address is recipient", func(i *Input, _ *source.Receipt) { i.Address = base.To }, true, ""},
		{"address is log emitter", func(i *Input, _ *source.Receipt) {
			i.Address = "0x3333333333333333333333333333333333333333"
		}, true, ""},
		{"address case-insensitive", func(i *Input, _ *source.Receipt) { i.Address = strings.ToUpper(base.From) }, true, ""},
		{"tx not found", func(_ *Input, r *source.Receipt) { r.Found = false }, false, "not found"},
		{"wrong block", func(i *Input, _ *source.Receipt) { i.BlockNumber = 999 }, false, "not in claimed block"},
		{"reverted", func(_ *Input, r *source.Receipt) { r.Status = 0 }, false, "reverted"},
		{"address not involved", func(i *Input, _ *source.Receipt) {
			i.Address = "0x9999999999999999999999999999999999999999"
		}, false, "not involved"},
		{"event sig match", func(i *Input, _ *source.Receipt) {
			i.EventSignature = strp("Transfer(address,address,uint256)")
		}, true, ""},
		{"event sig match at logIndex", func(i *Input, _ *source.Receipt) {
			i.EventSignature = strp("Transfer(address,address,uint256)")
			i.LogIndex = i64p(3)
		}, true, ""},
		{"event sig wrong logIndex", func(i *Input, _ *source.Receipt) {
			i.EventSignature = strp("Transfer(address,address,uint256)")
			i.LogIndex = i64p(4)
		}, false, "no matching event log at index 4"},
		{"event sig not present", func(i *Input, _ *source.Receipt) {
			i.EventSignature = strp("Approval(address,address,uint256)")
		}, false, "no matching event log"},
	}

	v := New()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotIn, gotR := in, base
			if tc.mut != nil {
				tc.mut(&gotIn, &gotR)
			}
			res := v.Verify(gotIn, gotR)
			if res.Verified != tc.wantVerify {
				t.Fatalf("Verified = %v, want %v (reason=%v)", res.Verified, tc.wantVerify, deref(res.Reason))
			}
			if tc.wantVerify {
				if res.BlockHash != gotR.BlockHash || res.TxIndex != gotR.TxIndex || res.BlockNumber != gotR.BlockNumber {
					t.Fatalf("block facts not captured: %+v", res)
				}
				if res.Reason != nil {
					t.Fatalf("verified result should have nil reason, got %q", *res.Reason)
				}
			} else {
				if res.Reason == nil || !strings.Contains(*res.Reason, tc.reasonHas) {
					t.Fatalf("reason %q does not contain %q", deref(res.Reason), tc.reasonHas)
				}
			}
		})
	}
}

func deref(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}
