package workers

import (
	"testing"

	"github.com/fystack/b2b-merchant/internal/queue"
)

func TestRouteToChainQueue(t *testing.T) {
	tests := []struct {
		chain     queue.ChainType
		wantQueue string
		wantErr   bool
	}{
		{queue.ChainSui, queue.QueueSuiTreasury, false},
		{queue.ChainSolana, queue.QueueSolanaTreasury, false},
		{queue.ChainBase, queue.QueueBaseTreasury, false},
		{"unknown", "", true},
		{"", "", true},
		{"ethereum", "", true},
	}
	for _, tc := range tests {
		got, err := routeToChainQueue(tc.chain)
		if tc.wantErr {
			if err == nil {
				t.Errorf("routeToChainQueue(%q): expected error, got none", tc.chain)
			}
		} else {
			if err != nil {
				t.Errorf("routeToChainQueue(%q): unexpected error: %v", tc.chain, err)
			}
			if got != tc.wantQueue {
				t.Errorf("routeToChainQueue(%q) = %q, want %q", tc.chain, got, tc.wantQueue)
			}
		}
	}
}

func TestRouteToChainQueue_AllChainsHaveDistinctQueues(t *testing.T) {
	chains := []queue.ChainType{queue.ChainSui, queue.ChainSolana, queue.ChainBase}
	seen := make(map[string]queue.ChainType)
	for _, c := range chains {
		q, err := routeToChainQueue(c)
		if err != nil {
			t.Fatalf("routeToChainQueue(%q): %v", c, err)
		}
		if prev, ok := seen[q]; ok {
			t.Errorf("chains %q and %q map to the same queue %q", prev, c, q)
		}
		seen[q] = c
	}
}
