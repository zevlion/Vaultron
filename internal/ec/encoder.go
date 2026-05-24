package ec

import (
	"fmt"

	"github.com/klauspost/reedsolomon"
)

// Encoder wraps a Reed-Solomon codec for a fixed (k, m) geometry.
type Encoder struct {
	dataShards   int
	parityShards int
	rs           reedsolomon.Encoder
}

// NewEncoder creates an Encoder for k data shards and m parity shards.
func NewEncoder(dataShards, parityShards int) *Encoder {
	rs, err := reedsolomon.New(dataShards, parityShards)
	if err != nil {
		// Invalid parameters are a programming error, not a runtime one.
		panic(fmt.Sprintf("reedsolomon.New(%d, %d): %v", dataShards, parityShards, err))
	}
	return &Encoder{
		dataShards:   dataShards,
		parityShards: parityShards,
		rs:           rs,
	}
}

// Encode splits data into k data shards and computes m parity shards.
// Returns a slice of (k+m) byte slices — data shards first, parity after.
func (e *Encoder) Encode(data []byte) ([][]byte, error) {
	shards, err := e.rs.Split(data)
	if err != nil {
		return nil, fmt.Errorf("split: %w", err)
	}
	if err := e.rs.Encode(shards); err != nil {
		return nil, fmt.Errorf("encode parity: %w", err)
	}
	return shards, nil
}

// Verify checks that all shards are internally consistent.
// Returns nil if the shard set is intact.
func (e *Encoder) Verify(shards [][]byte) (bool, error) {
	return e.rs.Verify(shards)
}

// DataShards returns k.
func (e *Encoder) DataShards() int { return e.dataShards }

// ParityShards returns m.
func (e *Encoder) ParityShards() int { return e.parityShards }

// Total returns k+m.
func (e *Encoder) Total() int { return e.dataShards + e.parityShards }