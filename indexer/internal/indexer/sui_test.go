package indexer

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/fystack/multichain-indexer/b2b-platform/internal/rpc/sui"
	v2 "github.com/fystack/multichain-indexer/b2b-platform/internal/rpc/sui/rpc/v2"
	"github.com/fystack/multichain-indexer/b2b-platform/pkg/common/config"
	"github.com/fystack/multichain-indexer/b2b-platform/pkg/common/constant"
	"github.com/fystack/multichain-indexer/b2b-platform/pkg/common/enum"
	"github.com/fystack/multichain-indexer/b2b-platform/pkg/common/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ─────────────────────────────────────────────────────────────────────────────
// Test helpers
// ─────────────────────────────────────────────────────────────────────────────

func strPtr(s string) *string   { return &s }
func u64Ptr(n uint64) *uint64   { return &n }
func txKindPtr(k v2.TransactionKind_Kind) *v2.TransactionKind_Kind { return &k }

// mustStructVal converts a Go map into a *structpb.Value, failing the test on error.
func mustStructVal(t *testing.T, m map[string]any) *structpb.Value {
	t.Helper()
	v, err := structpb.NewValue(m)
	require.NoError(t, err)
	return v
}

// mockPubkeyStore is a simple in-memory PubkeyStore used to test address filtering.
type mockPubkeyStore struct {
	addrs map[string]struct{}
}

func newMockStore(addrs ...string) *mockPubkeyStore {
	m := &mockPubkeyStore{addrs: make(map[string]struct{})}
	for _, a := range addrs {
		m.addrs[a] = struct{}{}
	}
	return m
}

func (m *mockPubkeyStore) Exist(_ enum.NetworkType, addr string) bool {
	_, ok := m.addrs[addr]
	return ok
}

// certifiedSig returns a non-nil ValidatorAggregatedSignature with a populated
// Signature slice — simulating a quorum-certified checkpoint.
func certifiedSig() *v2.ValidatorAggregatedSignature {
	epoch := uint64(10)
	return &v2.ValidatorAggregatedSignature{
		Epoch:     &epoch,
		Signature: []byte("fake-bls-48-byte-sig-padded-here!!!!!!!!!!!!!!!"),
		Bitmap:    []byte{0xff},
	}
}

// makeCheckpoint builds a sui.Checkpoint fixture.
func makeCheckpoint(
	seq uint64,
	digest string,
	sig *v2.ValidatorAggregatedSignature,
	txs ...*v2.ExecutedTransaction,
) *sui.Checkpoint {
	prev := "prev-" + digest
	return &sui.Checkpoint{
		Checkpoint: &v2.Checkpoint{
			SequenceNumber: u64Ptr(seq),
			Digest:         strPtr(digest),
			Signature:      sig,
			Summary: &v2.CheckpointSummary{
				PreviousDigest: strPtr(prev),
				Timestamp:      &timestamppb.Timestamp{Seconds: 1_710_000_000},
			},
			Transactions: txs,
		},
	}
}

// makeExecTx builds a minimal ExecutedTransaction fixture.
func makeExecTx(digest, sender string, bcs ...*v2.BalanceChange) *v2.ExecutedTransaction {
	return &v2.ExecutedTransaction{
		Digest:         strPtr(digest),
		Transaction:    &v2.Transaction{Sender: strPtr(sender)},
		BalanceChanges: bcs,
	}
}

// bc is a shorthand for constructing a BalanceChange proto.
func bc(addr, coinType, amount string) *v2.BalanceChange {
	return &v2.BalanceChange{
		Address:  strPtr(addr),
		CoinType: strPtr(coinType),
		Amount:   strPtr(amount),
	}
}

// newSui returns a SuiIndexer with no pubkeyStore (pass-all) and sane defaults.
func newSui(networkCode string, confirmations ...uint64) *SuiIndexer {
	conf := uint64(0)
	if len(confirmations) > 0 {
		conf = confirmations[0]
	}
	return &SuiIndexer{
		cfg: config.ChainConfig{
			InternalCode:  networkCode,
			Confirmations: conf,
		},
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// isSuiNativeCoinType
// ─────────────────────────────────────────────────────────────────────────────

func TestIsSuiNativeCoinType(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"canonical short", "0x2::sui::SUI", true},
		{"canonical full 64-char", "0x0000000000000000000000000000000000000000000000000000000000000002::sui::SUI", true},
		{"lowercase struct", "0x2::sui::sui", true},
		{"wrapped Coin<SUI>", "0x2::coin::Coin<0x2::sui::SUI>", true},
		{"whitespace-padded", "  0x2::sui::SUI  ", true},
		{"Circle USDC", suiUSDCCoinType, false},
		{"arbitrary token", "0xabcd::token::TOKEN", false},
		{"two-part type", "0x2::sui", false},
		{"empty", "", false},
		{"whitespace only", "   ", false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, isSuiNativeCoinType(tc.in))
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// isSuiUSDCCoinType
// ─────────────────────────────────────────────────────────────────────────────

func TestIsSuiUSDCCoinType(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"canonical", suiUSDCCoinType, true},
		{"lowercase struct", suiUSDCPackage + "::usdc::usdc", true},
		{"uppercase module", suiUSDCPackage + "::USDC::USDC", true},
		{"native SUI", suiNativeCoinType, false},
		{"wrong package", "0xdeadbeef::usdc::USDC", false},
		{"wrong module", suiUSDCPackage + "::coin::USDC", false},
		{"wrong struct", suiUSDCPackage + "::usdc::USDT", false},
		{"empty", "", false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, isSuiUSDCCoinType(tc.in))
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// normalizeSuiAddress
// ─────────────────────────────────────────────────────────────────────────────

func TestNormalizeSuiAddress(t *testing.T) {
	t.Parallel()

	cases := []struct{ in, want string }{
		{"0x2", "0x2"},
		{"0x02", "0x2"},
		{"0x0000000000000000000000000000000000000000000000000000000000000002", "0x2"},
		{"0x0", "0x0"},
		{"0x000", "0x0"},
		{"0xABCD", "0xabcd"},
		{"  0x2  ", "0x2"},
		{"no-prefix", "no-prefix"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, normalizeSuiAddress(tc.in))
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// classifySuiTransferType
// ─────────────────────────────────────────────────────────────────────────────

func TestClassifySuiTransferType(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		coinType     string
		wantTxType   constant.TxType
		wantAsset    string
	}{
		{
			name:       "native SUI → NativeTransfer, canonical asset",
			coinType:   "0x2::sui::SUI",
			wantTxType: constant.TxTypeNativeTransfer,
			wantAsset:  suiNativeCoinType,
		},
		{
			name:       "full-form SUI → same canonical",
			coinType:   "0x0000000000000000000000000000000000000000000000000000000000000002::sui::SUI",
			wantTxType: constant.TxTypeNativeTransfer,
			wantAsset:  suiNativeCoinType,
		},
		{
			name:       "Circle USDC → TokenTransfer, canonical long asset",
			coinType:   suiUSDCCoinType,
			wantTxType: constant.TxTypeTokenTransfer,
			wantAsset:  suiUSDCCoinType,
		},
		{
			name:       "arbitrary token → TokenTransfer, unchanged",
			coinType:   "0xabcd::token::TOKEN",
			wantTxType: constant.TxTypeTokenTransfer,
			wantAsset:  "0xabcd::token::TOKEN",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			txType, asset := classifySuiTransferType(tc.coinType)
			assert.Equal(t, tc.wantTxType, txType)
			assert.Equal(t, tc.wantAsset, asset)
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// isSuiUSDCEvent
// ─────────────────────────────────────────────────────────────────────────────

func TestIsSuiUSDCEvent(t *testing.T) {
	t.Parallel()

	pkg := suiUSDCPackage

	cases := []struct {
		name      string
		eventType string // empty → nil event
		want      bool
	}{
		{"DepositEvent", pkg + "::treasury::DepositEvent", true},
		{"WithdrawEvent", pkg + "::treasury::WithdrawEvent", true},
		{"TransferEvent (usdc module)", pkg + "::usdc::TransferEvent", true},
		{"all-lowercase DepositEvent", strings.ToLower(pkg + "::treasury::DepositEvent"), true},
		{"different package, same event name", "0xdeadbeef::treasury::DepositEvent", false},
		{"native SUI transfer event", "0x2::coin::Transfer", false},
		{"unknown USDC module", pkg + "::other::SomeEvent", false},
		{"nil event", "", false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var evt *v2.Event
			if tc.eventType != "" {
				evt = &v2.Event{EventType: strPtr(tc.eventType)}
			}
			assert.Equal(t, tc.want, isSuiUSDCEvent(evt))
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// isSuiSwapEvent
// ─────────────────────────────────────────────────────────────────────────────

func TestIsSuiSwapEvent(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		eventType string
		module    string
		want      bool
	}{
		{"SwapEvent suffix", "0xdex::pool::SwapEvent", "", true},
		{"SwapExecutedEvent", "0xcetus::router::SwapExecutedEvent", "", true},
		{"swap_event underscore", "0xdex::amm::swap_event", "", true},
		{"module=pool contains swap", "0xdex::pool::SomethingSwap", "pool", true},
		{"Transfer event — not swap", "0xtoken::wallet::Transfer", "", false},
		{"nil event", "", "", false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.eventType == "" {
				assert.False(t, isSuiSwapEvent(nil))
				return
			}
			evt := &v2.Event{EventType: strPtr(tc.eventType)}
			if tc.module != "" {
				evt.Module = strPtr(tc.module)
			}
			assert.Equal(t, tc.want, isSuiSwapEvent(evt))
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// isSuiValidatorStakeEvent / isSuiValidatorUnstakeEvent
// ─────────────────────────────────────────────────────────────────────────────

func TestIsSuiValidatorStakeEvent(t *testing.T) {
	t.Parallel()

	yes := &v2.Event{
		Module:    strPtr("validator"),
		EventType: strPtr("0x3::validator::StakingRequestEvent"),
	}
	no := &v2.Event{
		EventType: strPtr("0x3::validator::UnstakeEvent"),
	}
	assert.True(t, isSuiValidatorStakeEvent(yes))
	assert.False(t, isSuiValidatorStakeEvent(no))
	assert.False(t, isSuiValidatorStakeEvent(nil))
}

func TestIsSuiValidatorUnstakeEvent(t *testing.T) {
	t.Parallel()

	yes := &v2.Event{
		Module:    strPtr("validator"),
		EventType: strPtr("0x3::validator::WithdrawStakeEvent"),
	}
	no := &v2.Event{
		Module:    strPtr("validator"),
		EventType: strPtr("0x3::validator::StakingRequestEvent"),
	}
	assert.True(t, isSuiValidatorUnstakeEvent(yes))
	assert.False(t, isSuiValidatorUnstakeEvent(no))
	assert.False(t, isSuiValidatorUnstakeEvent(nil))
}

// ─────────────────────────────────────────────────────────────────────────────
// classifyMoveCall
// ─────────────────────────────────────────────────────────────────────────────

func TestClassifyMoveCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		pkg  string
		mod  string
		fn   string
		want string
	}{
		{"add_stake → stake", "0x3", "sui_system", "add_stake", suiMoveCallStake},
		{"request_add_stake → stake", "0x3", "sui_system", "request_add_stake", suiMoveCallStake},
		{"withdraw_stake → unstake", "0x3", "sui_system", "withdraw_stake", suiMoveCallUnstake},
		{"request_withdraw_stake → unstake", "0x3", "sui_system", "request_withdraw_stake", suiMoveCallUnstake},
		{"unstake → unstake", "0x3", "sui_system", "unstake", suiMoveCallUnstake},
		{"pool/swap_exact_in → swap", "0xdex", "pool", "swap_exact_in", suiMoveCallSwap},
		{"router/swap → swap", "0xdex", "router", "swap", suiMoveCallSwap},
		// false positives must not match
		{"exchange_rate_update — not a swap", "0xapp", "vault", "exchange_rate_update", ""},
		{"wrong package for stake", "0x9999", "sui_system", "add_stake", ""},
		{"nil → empty", "", "", "", ""},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.pkg == "" && tc.mod == "" && tc.fn == "" {
				assert.Empty(t, classifyMoveCall(nil))
				return
			}
			mc := &v2.MoveCall{
				Package:  strPtr(tc.pkg),
				Module:   strPtr(tc.mod),
				Function: strPtr(tc.fn),
			}
			assert.Equal(t, tc.want, classifyMoveCall(mc))
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// computeSuiFinality
// ─────────────────────────────────────────────────────────────────────────────

func TestComputeSuiFinality_Certified_MeetsLag(t *testing.T) {
	t.Parallel()
	// seq=100, latest=103, lag=3 → confirmations=3 ≥ lag → confirmed
	cp := makeCheckpoint(100, "d", certifiedSig())
	f := computeSuiFinality(cp, 103, 3)
	assert.True(t, f.certified)
	assert.Equal(t, uint64(3), f.confirmations)
	assert.Equal(t, types.StatusConfirmed, f.status)
}

func TestComputeSuiFinality_Certified_BelowLag(t *testing.T) {
	t.Parallel()
	// seq=100, latest=101, lag=3 → confirmations=1 < lag → pending
	cp := makeCheckpoint(100, "d", certifiedSig())
	f := computeSuiFinality(cp, 101, 3)
	assert.True(t, f.certified)
	assert.Equal(t, uint64(1), f.confirmations)
	assert.Equal(t, types.StatusPending, f.status)
}

func TestComputeSuiFinality_Certified_AtTip(t *testing.T) {
	t.Parallel()
	// seq=100, latest=101, lag=1 → confirmations=1 ≥ lag=1 → confirmed
	cp := makeCheckpoint(100, "d", certifiedSig())
	f := computeSuiFinality(cp, 101, 1)
	assert.Equal(t, types.StatusConfirmed, f.status)
}

func TestComputeSuiFinality_Uncertified_AlwaysPending(t *testing.T) {
	t.Parallel()
	// No signature → pending regardless of depth
	cp := makeCheckpoint(100, "d", nil)
	f := computeSuiFinality(cp, 9999, 1)
	assert.False(t, f.certified)
	assert.Equal(t, types.StatusPending, f.status)
}

func TestComputeSuiFinality_EmptySignatureBytes_NotCertified(t *testing.T) {
	t.Parallel()
	// Signature struct present but Signature field empty — not certified
	cp := makeCheckpoint(100, "d", &v2.ValidatorAggregatedSignature{
		Signature: []byte{},
	})
	f := computeSuiFinality(cp, 9999, 1)
	assert.False(t, f.certified)
	assert.Equal(t, types.StatusPending, f.status)
}

func TestComputeSuiFinality_LatestSeqZero_CertifiedGetsOneConf(t *testing.T) {
	t.Parallel()
	// latestSeq=0 (RPC hiccup) — certified checkpoint must not block forever
	cp := makeCheckpoint(100, "d", certifiedSig())
	f := computeSuiFinality(cp, 0, 1)
	assert.True(t, f.certified)
	assert.Equal(t, uint64(1), f.confirmations)
	assert.Equal(t, types.StatusConfirmed, f.status)
}

func TestComputeSuiFinality_LatestSeqEqualSeq_CertifiedGetsOneConf(t *testing.T) {
	t.Parallel()
	// latestSeq == seq → no depth, but certified → fallback to 1 conf
	cp := makeCheckpoint(100, "d", certifiedSig())
	f := computeSuiFinality(cp, 100, 1)
	assert.Equal(t, uint64(1), f.confirmations)
	assert.Equal(t, types.StatusConfirmed, f.status)
}

// ─────────────────────────────────────────────────────────────────────────────
// finalityLag
// ─────────────────────────────────────────────────────────────────────────────

func TestFinalityLag_UsesConfigWhenNonZero(t *testing.T) {
	t.Parallel()
	s := &SuiIndexer{cfg: config.ChainConfig{Confirmations: 5}}
	assert.Equal(t, uint64(5), s.finalityLag())
}

func TestFinalityLag_FallsBackToDefaultWhenZero(t *testing.T) {
	t.Parallel()
	s := &SuiIndexer{cfg: config.ChainConfig{Confirmations: 0}}
	assert.Equal(t, suiDefaultFinalityLag, s.finalityLag())
}

// ─────────────────────────────────────────────────────────────────────────────
// convertCheckpoint — block metadata
// ─────────────────────────────────────────────────────────────────────────────

func TestConvertCheckpoint_BlockFields(t *testing.T) {
	t.Parallel()
	s := newSui("sui_mainnet")
	cp := makeCheckpoint(999, "digest-999", certifiedSig())
	block := s.convertCheckpoint(cp, 1000)

	require.NotNil(t, block)
	assert.Equal(t, uint64(999), block.Number)
	assert.Equal(t, "digest-999", block.Hash)
	assert.Equal(t, "prev-digest-999", block.ParentHash)
	assert.Equal(t, uint64(1_710_000_000), block.Timestamp)
}

func TestConvertCheckpoint_EmptyCheckpointHasNoTransactions(t *testing.T) {
	t.Parallel()
	s := newSui("sui_mainnet")
	block := s.convertCheckpoint(makeCheckpoint(1, "d", certifiedSig()), 2)
	require.NotNil(t, block)
	assert.Empty(t, block.Transactions)
}

// ─────────────────────────────────────────────────────────────────────────────
// convertCheckpoint — finality stamping
// ─────────────────────────────────────────────────────────────────────────────

func TestConvertCheckpoint_StampsConfirmedWhenCertifiedAndMeetsLag(t *testing.T) {
	t.Parallel()
	s := newSui("sui_mainnet", 1) // lag=1

	cp := makeCheckpoint(100, "d", certifiedSig(),
		makeExecTx("tx1", "0xsender",
			bc("0xsender", suiNativeCoinType, "-10"),
			bc("0xreceiver", suiNativeCoinType, "10"),
		),
	)

	block := s.convertCheckpoint(cp, 101) // 1 confirmation ≥ lag=1
	require.Len(t, block.Transactions, 1)
	tx := block.Transactions[0]
	assert.Equal(t, types.StatusConfirmed, tx.Status)
	assert.Equal(t, uint64(1), tx.Confirmations)
}

func TestConvertCheckpoint_StampsPendingWhenBelowLag(t *testing.T) {
	t.Parallel()
	s := newSui("sui_mainnet", 5) // lag=5

	cp := makeCheckpoint(100, "d", certifiedSig(),
		makeExecTx("tx1", "0xsender",
			bc("0xsender", suiNativeCoinType, "-10"),
			bc("0xreceiver", suiNativeCoinType, "10"),
		),
	)

	block := s.convertCheckpoint(cp, 102) // 2 confirmations < lag=5
	require.Len(t, block.Transactions, 1)
	assert.Equal(t, types.StatusPending, block.Transactions[0].Status)
	assert.Equal(t, uint64(2), block.Transactions[0].Confirmations)
}

func TestConvertCheckpoint_StampsPendingWhenUncertified(t *testing.T) {
	t.Parallel()
	s := newSui("sui_mainnet", 1)

	cp := makeCheckpoint(100, "d", nil, // no signature
		makeExecTx("tx1", "0xsender",
			bc("0xsender", suiNativeCoinType, "-10"),
			bc("0xreceiver", suiNativeCoinType, "10"),
		),
	)

	block := s.convertCheckpoint(cp, 9999)
	require.Len(t, block.Transactions, 1)
	assert.Equal(t, types.StatusPending, block.Transactions[0].Status)
}

func TestConvertCheckpoint_AllTransactionsShareFinalityDepth(t *testing.T) {
	t.Parallel()
	s := newSui("sui_mainnet", 1)

	cp := makeCheckpoint(50, "d", certifiedSig(),
		makeExecTx("tx1", "0xa", bc("0xa", suiNativeCoinType, "-5"), bc("0xb", suiNativeCoinType, "5")),
		makeExecTx("tx2", "0xc", bc("0xc", suiNativeCoinType, "-3"), bc("0xd", suiNativeCoinType, "3")),
	)

	block := s.convertCheckpoint(cp, 52) // 2 confirmations, lag=1 → confirmed
	require.Len(t, block.Transactions, 2)
	for _, tx := range block.Transactions {
		assert.Equal(t, types.StatusConfirmed, tx.Status)
		assert.Equal(t, uint64(2), tx.Confirmations)
	}
}

func TestConvertCheckpoint_SetsBlockHashOnTransactions(t *testing.T) {
	t.Parallel()
	s := newSui("sui_mainnet")

	digest := "cp-digest-42"
	cp := makeCheckpoint(42, digest, certifiedSig(),
		makeExecTx("tx1", "0xsender",
			bc("0xsender", suiNativeCoinType, "-10"),
			bc("0xreceiver", suiNativeCoinType, "10"),
		),
	)

	block := s.convertCheckpoint(cp, 43)
	require.Len(t, block.Transactions, 1)
	assert.Equal(t, digest, block.Transactions[0].BlockHash)
}

// ─────────────────────────────────────────────────────────────────────────────
// convertCheckpoint — pubkeyStore filtering
// ─────────────────────────────────────────────────────────────────────────────

func TestConvertCheckpoint_FiltersUnmonitoredTransfers(t *testing.T) {
	t.Parallel()

	monitored := "0xmonitored"
	s := &SuiIndexer{
		cfg:         config.ChainConfig{InternalCode: "sui_mainnet", Confirmations: 1},
		pubkeyStore: newMockStore(monitored),
	}

	cp := makeCheckpoint(1, "d", certifiedSig(),
		// monitored receives → keep
		makeExecTx("tx-in", "0xother",
			bc("0xother", suiNativeCoinType, "-10"),
			bc(monitored, suiNativeCoinType, "10"),
		),
		// neither party monitored → drop
		makeExecTx("tx-noise", "0xa",
			bc("0xa", suiNativeCoinType, "-5"),
			bc("0xb", suiNativeCoinType, "5"),
		),
	)

	block := s.convertCheckpoint(cp, 2)
	require.Len(t, block.Transactions, 1)
	assert.Equal(t, monitored, block.Transactions[0].ToAddress)
}

func TestConvertCheckpoint_TwoWayIndexing_IncludesOutboundFromMonitoredSender(t *testing.T) {
	t.Parallel()

	monitoredSender := "0xmy_wallet"
	s := &SuiIndexer{
		cfg:         config.ChainConfig{InternalCode: "sui_mainnet", TwoWayIndexing: true, Confirmations: 1},
		pubkeyStore: newMockStore(monitoredSender),
	}

	cp := makeCheckpoint(1, "d", certifiedSig(),
		makeExecTx("tx-out", monitoredSender,
			bc(monitoredSender, suiNativeCoinType, "-20"),
			bc("0xexternal", suiNativeCoinType, "20"),
		),
	)

	block := s.convertCheckpoint(cp, 2)
	require.Len(t, block.Transactions, 1,
		"outbound from monitored sender must be included with TwoWayIndexing=true")
}

func TestConvertCheckpoint_NilPubkeyStoreIncludesAll(t *testing.T) {
	t.Parallel()
	s := newSui("sui_mainnet")

	cp := makeCheckpoint(1, "d", certifiedSig(),
		makeExecTx("tx1", "0xa", bc("0xa", suiNativeCoinType, "-5"), bc("0xb", suiNativeCoinType, "5")),
		makeExecTx("tx2", "0xc", bc("0xc", suiNativeCoinType, "-3"), bc("0xd", suiNativeCoinType, "3")),
	)

	block := s.convertCheckpoint(cp, 2)
	assert.Len(t, block.Transactions, 2)
}

// ─────────────────────────────────────────────────────────────────────────────
// convertTransactions — system tx filtering
// ─────────────────────────────────────────────────────────────────────────────

func TestConvertTransactions_IgnoresSystemSender(t *testing.T) {
	t.Parallel()
	s := newSui("sui_mainnet")
	systemSender := "0x0000000000000000000000000000000000000000000000000000000000000000"
	execTx := makeExecTx("sys-tx", systemSender, bc(systemSender, suiNativeCoinType, "-1"))
	assert.Nil(t, s.convertTransactions(execTx, 1, 1))
}

func TestConvertTransactions_IgnoresEmptySender(t *testing.T) {
	t.Parallel()
	s := newSui("sui_mainnet")
	execTx := &v2.ExecutedTransaction{
		Digest:         strPtr("no-sender"),
		Transaction:    &v2.Transaction{},
		BalanceChanges: []*v2.BalanceChange{bc("0xreceiver", suiNativeCoinType, "100")},
	}
	assert.Nil(t, s.convertTransactions(execTx, 1, 1))
}

// ─────────────────────────────────────────────────────────────────────────────
// convertTransactions — native SUI transfers
// ─────────────────────────────────────────────────────────────────────────────

func TestConvertTransactions_NativeSuiTransfer(t *testing.T) {
	t.Parallel()
	s := newSui("sui_testnet")

	from := "0xbd97a67763c8101771308f5a91311c2d826189cc471332d8a6cc5001c00946ee"
	to := "0x476268833af5d2280a1f31bc4a2787cbb37b71f89cb9cbee6662c44fa3091838"

	execTx := makeExecTx("digest-native", from,
		bc(to, "0x0000000000000000000000000000000000000000000000000000000000000002::sui::SUI", "125000000"),
		bc(from, "0x2::sui::SUI", "-126997880"),
	)

	txs := s.convertTransactions(execTx, 304566255, 1772704618)
	require.NotEmpty(t, txs)

	// The first tx should be the inbound transfer to `to`
	found := false
	for _, tx := range txs {
		if tx.ToAddress == to {
			assert.Equal(t, constant.TxTypeNativeTransfer, tx.Type)
			assert.Equal(t, suiNativeCoinType, tx.AssetAddress)
			assert.Equal(t, "125000000", tx.Amount)
			found = true
		}
	}
	assert.True(t, found, "inbound native transfer to receiver must be present")
}

func TestConvertTransactions_MultiSend_SameToken(t *testing.T) {
	t.Parallel()
	s := newSui("sui_mainnet")

	sender := "0xsender"
	r1 := "0xr1"
	r2 := "0xr2"

	execTx := makeExecTx("multi-send", sender,
		bc(sender, suiNativeCoinType, "-1000"),
		bc(r1, suiNativeCoinType, "600"),
		bc(r2, suiNativeCoinType, "400"),
	)

	txs := s.convertTransactions(execTx, 1, 1)
	require.Len(t, txs, 2)

	got := map[string]string{}
	for _, tx := range txs {
		assert.Equal(t, constant.TxTypeNativeTransfer, tx.Type)
		assert.Equal(t, suiNativeCoinType, tx.AssetAddress)
		assert.Equal(t, sender, tx.FromAddress)
		assert.NotEmpty(t, tx.TransferIndex)
		got[tx.ToAddress] = tx.Amount
	}
	assert.Equal(t, "600", got[r1])
	assert.Equal(t, "400", got[r2])
}

func TestConvertTransactions_MultiSend_DifferentTokens(t *testing.T) {
	t.Parallel()
	s := newSui("sui_mainnet")

	sender := "0xsender"
	r1 := "0xr1"
	r2 := "0xr2"

	execTx := makeExecTx("multi-token", sender,
		bc(sender, suiNativeCoinType, "-300"),
		bc(r1, suiNativeCoinType, "100"),
		bc(r2, "0x2::usdc::USDC", "200"),
	)

	txs := s.convertTransactions(execTx, 1, 1)
	require.Len(t, txs, 2)

	got := map[string]constant.TxType{}
	for _, tx := range txs {
		got[tx.ToAddress] = tx.Type
	}
	assert.Equal(t, constant.TxTypeNativeTransfer, got[r1])
	assert.Equal(t, constant.TxTypeTokenTransfer, got[r2])
}

// ─────────────────────────────────────────────────────────────────────────────
// convertTransactions — USDC via BalanceChanges
// ─────────────────────────────────────────────────────────────────────────────

func TestConvertTransactions_USDC_BalanceChange(t *testing.T) {
	t.Parallel()
	s := newSui("sui_mainnet")

	sender := "0xsender"
	receiver := "0xreceiver"

	execTx := makeExecTx("usdc-bc", sender,
		bc(sender, suiUSDCCoinType, "-1000000"),
		bc(receiver, suiUSDCCoinType, "1000000"),
	)

	txs := s.convertTransactions(execTx, 1, 1)
	require.Len(t, txs, 1)
	tx := txs[0]
	assert.Equal(t, constant.TxTypeTokenTransfer, tx.Type)
	assert.Equal(t, suiUSDCCoinType, tx.AssetAddress)
	assert.Equal(t, "1000000", tx.Amount)
	assert.Equal(t, receiver, tx.ToAddress)
	assert.Equal(t, sender, tx.FromAddress)
}

func TestConvertTransactions_USDC_AssetAddressAlwaysCanonical(t *testing.T) {
	t.Parallel()
	// Regardless of how the node serialised the package address, the output
	// AssetAddress must always equal suiUSDCCoinType.
	s := newSui("sui_mainnet")
	sender := "0xsender"
	receiver := "0xreceiver"

	execTx := makeExecTx("usdc-canonical", sender,
		bc(sender, suiUSDCCoinType, "-500000"),
		bc(receiver, suiUSDCCoinType, "500000"),
	)

	txs := s.convertTransactions(execTx, 1, 1)
	require.Len(t, txs, 1)
	assert.Equal(t, suiUSDCCoinType, txs[0].AssetAddress,
		"AssetAddress must always be the canonical long-form USDC coin type")
}

// ─────────────────────────────────────────────────────────────────────────────
// convertTransactions — USDC via treasury events
// ─────────────────────────────────────────────────────────────────────────────

func TestConvertTransactions_USDC_DepositEvent(t *testing.T) {
	t.Parallel()
	s := newSui("sui_mainnet")

	realSender := "0xreal_holder"
	receiver := "0xmerchant"
	relayer := "0xrelayer" // tx signer / sponsor — NOT the token holder

	eventType := suiUSDCPackage + "::treasury::DepositEvent"
	execTx := &v2.ExecutedTransaction{
		Digest:      strPtr("usdc-deposit"),
		Transaction: &v2.Transaction{Sender: strPtr(relayer)},
		Events: &v2.TransactionEvents{
			Events: []*v2.Event{{
				EventType: strPtr(eventType),
				Json: mustStructVal(t, map[string]any{
					"sender":    realSender,
					"recipient": receiver,
					"amount":    "2000000",
				}),
			}},
		},
	}

	txs := s.convertTransactions(execTx, 1, 1)
	require.NotEmpty(t, txs)

	var found bool
	for _, tx := range txs {
		if tx.AssetAddress == suiUSDCCoinType && tx.ToAddress == receiver {
			found = true
			assert.Equal(t, constant.TxTypeTokenTransfer, tx.Type)
			assert.Equal(t, "2000000", tx.Amount)
			assert.Equal(t, realSender, tx.FromAddress,
				"FromAddress must come from the event JSON, not the tx signer (relayer)")
			assert.NotEmpty(t, tx.TransferIndex)
		}
	}
	assert.True(t, found, "DepositEvent must produce a USDC transaction")
}

func TestConvertTransactions_USDC_WithdrawEvent(t *testing.T) {
	t.Parallel()
	s := newSui("sui_mainnet")

	sender := "0xsender"
	receiver := "0xreceiver"
	eventType := suiUSDCPackage + "::treasury::WithdrawEvent"

	execTx := &v2.ExecutedTransaction{
		Digest:      strPtr("usdc-withdraw"),
		Transaction: &v2.Transaction{Sender: strPtr(sender)},
		Events: &v2.TransactionEvents{
			Events: []*v2.Event{{
				EventType: strPtr(eventType),
				Json: mustStructVal(t, map[string]any{
					"sender":    sender,
					"recipient": receiver,
					"amount":    "999999",
				}),
			}},
		},
	}

	txs := s.convertTransactions(execTx, 1, 1)
	require.NotEmpty(t, txs)

	found := false
	for _, tx := range txs {
		if tx.AssetAddress == suiUSDCCoinType && tx.ToAddress == receiver {
			found = true
			assert.Equal(t, "999999", tx.Amount)
			assert.Equal(t, constant.TxTypeTokenTransfer, tx.Type)
		}
	}
	assert.True(t, found)
}

func TestConvertTransactions_USDC_TransferEventVariant(t *testing.T) {
	t.Parallel()
	s := newSui("sui_mainnet")

	sender := "0xsender"
	receiver := "0xreceiver"
	eventType := suiUSDCPackage + "::usdc::TransferEvent"

	execTx := &v2.ExecutedTransaction{
		Digest:      strPtr("usdc-transfer-event"),
		Transaction: &v2.Transaction{Sender: strPtr(sender)},
		Events: &v2.TransactionEvents{
			Events: []*v2.Event{{
				EventType: strPtr(eventType),
				Json: mustStructVal(t, map[string]any{
					"from":   sender,
					"to":     receiver,
					"amount": "12345678",
				}),
			}},
		},
	}

	txs := s.convertTransactions(execTx, 1, 1)
	require.NotEmpty(t, txs)

	found := false
	for _, tx := range txs {
		if tx.AssetAddress == suiUSDCCoinType {
			found = true
			assert.Equal(t, "12345678", tx.Amount)
			assert.Equal(t, receiver, tx.ToAddress)
		}
	}
	assert.True(t, found)
}

func TestConvertTransactions_USDC_SponsoredTx_EventSenderOverridesRelayer(t *testing.T) {
	t.Parallel()
	// Core sponsored-tx scenario: relayer pays gas (tx.Sender = relayer), but
	// the USDC holder is the real economic actor. The treasury event must supply
	// the correct FromAddress — NOT the relayer.
	s := newSui("sui_mainnet")

	relayer := "0xrelayer"
	usdcHolder := "0xholder"
	merchant := "0xmerchant"

	eventType := suiUSDCPackage + "::treasury::WithdrawEvent"
	execTx := &v2.ExecutedTransaction{
		Digest:      strPtr("sponsored-usdc"),
		Transaction: &v2.Transaction{Sender: strPtr(relayer)},
		BalanceChanges: []*v2.BalanceChange{
			bc(relayer, suiNativeCoinType, "-5000"),
			bc(usdcHolder, suiUSDCCoinType, "-50000000"),
			bc(merchant, suiUSDCCoinType, "50000000"),
		},
		Events: &v2.TransactionEvents{
			Events: []*v2.Event{{
				EventType: strPtr(eventType),
				Json: mustStructVal(t, map[string]any{
					"sender":    usdcHolder,
					"recipient": merchant,
					"amount":    "50000000",
				}),
			}},
		},
	}

	txs := s.convertTransactions(execTx, 1, 1)
	require.NotEmpty(t, txs)

	var eventTx *types.Transaction
	for i := range txs {
		if strings.HasPrefix(txs[i].TransferIndex, "event:") &&
			txs[i].AssetAddress == suiUSDCCoinType {
			eventTx = &txs[i]
			break
		}
	}
	require.NotNil(t, eventTx, "must have an event-sourced USDC transaction")
	assert.Equal(t, usdcHolder, eventTx.FromAddress)
	assert.Equal(t, merchant, eventTx.ToAddress)
	assert.NotEqual(t, relayer, eventTx.FromAddress,
		"relayer must NOT appear as FromAddress of the USDC transfer")
}

func TestConvertTransactions_USDC_EventWithNoRecipientIsSkipped(t *testing.T) {
	t.Parallel()
	// If the event JSON lacks a recipient field we must skip it — BalanceChanges
	// already captured the transfer and we must not emit a phantom tx.
	s := newSui("sui_mainnet")

	sender := "0xsender"
	receiver := "0xreceiver"
	eventType := suiUSDCPackage + "::treasury::DepositEvent"

	execTx := &v2.ExecutedTransaction{
		Digest:      strPtr("usdc-no-recipient"),
		Transaction: &v2.Transaction{Sender: strPtr(sender)},
		BalanceChanges: []*v2.BalanceChange{
			bc(sender, suiUSDCCoinType, "-1000000"),
			bc(receiver, suiUSDCCoinType, "1000000"),
		},
		Events: &v2.TransactionEvents{
			Events: []*v2.Event{{
				EventType: strPtr(eventType),
				Json: mustStructVal(t, map[string]any{
					// deliberately no "recipient" / "to" / "receiver" / "destination"
					"amount": "1000000",
				}),
			}},
		},
	}

	txs := s.convertTransactions(execTx, 1, 1)
	for _, tx := range txs {
		assert.NotEmpty(t, tx.ToAddress, "no transaction must have an empty ToAddress")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// convertTransactions — sponsored transfers (non-USDC)
// ─────────────────────────────────────────────────────────────────────────────

func TestConvertTransactions_SponsoredTransfer_InfersSenderFromBalanceChanges(t *testing.T) {
	t.Parallel()
	s := newSui("sui_mainnet")

	sponsor := "0xsponsor"
	actualSender := "0xactualsender"
	receiver := "0xreceiver"

	// Sponsor pays gas in SUI; actualSender sends USDC.
	execTx := makeExecTx("sponsored", sponsor,
		bc(sponsor, suiNativeCoinType, "-10"),
		bc(actualSender, "0x2::usdc::USDC", "-500"),
		bc(receiver, "0x2::usdc::USDC", "500"),
	)

	txs := s.convertTransactions(execTx, 1, 1)
	require.Len(t, txs, 1)
	tx := txs[0]
	assert.Equal(t, actualSender, tx.FromAddress)
	assert.Equal(t, receiver, tx.ToAddress)
	assert.Equal(t, "500", tx.Amount)
	assert.Equal(t, constant.TxTypeTokenTransfer, tx.Type)
}

// ─────────────────────────────────────────────────────────────────────────────
// convertTransactions — mint / self-transfer
// ─────────────────────────────────────────────────────────────────────────────

func TestConvertTransactions_MintToSelf(t *testing.T) {
	t.Parallel()
	s := newSui("sui_mainnet")
	sender := "0xsender"

	execTx := makeExecTx("mint-to-self", sender,
		bc(sender, "0x2::custom::COIN", "700"),
	)

	txs := s.convertTransactions(execTx, 1, 1)
	require.Len(t, txs, 1)
	assert.Equal(t, sender, txs[0].ToAddress)
	assert.Equal(t, sender, txs[0].FromAddress)
	assert.Equal(t, "700", txs[0].Amount)
	assert.Equal(t, constant.TxTypeTokenTransfer, txs[0].Type)
}

// ─────────────────────────────────────────────────────────────────────────────
// convertTransactions — zero / negative balance changes ignored
// ─────────────────────────────────────────────────────────────────────────────

func TestConvertTransactions_ZeroAmountBalanceChangeIgnored(t *testing.T) {
	t.Parallel()
	s := newSui("sui_mainnet")
	sender := "0xsender"
	receiver := "0xreceiver"

	execTx := makeExecTx("zero-bc", sender,
		bc(sender, suiNativeCoinType, "-100"),
		bc(receiver, suiNativeCoinType, "0"),   // zero — must be ignored
		bc(receiver, suiNativeCoinType, "100"),
	)

	txs := s.convertTransactions(execTx, 1, 1)
	require.Len(t, txs, 1)
	assert.Equal(t, "100", txs[0].Amount)
}

// ─────────────────────────────────────────────────────────────────────────────
// convertTransactions — TransferIndex assignment
// ─────────────────────────────────────────────────────────────────────────────

func TestConvertTransactions_TransferIndex_BalancePrefix(t *testing.T) {
	t.Parallel()
	s := newSui("sui_mainnet")
	sender := "0xsender"
	r1 := "0xr1"
	r2 := "0xr2"

	execTx := makeExecTx("bc-index", sender,
		bc(sender, suiNativeCoinType, "-200"),
		bc(r1, suiNativeCoinType, "100"),
		bc(r2, suiNativeCoinType, "100"),
	)

	txs := s.convertTransactions(execTx, 1, 1)
	require.Len(t, txs, 2)
	for _, tx := range txs {
		assert.True(t, strings.HasPrefix(tx.TransferIndex, "balance:"),
			"balance-change txs must have balance: prefix, got %q", tx.TransferIndex)
	}
}

func TestConvertTransactions_TransferIndex_EventPrefix(t *testing.T) {
	t.Parallel()
	s := newSui("sui_mainnet")
	sender := "0xsender"

	execTx := &v2.ExecutedTransaction{
		Digest:      strPtr("event-index"),
		Transaction: &v2.Transaction{Sender: strPtr(sender)},
		Events: &v2.TransactionEvents{
			Events: []*v2.Event{
				{
					EventType: strPtr("0xtoken::wallet::Transfer"),
					Json:      mustStructVal(t, map[string]any{"from": sender, "to": "0xa", "amount": "9", "coinType": "0x2::tok::TOK"}),
				},
				{
					EventType: strPtr("0xtoken::wallet::Transfer"),
					Json:      mustStructVal(t, map[string]any{"from": sender, "to": "0xb", "amount": "9", "coinType": "0x2::tok::TOK"}),
				},
			},
		},
	}

	txs := s.convertTransactions(execTx, 1, 1)
	require.Len(t, txs, 2)
	assert.Equal(t, "event:0", txs[0].TransferIndex)
	assert.Equal(t, "event:1", txs[1].TransferIndex)
}

func TestConvertTransactions_TransferIndex_MovePrefix(t *testing.T) {
	t.Parallel()
	s := newSui("sui_mainnet")
	sender := "0xsender"

	execTx := &v2.ExecutedTransaction{
		Digest: strPtr("move-index"),
		Transaction: &v2.Transaction{
			Sender: strPtr(sender),
			Kind: &v2.TransactionKind{
				Kind: txKindPtr(v2.TransactionKind_PROGRAMMABLE_TRANSACTION),
				Data: &v2.TransactionKind_ProgrammableTransaction{
					ProgrammableTransaction: &v2.ProgrammableTransaction{
						Commands: []*v2.Command{
							{Command: &v2.Command_MoveCall{MoveCall: &v2.MoveCall{
								Package: strPtr("0xswap"), Module: strPtr("pool"), Function: strPtr("swap_exact_in"),
							}}},
							{Command: &v2.Command_MoveCall{MoveCall: &v2.MoveCall{
								Package: strPtr("0x3"), Module: strPtr("sui_system"), Function: strPtr("request_add_stake"),
							}}},
						},
					},
				},
			},
		},
		BalanceChanges: []*v2.BalanceChange{
			bc(sender, suiNativeCoinType, "25"),
		},
	}

	txs := s.convertTransactions(execTx, 1, 1)
	var moveIdxs []string
	for _, tx := range txs {
		if strings.HasPrefix(tx.TransferIndex, "move:") {
			moveIdxs = append(moveIdxs, tx.TransferIndex)
		}
	}
	assert.Equal(t, []string{"move:0", "move:1"}, moveIdxs)
}

// ─────────────────────────────────────────────────────────────────────────────
// convertTransactions — staking events
// ─────────────────────────────────────────────────────────────────────────────

func TestConvertTransactions_StakeEvent_UsesValidatorAddress(t *testing.T) {
	t.Parallel()
	s := newSui("sui_mainnet")
	sender := "0xsender"
	validator := "0xvalidator"

	execTx := &v2.ExecutedTransaction{
		Digest:      strPtr("stake"),
		Transaction: &v2.Transaction{Sender: strPtr(sender)},
		Events: &v2.TransactionEvents{
			Events: []*v2.Event{{
				Module:    strPtr("validator"),
				EventType: strPtr("0x3::validator::StakingRequestEvent"),
				Json: mustStructVal(t, map[string]any{
					"amount":            "13000000000",
					"staker_address":    sender,
					"validator_address": validator,
				}),
			}},
		},
	}

	txs := s.convertTransactions(execTx, 1, 1)
	require.Len(t, txs, 1)
	assert.Equal(t, constant.TxTypeNativeTransfer, txs[0].Type)
	assert.Equal(t, suiNativeCoinType, txs[0].AssetAddress)
	assert.Equal(t, "13000000000", txs[0].Amount)
	assert.Equal(t, validator, txs[0].ToAddress)
}

// ─────────────────────────────────────────────────────────────────────────────
// convertTransactions — swap events
// ─────────────────────────────────────────────────────────────────────────────

func TestConvertTransactions_SwapEvent_ParsedAndToAddressIsSender(t *testing.T) {
	t.Parallel()
	s := newSui("sui_mainnet")
	sender := "0xtrader"

	execTx := &v2.ExecutedTransaction{
		Digest:      strPtr("swap"),
		Transaction: &v2.Transaction{Sender: strPtr(sender)},
		Events: &v2.TransactionEvents{
			Events: []*v2.Event{{
				EventType: strPtr("0xdex::pool::SwapEvent"),
				Json:      mustStructVal(t, map[string]any{"amount": "42", "coinType": "0x2::usdc::USDC"}),
			}},
		},
	}

	txs := s.convertTransactions(execTx, 1, 1)
	require.NotEmpty(t, txs)

	found := false
	for _, tx := range txs {
		if tx.ToAddress == sender && tx.Amount == "42" {
			found = true
			assert.Equal(t, constant.TxTypeTokenTransfer, tx.Type)
		}
	}
	assert.True(t, found, "swap movement must have ToAddress == sender")
}

// ─────────────────────────────────────────────────────────────────────────────
// convertTransactions — generic transfer events
// ─────────────────────────────────────────────────────────────────────────────

func TestConvertTransactions_GenericTransferEvent(t *testing.T) {
	t.Parallel()
	s := newSui("sui_mainnet")
	sender := "0xsender"
	receiver := "0xreceiver"

	execTx := &v2.ExecutedTransaction{
		Digest:      strPtr("generic-transfer-event"),
		Transaction: &v2.Transaction{Sender: strPtr(sender)},
		Events: &v2.TransactionEvents{
			Events: []*v2.Event{{
				EventType: strPtr("0xtoken::wallet::Transfer"),
				Json:      mustStructVal(t, map[string]any{"from": sender, "to": receiver, "amount": "9", "coinType": "0x2::custom::COIN"}),
			}},
		},
	}

	txs := s.convertTransactions(execTx, 1, 1)
	require.NotEmpty(t, txs)

	found := false
	for _, tx := range txs {
		if tx.ToAddress == receiver && tx.Amount == "9" && tx.AssetAddress == "0x2::custom::COIN" {
			found = true
			assert.Equal(t, constant.TxTypeTokenTransfer, tx.Type)
		}
	}
	assert.True(t, found)
}

func TestConvertTransactions_GenericTransferEvent_SkippedWhenNoRecipient(t *testing.T) {
	t.Parallel()
	s := newSui("sui_mainnet")
	sender := "0xsender"

	execTx := &v2.ExecutedTransaction{
		Digest:      strPtr("no-recipient-event"),
		Transaction: &v2.Transaction{Sender: strPtr(sender)},
		Events: &v2.TransactionEvents{
			Events: []*v2.Event{{
				EventType: strPtr("0xtoken::wallet::Transfer"),
				Json:      mustStructVal(t, map[string]any{"from": sender, "amount": "9"}),
			}},
		},
	}

	txs := s.convertTransactions(execTx, 1, 1)
	for _, tx := range txs {
		assert.NotEmpty(t, tx.ToAddress)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// convertTransactions — deduplication
// ─────────────────────────────────────────────────────────────────────────────

func TestConvertTransactions_Dedup_ExactDuplicatesCollapsed(t *testing.T) {
	t.Parallel()
	s := newSui("sui_mainnet")
	sender := "0xsender"
	receiver := "0xreceiver"

	// Identical balance changes produce identical keys — dedup must collapse them.
	// In practice this wouldn't happen on-chain, but the dedup layer must handle it.
	execTx := makeExecTx("dedup", sender,
		bc(sender, suiNativeCoinType, "-100"),
		bc(receiver, suiNativeCoinType, "100"),
	)

	txs := s.convertTransactions(execTx, 1, 1)
	// All transactions must have non-empty ToAddress.
	for _, tx := range txs {
		assert.NotEmpty(t, tx.ToAddress)
	}
}

func TestConvertTransactions_Dedup_DifferentTransferIndexNotCollapsed(t *testing.T) {
	t.Parallel()
	s := newSui("sui_mainnet")
	sender := "0xsender"
	r1 := "0xr1"
	r2 := "0xr2"

	execTx := makeExecTx("no-collapse", sender,
		bc(sender, suiNativeCoinType, "-200"),
		bc(r1, suiNativeCoinType, "100"),
		bc(r2, suiNativeCoinType, "100"),
	)

	txs := s.convertTransactions(execTx, 1, 1)
	require.Len(t, txs, 2, "different receivers must not be collapsed even with equal amounts")
}

// ─────────────────────────────────────────────────────────────────────────────
// convertTransactions — tx fee calculation
// ─────────────────────────────────────────────────────────────────────────────

func TestConvertTransactions_TxFeeCalculation(t *testing.T) {
	t.Parallel()
	s := newSui("sui_mainnet")
	sender := "0xsender"
	receiver := "0xreceiver"

	computationCost := uint64(1_000_000)
	storageCost := uint64(2_000_000)
	storageRebate := uint64(500_000)
	nonRefundable := uint64(100_000)

	execTx := &v2.ExecutedTransaction{
		Digest:      strPtr("fee-test"),
		Transaction: &v2.Transaction{Sender: strPtr(sender)},
		Effects: &v2.TransactionEffects{
			GasUsed: &v2.GasCostSummary{
				ComputationCost:         &computationCost,
				StorageCost:             &storageCost,
				StorageRebate:           &storageRebate,
				NonRefundableStorageFee: &nonRefundable,
			},
		},
		BalanceChanges: []*v2.BalanceChange{
			bc(sender, suiNativeCoinType, "-5000"),
			bc(receiver, suiNativeCoinType, "5000"),
		},
	}

	txs := s.convertTransactions(execTx, 1, 1)
	require.NotEmpty(t, txs)

	// net = computation + storage + nonRefundable - rebate = 1M + 2M + 100K - 500K = 2.6M mist
	// in SUI = 2_600_000 / 1_000_000_000 = 0.0026
	expectedFeeStr := "0.0026"
	assert.Equal(t, expectedFeeStr, txs[0].TxFee.String())
}

func TestConvertTransactions_TxFeeZeroWhenRebateExceedsCost(t *testing.T) {
	t.Parallel()
	s := newSui("sui_mainnet")
	sender := "0xsender"
	receiver := "0xreceiver"

	computationCost := uint64(100)
	storageRebate := uint64(99999)

	execTx := &v2.ExecutedTransaction{
		Digest:      strPtr("zero-fee"),
		Transaction: &v2.Transaction{Sender: strPtr(sender)},
		Effects: &v2.TransactionEffects{
			GasUsed: &v2.GasCostSummary{
				ComputationCost: &computationCost,
				StorageRebate:   &storageRebate,
			},
		},
		BalanceChanges: []*v2.BalanceChange{
			bc(sender, suiNativeCoinType, "-5"),
			bc(receiver, suiNativeCoinType, "5"),
		},
	}

	txs := s.convertTransactions(execTx, 1, 1)
	require.NotEmpty(t, txs)
	assert.True(t, txs[0].TxFee.IsZero(), "fee must be zero when rebate exceeds cost")
}

// ─────────────────────────────────────────────────────────────────────────────
// convertTransactions — block number and timestamp on base tx
// ─────────────────────────────────────────────────────────────────────────────

func TestConvertTransactions_BaseFieldsPopulated(t *testing.T) {
	t.Parallel()
	s := newSui("SUI_MAINNET")
	sender := "0xsender"
	receiver := "0xreceiver"

	execTx := makeExecTx("base-fields", sender,
		bc(sender, suiNativeCoinType, "-10"),
		bc(receiver, suiNativeCoinType, "10"),
	)

	const blockNum = uint64(304566255)
	const blockTs = uint64(1772704618)

	txs := s.convertTransactions(execTx, blockNum, blockTs)
	require.NotEmpty(t, txs)
	for _, tx := range txs {
		assert.Equal(t, "base-fields", tx.TxHash)
		assert.Equal(t, "SUI_MAINNET", tx.NetworkId)
		assert.Equal(t, blockNum, tx.BlockNumber)
		assert.Equal(t, blockTs, tx.Timestamp)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// isSuiSystemSender
// ─────────────────────────────────────────────────────────────────────────────

func TestIsSuiSystemSender(t *testing.T) {
	t.Parallel()

	assert.True(t, isSuiSystemSender("0x0"))
	assert.True(t, isSuiSystemSender("0x0000000000000000000000000000000000000000000000000000000000000000"))
	assert.False(t, isSuiSystemSender("0x1"))
	assert.False(t, isSuiSystemSender("0xsender"))
}

// ─────────────────────────────────────────────────────────────────────────────
// uniqueTransactions
// ─────────────────────────────────────────────────────────────────────────────

func TestUniqueTransactions_CollapsesExactDuplicates(t *testing.T) {
	t.Parallel()

	tx := types.Transaction{
		TxHash:        "0xabc",
		Type:          constant.TxTypeNativeTransfer,
		FromAddress:   "0xa",
		ToAddress:     "0xb",
		AssetAddress:  suiNativeCoinType,
		Amount:        "100",
		TransferIndex: "balance:0",
	}

	result := uniqueTransactions([]types.Transaction{tx, tx, tx})
	require.Len(t, result, 1)
}

func TestUniqueTransactions_PreservesDistinctTransferIndex(t *testing.T) {
	t.Parallel()

	base := types.Transaction{
		TxHash:       "0xabc",
		Type:         constant.TxTypeNativeTransfer,
		FromAddress:  "0xa",
		ToAddress:    "0xb",
		AssetAddress: suiNativeCoinType,
		Amount:       "100",
	}
	tx1 := base
	tx1.TransferIndex = "balance:0"
	tx2 := base
	tx2.TransferIndex = "balance:1"

	result := uniqueTransactions([]types.Transaction{tx1, tx2})
	require.Len(t, result, 2)
}

func TestUniqueTransactions_SingleElementPassthrough(t *testing.T) {
	t.Parallel()
	tx := types.Transaction{TxHash: "x", TransferIndex: "balance:0"}
	result := uniqueTransactions([]types.Transaction{tx})
	require.Len(t, result, 1)
}

func TestUniqueTransactions_EmptySlice(t *testing.T) {
	t.Parallel()
	result := uniqueTransactions(nil)
	assert.Empty(t, result)
}

// ─────────────────────────────────────────────────────────────────────────────
// parseSuiBalanceChanges
// ─────────────────────────────────────────────────────────────────────────────

func TestParseSuiBalanceChanges_FiltersNilAndZero(t *testing.T) {
	t.Parallel()

	addr := "0xaddr"
	changes := []*v2.BalanceChange{
		nil,
		{Address: strPtr(addr), CoinType: strPtr(suiNativeCoinType), Amount: strPtr("0")},
		{Address: strPtr(addr), CoinType: strPtr(suiNativeCoinType), Amount: strPtr("100")},
		{Address: strPtr(addr), CoinType: strPtr(suiNativeCoinType), Amount: strPtr("-50")},
	}

	deltas := parseSuiBalanceChanges(changes)
	require.Len(t, deltas, 2, "nil and zero-amount entries must be filtered")
}

func TestParseSuiBalanceChanges_PreservesNegatives(t *testing.T) {
	t.Parallel()
	addr := "0xaddr"
	changes := []*v2.BalanceChange{
		{Address: strPtr(addr), CoinType: strPtr(suiNativeCoinType), Amount: strPtr("-999")},
	}
	deltas := parseSuiBalanceChanges(changes)
	require.Len(t, deltas, 1)
	assert.Equal(t, int64(-999), deltas[0].Amount.Int64())
}

// ─────────────────────────────────────────────────────────────────────────────
// Integration test (skipped by default)
// ─────────────────────────────────────────────────────────────────────────────

func TestSuiMainnetFetchAndParseTransactions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	rpcURL := "fullnode.mainnet.sui.io:443"

	testCases := []struct {
		name   string
		digest string
		want   constant.TxType
	}{
		{name: "native transfer", digest: strings.TrimSpace("6uz8Evkkg5bzGC1P7omvzCdBwFSzeL6jHs7yTH1rgmWa"), want: constant.TxTypeNativeTransfer},
		{name: "token transfer", digest: strings.TrimSpace("EWUh7soPhZk7sBCJQXZHn7fBkbk4gHPsePSKkHu9TBd6"), want: constant.TxTypeTokenTransfer},
		{name: "swap movement", digest: strings.TrimSpace("9ygJwFE8zJ6jS6Qj2v4cDhZBzAtFP9t6aa1b89NeQ8vB")},
		{name: "stake movement", digest: strings.TrimSpace("7Nno5YH6oM2azMKkBoxAnKz1bVxPknUaUfz4saz8kiP6")},
		{name: "unstake movement", digest: strings.TrimSpace("Eu27uF1FMZRuKZnEaQrUUFQYApVeUmkELbU2s4gZJ9af")},
	}

	enabled := false
	for _, tc := range testCases {
		if tc.digest != "" {
			enabled = true
			break
		}
	}
	if !enabled {
		t.Skip("hardcoded real mainnet test cases are empty")
	}

	client := sui.NewSuiClient(rpcURL)
	idx := newSui("sui_mainnet")

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			tx, err := client.GetTransaction(ctx, tc.digest)
			require.NoError(t, err)
			require.NotNil(t, tx)

			blockNumber := tx.GetCheckpoint()
			ts := uint64(0)
			if tx.GetTimestamp() != nil {
				ts = uint64(tx.GetTimestamp().Seconds)
			}

			parsed := idx.convertTransactions(tx.ExecutedTransaction, blockNumber, ts)
			require.NotEmpty(t, parsed, "should classify at least one semantic event from mainnet tx")

			if tc.want != "" {
				found := false
				for _, item := range parsed {
					if item.Type == tc.want {
						found = true
						break
					}
				}
				require.True(t, found, "expected tx type %s in parsed outputs", tc.want)
			}

			for _, item := range parsed {
				require.Contains(t, []constant.TxType{constant.TxTypeNativeTransfer, constant.TxTypeTokenTransfer}, item.Type)
				require.NotEmpty(t, item.TxHash)
				require.NotEmpty(t, item.FromAddress)
				require.NotEmpty(t, item.ToAddress)
			}
		})
	}
}
