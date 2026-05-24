package ec

import (
	"fmt"

	"github.com/klauspost/reedsolomon"
)

// Repair reconstructs missing or corrupted shards in-place.
// Pass nil for any shard that is missing or known-bad.
// Returns the fully reconstructed shard set.
func (e *Encoder) Repair(shards [][]byte) ([][]byte, error) {
	if len(shards) != e.Total() {
		return nil, fmt.Errorf("expected %d shards, got %d", e.Total(), len(shards))
	}

	if err := e.rs.Reconstruct(shards); err != nil {
		if err == reedsolomon.ErrTooFewShards {
			return nil, fmt.Errorf("too many shards lost to repair: need at least %d", e.dataShards)
		}
		return nil, fmt.Errorf("reconstruct: %w", err)
	}

	// Verify the reconstruction is consistent before the caller writes it back.
	ok, err := e.rs.Verify(shards)
	if err != nil {
		return nil, fmt.Errorf("post-repair verify: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("post-repair verify failed: shard set is inconsistent")
	}

	return shards, nil
}

// NilBadShards sets shards[i] = nil for every index in badIndices.
// Call this before Repair when the scrubber identifies specific corrupt shards.
func NilBadShards(shards [][]byte, badIndices []int) {
	for _, i := range badIndices {
		shards[i] = nil
	}
}
