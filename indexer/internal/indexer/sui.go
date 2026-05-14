package indexer

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fystack/multichain-indexer/b2b-platform/internal/rpc"
	"github.com/fystack/multichain-indexer/b2b-platform/internal/rpc/sui"
	v2 "github.com/fystack/multichain-indexer/b2b-platform/internal/rpc/sui/rpc/v2"
	"github.com/fystack/multichain-indexer/b2b-platform/pkg/common/config"
	"github.com/fystack/multichain-indexer/b2b-platform/pkg/common/constant"
	"github.com/fystack/multichain-indexer/b2b-platform/pkg/common/enum"
	"github.com/fystack/multichain-indexer/b2b-platform/pkg/common/types"
	"github.com/shopspring/decimal"
	"golang.org/x/sync/semaphore"
)

// SuiIndexer implements the generic Indexer interface for the Sui blockchain,
// using the gRPC-based SuiAPI client defined in internal/rpc/sui.
type SuiIndexer struct {
	chainName   string
	cfg         config.ChainConfig
	failover    *rpc.Failover[sui.SuiAPI]
	pubkeyStore PubkeyStore
}

const suiMistPerSUI = 1_000_000_000
const suiNativeCoinType = "0x2::sui::SUI"
const suiSystemPackage = "0x3"
const suiZeroAddress = "0x0"
const suiMoveCallStake = "stake"
const suiMoveCallUnstake = "unstake"
const suiMoveCallSwap = "swap"

const suiUSDCPackage = "0xa1ec7fc00a6f40db9693ad1415d0c193ad3906494428cf252621037bd7117e29"
const suiUSDCCoinType = suiUSDCPackage + "::usdc::USDC"

// suiDefaultFinalityLag is the number of checkpoints we stay behind the tip
// before treating a checkpoint as confirmed. On Sui, checkpoints are BFT-final
// the moment they carry a ValidatorAggregatedSignature — there are no reorgs.
// This lag is purely a propagation hedge for public full-nodes that may briefly
// disagree on the latest sequence number. Private/dedicated nodes can set
// confirmations: 0 in YAML to process every checkpoint immediately.
const suiDefaultFinalityLag uint64 = 1

// ── balance-change delta ──────────────────────────────────────────────────────

type suiBalanceDelta struct {
	Address  string
	CoinType string
	Amount   *big.Int
}

// ── coin-type helpers ─────────────────────────────────────────────────────────

func isSuiNativeCoinType(coinType string) bool {
	coinType = strings.TrimSpace(coinType)
	if coinType == "" {
		return false
	}
	// Some nodes/serializers may return wrapped types (e.g. Coin<0x2::sui::SUI>).
	if i := strings.Index(coinType, "<"); i >= 0 && strings.HasSuffix(coinType, ">") {
		inner := strings.TrimSpace(coinType[i+1 : len(coinType)-1])
		if isSuiNativeCoinType(inner) {
			return true
		}
	}
	parts := strings.Split(coinType, "::")
	if len(parts) != 3 {
		return false
	}
	return normalizeSuiAddress(parts[0]) == "0x2" &&
		strings.EqualFold(parts[1], "sui") &&
		strings.EqualFold(parts[2], "SUI")
}

// isSuiUSDCCoinType reports whether coinType is Circle's native USDC on Sui.
// Both short-form and full-length package addresses are matched after normalisation.
func isSuiUSDCCoinType(coinType string) bool {
	coinType = strings.TrimSpace(coinType)
	if coinType == "" {
		return false
	}
	parts := strings.Split(coinType, "::")
	if len(parts) != 3 {
		return false
	}
	return normalizeSuiAddress(parts[0]) == normalizeSuiAddress(suiUSDCPackage) &&
		strings.EqualFold(parts[1], "usdc") &&
		strings.EqualFold(parts[2], "USDC")
}

// ── address helpers ───────────────────────────────────────────────────────────

func normalizeSuiAddress(addr string) string {
	addr = strings.TrimSpace(strings.ToLower(addr))
	if !strings.HasPrefix(addr, "0x") {
		return addr
	}
	hex := strings.TrimLeft(addr[2:], "0")
	if hex == "" {
		hex = "0"
	}
	return "0x" + hex
}

func normalizeSuiIdentifier(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func sameSuiAddress(a, b string) bool {
	return normalizeSuiAddress(a) == normalizeSuiAddress(b)
}

func isSuiSystemSender(addr string) bool {
	return sameSuiAddress(addr, suiZeroAddress)
}

// ── finality ──────────────────────────────────────────────────────────────────

// suiCheckpointFinality holds the derived finality attributes for a single
// checkpoint, computed once and stamped on every transaction it contains.
type suiCheckpointFinality struct {
	confirmations uint64
	status        string
	certified     bool // true when ValidatorAggregatedSignature is present
}

// computeSuiFinality derives finality for the checkpoint at sequence number seq,
// given the current chain tip (latestSeq) and the operator-configured lag.
//
// Sui finality model:
//   - Every checkpoint that exists on-chain already carries a quorum
//     ValidatorAggregatedSignature — there are no reorgs. Finality is
//     deterministic, not probabilistic.
//   - We set Status=confirmed for any checkpoint whose Signature field is
//     populated AND that is at least `lag` checkpoints behind the tip.
//   - Confirmations mirrors the EVM convention (latestSeq - seq) so downstream
//     consumers have a consistent field to reason about.
//   - If latestSeq is unknown (0) we still confirm certified checkpoints with
//     confirmations=1 rather than blocking indefinitely.
func computeSuiFinality(cp *sui.Checkpoint, latestSeq uint64, lag uint64) suiCheckpointFinality {
	seq := cp.SequenceNumber()

	// A checkpoint is certified when its aggregated validator signature is set.
	certified := cp.GetSignature() != nil && len(cp.GetSignature().GetSignature()) > 0

	var confirmations uint64
	if latestSeq > seq {
		confirmations = latestSeq - seq
	} else if certified {
		// latestSeq unknown or equal — treat a certified checkpoint as 1 conf.
		confirmations = 1
	}

	status := types.StatusPending
	if certified && confirmations >= lag {
		status = types.StatusConfirmed
	}

	return suiCheckpointFinality{
		confirmations: confirmations,
		status:        status,
		certified:     certified,
	}
}

// ── tx construction helpers ───────────────────────────────────────────────────

func makeSuiBaseTransaction(execTx *v2.ExecutedTransaction, networkID string, blockNumber, blockTs uint64) types.Transaction {
	t := types.Transaction{
		TxHash:      execTx.GetDigest(),
		NetworkId:   networkID,
		BlockNumber: blockNumber,
		Timestamp:   blockTs,
	}
	if execTx.Transaction != nil {
		t.FromAddress = execTx.Transaction.GetSender()
	}
	if execTx.Effects != nil && execTx.Effects.GasUsed != nil {
		g := execTx.Effects.GasUsed
		cost := uint64(0)
		if g.StorageCost != nil {
			cost += *g.StorageCost
		}
		if g.ComputationCost != nil {
			cost += *g.ComputationCost
		}
		if g.NonRefundableStorageFee != nil {
			cost += *g.NonRefundableStorageFee
		}
		if g.StorageRebate != nil {
			if cost > *g.StorageRebate {
				cost -= *g.StorageRebate
			} else {
				cost = 0
			}
		}
		t.TxFee = decimal.NewFromBigInt(new(big.Int).SetUint64(cost), 0).Div(decimal.NewFromInt(suiMistPerSUI))
	}
	return t
}

func parseSuiBalanceChanges(changes []*v2.BalanceChange) []suiBalanceDelta {
	deltas := make([]suiBalanceDelta, 0, len(changes))
	for _, bc := range changes {
		if bc == nil {
			continue
		}
		amt, ok := new(big.Int).SetString(bc.GetAmount(), 10)
		if !ok || amt.Sign() == 0 {
			continue
		}
		deltas = append(deltas, suiBalanceDelta{
			Address:  bc.GetAddress(),
			CoinType: bc.GetCoinType(),
			Amount:   amt,
		})
	}
	return deltas
}

func biggestPositiveDelta(deltas []suiBalanceDelta, skipAddr string) (suiBalanceDelta, bool) {
	var best suiBalanceDelta
	found := false
	for _, delta := range deltas {
		if skipAddr != "" && sameSuiAddress(delta.Address, skipAddr) {
			continue
		}
		if delta.Amount.Sign() <= 0 {
			continue
		}
		if !found || delta.Amount.Cmp(best.Amount) > 0 {
			best = delta
			found = true
		}
	}
	return best, found
}

func biggestNegativeDeltaByCoinType(deltas []suiBalanceDelta, coinType string) (suiBalanceDelta, bool) {
	var best suiBalanceDelta
	found := false
	for _, delta := range deltas {
		if delta.Amount.Sign() >= 0 || delta.CoinType != coinType {
			continue
		}
		if !found || delta.Amount.Cmp(best.Amount) < 0 {
			best = delta
			found = true
		}
	}
	return best, found
}

func parseBigIntString(v string) (*big.Int, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, false
	}
	n, ok := new(big.Int).SetString(v, 10)
	return n, ok
}

func classifySuiTransferType(coinType string) (constant.TxType, string) {
	if isSuiNativeCoinType(coinType) {
		return constant.TxTypeNativeTransfer, suiNativeCoinType
	}
	// Normalise USDC coin type to the canonical long-form address so downstream
	// consumers always see a consistent AssetAddress regardless of how the node
	// serialised it.
	if isSuiUSDCCoinType(coinType) {
		return constant.TxTypeTokenTransfer, suiUSDCCoinType
	}
	return constant.TxTypeTokenTransfer, coinType
}

func normalizeSuiMovementType(tx *types.Transaction) {
	if tx == nil {
		return
	}
	tx.Type, tx.AssetAddress = classifySuiTransferType(tx.AssetAddress)
}

// ── event JSON helpers ────────────────────────────────────────────────────────

func eventJSONMap(evt *v2.Event) map[string]any {
	if evt == nil || evt.GetJson() == nil {
		return nil
	}
	raw := evt.GetJson().AsInterface()
	m, _ := raw.(map[string]any)
	return m
}

func jsonString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		v, ok := m[key]
		if !ok || v == nil {
			continue
		}
		switch val := v.(type) {
		case string:
			return strings.TrimSpace(val)
		case float64:
			if val == float64(int64(val)) {
				return strconv.FormatInt(int64(val), 10)
			}
		}
	}
	return ""
}

func eventAmountString(m map[string]any) string {
	for _, key := range []string{"amount", "value", "qty"} {
		if v := jsonString(m, key); v != "" {
			if n, ok := parseBigIntString(v); ok {
				return n.String()
			}
		}
	}
	return "0"
}

func eventCoinType(evt *v2.Event, m map[string]any) string {
	if coinType := jsonString(m, "coinType", "coin_type", "type", "asset", "tokenType"); coinType != "" {
		return coinType
	}
	eventType := evt.GetEventType()
	if parts := strings.Split(eventType, "::"); len(parts) >= 3 {
		return strings.Join(parts[:3], "::")
	}
	return ""
}

// ── event-type classifiers ────────────────────────────────────────────────────

func isSuiSwapModule(module string) bool {
	module = normalizeSuiIdentifier(module)
	for _, keyword := range []string{"pool", "router", "amm", "clmm"} {
		if strings.Contains(module, keyword) {
			return true
		}
	}
	return false
}

func isSuiSwapFunction(function string) bool {
	function = normalizeSuiIdentifier(function)
	for _, keyword := range []string{"swap", "swap_exact", "swap_x", "swap_y", "swap_a", "swap_b"} {
		if strings.Contains(function, keyword) {
			return true
		}
	}
	return false
}

func isSuiSwapEvent(evt *v2.Event) bool {
	if evt == nil {
		return false
	}
	eventType := normalizeSuiIdentifier(evt.GetEventType())
	module := normalizeSuiIdentifier(evt.GetModule())

	switch {
	case strings.Contains(eventType, "::swapevent"),
		strings.Contains(eventType, "::swap_event"),
		strings.Contains(eventType, "::swapexecutedevent"),
		strings.Contains(eventType, "::swap_executed_event"),
		strings.Contains(eventType, "::swapcompletedevent"),
		strings.Contains(eventType, "::swap_completed_event"):
		return true
	case isSuiSwapModule(module):
		return strings.Contains(eventType, "swap")
	default:
		return false
	}
}

func isSuiValidatorStakeEvent(evt *v2.Event) bool {
	if evt == nil {
		return false
	}
	eventType := normalizeSuiIdentifier(evt.GetEventType())
	module := normalizeSuiIdentifier(evt.GetModule())
	return strings.Contains(eventType, "::validator::stakingrequestevent") ||
		(module == "validator" && strings.Contains(eventType, "stakingrequest"))
}

func isSuiValidatorUnstakeEvent(evt *v2.Event) bool {
	if evt == nil {
		return false
	}
	eventType := normalizeSuiIdentifier(evt.GetEventType())
	module := normalizeSuiIdentifier(evt.GetModule())
	return (module == "validator" || strings.Contains(eventType, "::validator::")) &&
		(strings.Contains(eventType, "withdrawstake") ||
			strings.Contains(eventType, "unstake") ||
			strings.Contains(eventType, "withdrawal"))
}

// isSuiUSDCEvent reports whether the event was emitted by Circle's native USDC
// treasury module on Sui. Circle emits events from the `treasury` module inside
// the USDC package for mint/burn/transfer operations:
//
//	0xdba3...::treasury::DepositEvent   – user receives USDC (mint or inbound transfer)
//	0xdba3...::treasury::WithdrawEvent  – user sends USDC (burn or outbound transfer)
//	0xdba3...::usdc::TransferEvent      – peer-to-peer transfer (some contract versions)
//
// We match on the normalised package prefix so future module additions inside
// the same USDC package are picked up automatically.
func isSuiUSDCEvent(evt *v2.Event) bool {
	if evt == nil {
		return false
	}
	eventType := normalizeSuiIdentifier(evt.GetEventType())
	normalizedPkg := normalizeSuiIdentifier(suiUSDCPackage)

	if !strings.HasPrefix(eventType, normalizedPkg) {
		return false
	}

	return strings.Contains(eventType, "::treasury::depositevent") ||
		strings.Contains(eventType, "::treasury::withdrawevent") ||
		strings.Contains(eventType, "::usdc::transferevent")
}

// ── move-call helpers ─────────────────────────────────────────────────────────

func commandSummary(tx *v2.Transaction) (moveCalls []*v2.MoveCall) {
	if tx == nil || tx.GetKind() == nil {
		return
	}
	pt := tx.GetKind().GetProgrammableTransaction()
	if pt == nil {
		return
	}
	for _, cmd := range pt.GetCommands() {
		if cmd.GetMoveCall() != nil {
			moveCalls = append(moveCalls, cmd.GetMoveCall())
		}
	}
	return
}

func classifyMoveCall(mc *v2.MoveCall) string {
	if mc == nil {
		return ""
	}
	module := normalizeSuiIdentifier(mc.GetModule())
	function := normalizeSuiIdentifier(mc.GetFunction())
	pkg := normalizeSuiAddress(mc.GetPackage())

	switch {
	case pkg == suiSystemPackage && module == "sui_system" &&
		(strings.Contains(function, "add_stake") || strings.Contains(function, "request_add_stake")):
		return suiMoveCallStake
	case pkg == suiSystemPackage && module == "sui_system" &&
		(strings.Contains(function, "withdraw_stake") || strings.Contains(function, "request_withdraw_stake") || strings.Contains(function, "unstake")):
		return suiMoveCallUnstake
	case isSuiSwapModule(module) && isSuiSwapFunction(function):
		return suiMoveCallSwap
	default:
		return ""
	}
}

// ── dedup ─────────────────────────────────────────────────────────────────────

func uniqueTransactions(txs []types.Transaction) []types.Transaction {
	if len(txs) <= 1 {
		return txs
	}

	out := make([]types.Transaction, 0, len(txs))
	seen := make(map[string]struct{}, len(txs))
	for _, tx := range txs {
		key := strings.Join([]string{
			tx.TxHash,
			string(tx.Type),
			normalizeSuiAddress(tx.FromAddress),
			normalizeSuiAddress(tx.ToAddress),
			tx.AssetAddress,
			tx.Amount,
			tx.TransferIndex,
		}, "|")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, tx)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		if out[i].AssetAddress != out[j].AssetAddress {
			return out[i].AssetAddress < out[j].AssetAddress
		}
		if out[i].ToAddress != out[j].ToAddress {
			return out[i].ToAddress < out[j].ToAddress
		}
		ia, iok := new(big.Int).SetString(out[i].Amount, 10)
		ja, jok := new(big.Int).SetString(out[j].Amount, 10)
		if iok && jok {
			return ia.Cmp(ja) < 0
		}
		return out[i].Amount < out[j].Amount
	})

	return out
}

// ── transaction parsers ───────────────────────────────────────────────────────

func (s *SuiIndexer) transferTransactionsFromBalanceChanges(base types.Transaction, deltas []suiBalanceDelta) []types.Transaction {
	out := make([]types.Transaction, 0, len(deltas))
	for i, delta := range deltas {
		if delta.Amount.Sign() <= 0 {
			continue
		}
		tx := base
		if senderDelta, ok := biggestNegativeDeltaByCoinType(deltas, delta.CoinType); ok {
			tx.FromAddress = senderDelta.Address
		}
		tx.ToAddress = delta.Address
		tx.Amount = delta.Amount.String()
		tx.Type, tx.AssetAddress = classifySuiTransferType(delta.CoinType)
		tx.TransferIndex = fmt.Sprintf("balance:%d", i)
		out = append(out, tx)
	}
	return out
}

func (s *SuiIndexer) eventTransactions(base types.Transaction, execTx *v2.ExecutedTransaction) []types.Transaction {
	if execTx.GetEvents() == nil {
		return nil
	}

	var out []types.Transaction
	eventIdx := 0
	for _, evt := range execTx.GetEvents().GetEvents() {
		if evt == nil {
			continue
		}
		eventType := normalizeSuiIdentifier(evt.GetEventType())
		data := eventJSONMap(evt)

		switch {
		case isSuiValidatorStakeEvent(evt):
			tx := base
			tx.TransferIndex = fmt.Sprintf("event:%d", eventIdx)
			tx.ToAddress = jsonString(data, "validator_address", "validator", "pool_id")
			if tx.ToAddress == "" {
				tx.ToAddress = base.FromAddress
			}
			tx.AssetAddress = suiNativeCoinType
			tx.Amount = eventAmountString(data)
			normalizeSuiMovementType(&tx)
			out = append(out, tx)
			eventIdx++

		case isSuiValidatorUnstakeEvent(evt):
			tx := base
			tx.TransferIndex = fmt.Sprintf("event:%d", eventIdx)
			tx.ToAddress = base.FromAddress
			tx.AssetAddress = suiNativeCoinType
			tx.Amount = eventAmountString(data)
			normalizeSuiMovementType(&tx)
			out = append(out, tx)
			eventIdx++

		case isSuiSwapEvent(evt):
			tx := base
			tx.TransferIndex = fmt.Sprintf("event:%d", eventIdx)
			tx.ToAddress = base.FromAddress
			tx.AssetAddress = eventCoinType(evt, data)
			tx.Amount = eventAmountString(data)
			normalizeSuiMovementType(&tx)
			out = append(out, tx)
			eventIdx++

		case isSuiUSDCEvent(evt):
			// Circle's native USDC emits structured treasury events with explicit
			// sender/recipient fields. Parsing these directly gives us the true
			// economic parties even for sponsored transactions where the tx signer
			// is a relayer rather than the actual token holder.
			tx := base
			tx.TransferIndex = fmt.Sprintf("event:%d", eventIdx)
			tx.FromAddress = jsonString(data, "sender", "from", "source")
			if tx.FromAddress == "" {
				tx.FromAddress = base.FromAddress
			}
			tx.ToAddress = jsonString(data, "recipient", "to", "receiver", "destination")
			if tx.ToAddress == "" {
				// No recipient encoded in the event — BalanceChanges already
				// captured this transfer, so skip to avoid a phantom duplicate.
				continue
			}
			tx.AssetAddress = suiUSDCCoinType
			tx.Amount = eventAmountString(data)
			tx.Type = constant.TxTypeTokenTransfer
			out = append(out, tx)
			eventIdx++

		case strings.Contains(eventType, "::transfer"):
			tx := base
			tx.TransferIndex = fmt.Sprintf("event:%d", eventIdx)
			tx.FromAddress = jsonString(data, "from", "sender")
			if tx.FromAddress == "" {
				tx.FromAddress = base.FromAddress
			}
			tx.ToAddress = jsonString(data, "to", "recipient", "receiver", "owner")
			if tx.ToAddress == "" {
				continue
			}
			tx.AssetAddress = eventCoinType(evt, data)
			tx.Amount = eventAmountString(data)
			normalizeSuiMovementType(&tx)
			out = append(out, tx)
			eventIdx++
		}
	}

	return out
}

// ── constructor ───────────────────────────────────────────────────────────────

func NewSuiIndexer(chainName string, cfg config.ChainConfig, f *rpc.Failover[sui.SuiAPI], pubkeyStore PubkeyStore) *SuiIndexer {
	s := &SuiIndexer{
		chainName:   chainName,
		cfg:         cfg,
		failover:    f,
		pubkeyStore: pubkeyStore,
	}

	// Start streaming in background on best available node.
	go func() {
		time.Sleep(1 * time.Second)
		_ = s.failover.ExecuteWithRetry(context.Background(), func(c sui.SuiAPI) error {
			return c.StartStreaming(context.Background())
		})
	}()

	return s
}

// ── Indexer interface ─────────────────────────────────────────────────────────

func (s *SuiIndexer) GetName() string                  { return strings.ToUpper(s.chainName) }
func (s *SuiIndexer) GetNetworkType() enum.NetworkType { return enum.NetworkTypeSui }
func (s *SuiIndexer) GetNetworkInternalCode() string   { return s.cfg.InternalCode }

func (s *SuiIndexer) isMonitoredAddress(addr string) bool {
	if s.pubkeyStore == nil {
		return true
	}
	candidates := []string{addr, normalizeSuiAddress(addr)}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if s.pubkeyStore.Exist(enum.NetworkTypeSui, candidate) {
			return true
		}
	}
	return false
}

func (s *SuiIndexer) isMonitoredTransfer(from, to string) bool {
	if s.pubkeyStore == nil {
		return true
	}
	if s.isMonitoredAddress(to) {
		return true
	}
	return s.cfg.TwoWayIndexing && s.isMonitoredAddress(from)
}

// finalityLag returns the configured checkpoint lag, falling back to the
// package-level default when the operator hasn't set confirmations in YAML.
func (s *SuiIndexer) finalityLag() uint64 {
	if s.cfg.Confirmations > 0 {
		return s.cfg.Confirmations
	}
	return suiDefaultFinalityLag
}

// GetLatestBlockNumber returns the latest checkpoint sequence number.
func (s *SuiIndexer) GetLatestBlockNumber(ctx context.Context) (uint64, error) {
	timeout := s.cfg.Client.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var latest uint64
	err := s.failover.ExecuteWithRetry(ctx, func(c sui.SuiAPI) error {
		n, err := c.GetLatestCheckpointSequence(ctx)
		latest = n
		return err
	})
	return latest, err
}

// GetBlock fetches a single checkpoint and converts it into the generic Block type.
//
// Finality: we fetch the latest checkpoint sequence alongside the target so we
// can compute Confirmations and Status on every transaction. The extra call is
// cheap — served from the streaming buffer on connected nodes.
func (s *SuiIndexer) GetBlock(ctx context.Context, number uint64) (*types.Block, error) {
	var cp *sui.Checkpoint
	err := s.failover.ExecuteWithRetry(ctx, func(c sui.SuiAPI) error {
		var err error
		cp, err = c.GetCheckpoint(ctx, number)
		return err
	})

	// If the checkpoint is not found but should exist (based on latest height),
	// wait briefly and retry once to handle public node propagation lag.
	if err != nil && strings.Contains(err.Error(), "not found") {
		latest, _ := s.GetLatestBlockNumber(ctx)
		if latest > 0 && number <= latest {
			time.Sleep(500 * time.Millisecond)
			_ = s.failover.ExecuteWithRetry(ctx, func(c sui.SuiAPI) error {
				var retryErr error
				cp, retryErr = c.GetCheckpoint(ctx, number)
				return retryErr
			})
			if cp != nil {
				err = nil
			}
		}
	}

	if err != nil {
		return nil, fmt.Errorf("get sui checkpoint %d failed: %w", number, err)
	}
	if cp == nil {
		return nil, fmt.Errorf("sui checkpoint %d not found", number)
	}

	// Fetch current tip for finality computation. Non-fatal on failure —
	// computeSuiFinality degrades gracefully when latestSeq is 0.
	latestSeq, _ := s.GetLatestBlockNumber(ctx)

	return s.convertCheckpoint(cp, latestSeq), nil
}

// GetBlocks fetches a contiguous range of checkpoints.
func (s *SuiIndexer) GetBlocks(
	ctx context.Context,
	from, to uint64,
	isParallel bool,
) ([]BlockResult, error) {
	if to < from {
		return nil, fmt.Errorf("invalid range: from %d > to %d", from, to)
	}
	nums := make([]uint64, 0, to-from+1)
	for n := from; n <= to; n++ {
		nums = append(nums, n)
	}
	return s.GetBlocksByNumbers(ctx, nums)
}

// GetBlocksByNumbers fetches checkpoints by sequence numbers concurrently.
func (s *SuiIndexer) GetBlocksByNumbers(
	ctx context.Context,
	blockNumbers []uint64,
) ([]BlockResult, error) {
	if len(blockNumbers) == 0 {
		return nil, nil
	}

	workers := s.cfg.Throttle.Concurrency
	if workers <= 0 {
		workers = 10
	}
	if workers > 50 {
		workers = 50
	}
	sem := semaphore.NewWeighted(int64(workers))

	resultsMap := make(map[uint64]BlockResult)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, num := range blockNumbers {
		if err := sem.Acquire(ctx, 1); err != nil {
			return nil, err
		}
		wg.Add(1)

		go func(blockNum uint64) {
			defer sem.Release(1)
			defer wg.Done()

			blk, err := s.GetBlock(ctx, blockNum)
			res := BlockResult{Number: blockNum}
			if err != nil {
				res.Error = &Error{
					ErrorType: ErrorTypeUnknown,
					Message:   err.Error(),
				}
			} else {
				res.Block = blk
			}

			mu.Lock()
			resultsMap[blockNum] = res
			mu.Unlock()
		}(num)
	}

	wg.Wait()

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	results := make([]BlockResult, 0, len(blockNumbers))
	var firstErr error
	for _, num := range blockNumbers {
		if res, ok := resultsMap[num]; ok {
			results = append(results, res)
			if firstErr == nil && res.Error != nil {
				firstErr = fmt.Errorf("block %d: %s", res.Number, res.Error.Message)
			}
		}
	}
	return results, firstErr
}

// IsHealthy does a quick gRPC health check by asking for the latest checkpoint.
func (s *SuiIndexer) IsHealthy() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := s.GetLatestBlockNumber(ctx)
	return err == nil
}

// ── checkpoint → Block conversion ─────────────────────────────────────────────

// convertCheckpoint maps a Sui checkpoint into the generic Block representation.
//
// latestSeq is the current chain tip, used to derive Confirmations and Status
// for every transaction in this checkpoint.
func (s *SuiIndexer) convertCheckpoint(cp *sui.Checkpoint, latestSeq uint64) *types.Block {
	ts := cp.TimestampMs() / 1000

	// Compute finality once — all txs in a checkpoint share the same depth.
	finality := computeSuiFinality(cp, latestSeq, s.finalityLag())

	txs := make([]types.Transaction, 0, len(cp.GetTransactions()))
	for _, execTx := range cp.GetTransactions() {
		for _, tx := range s.convertTransactions(execTx, cp.SequenceNumber(), ts) {
			if tx.ToAddress == "" {
				continue
			}
			if !s.isMonitoredTransfer(tx.FromAddress, tx.ToAddress) {
				continue
			}
			tx.BlockHash = cp.Digest()
			tx.Confirmations = finality.confirmations
			tx.Status = finality.status
			txs = append(txs, tx)
		}
	}

	return &types.Block{
		Number:       cp.SequenceNumber(),
		Hash:         cp.Digest(),
		ParentHash:   cp.PreviousDigest(),
		Timestamp:    ts,
		Transactions: txs,
	}
}

func (s *SuiIndexer) convertTransaction(execTx *v2.ExecutedTransaction, blockNumber, blockTs uint64) types.Transaction {
	txs := s.convertTransactions(execTx, blockNumber, blockTs)
	if len(txs) == 0 {
		return types.Transaction{}
	}
	return txs[0]
}

func (s *SuiIndexer) convertTransactions(execTx *v2.ExecutedTransaction, blockNumber, blockTs uint64) []types.Transaction {
	base := makeSuiBaseTransaction(execTx, s.cfg.InternalCode, blockNumber, blockTs)
	if base.FromAddress == "" || isSuiSystemSender(base.FromAddress) {
		return nil
	}

	deltas := parseSuiBalanceChanges(execTx.GetBalanceChanges())
	var out []types.Transaction

	out = append(out, s.transferTransactionsFromBalanceChanges(base, deltas)...)

	moveCalls := commandSummary(execTx.GetTransaction())
	moveIdx := 0
	for _, mc := range moveCalls {
		eventType := classifyMoveCall(mc)
		if eventType == "" {
			continue
		}
		tx := base
		tx.TransferIndex = fmt.Sprintf("move:%d", moveIdx)
		tx.ToAddress = base.FromAddress
		tx.Amount = "0"

		switch eventType {
		case suiMoveCallSwap:
			if delta, ok := biggestPositiveDelta(deltas, ""); ok {
				tx.ToAddress = base.FromAddress
				tx.Amount = delta.Amount.String()
				tx.AssetAddress = delta.CoinType
			}
		case suiMoveCallStake, suiMoveCallUnstake:
			if delta, ok := biggestPositiveDelta(deltas, ""); ok {
				tx.AssetAddress = delta.CoinType
				tx.Amount = delta.Amount.String()
			}
		}

		normalizeSuiMovementType(&tx)
		out = append(out, tx)
		moveIdx++
	}

	out = append(out, s.eventTransactions(base, execTx)...)

	return uniqueTransactions(out)
}
