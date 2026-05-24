package config

import "time"

// Config holds all tunables for a Vaultron node.
type Config struct {
	// DataDir is the root directory for all shard files.
	DataDir string

	// IndexDir is where BadgerDB stores its files.
	IndexDir string

	// ListenAddr is the HTTP server bind address.
	ListenAddr string

	// EC holds erasure coding parameters.
	EC ECConfig

	// Scrub holds background scrubber parameters.
	Scrub ScrubConfig
}

// ECConfig controls Reed-Solomon shard geometry.
type ECConfig struct {
	// DataShards is the number of data shards (k).
	DataShards int

	// ParityShards is the number of parity shards (m).
	// Any DataShards-of-(DataShards+ParityShards) can reconstruct the object.
	ParityShards int
}

// ScrubConfig controls the background bitrot scrubber.
type ScrubConfig struct {
	// Interval between full scrub passes.
	Interval time.Duration

	// Workers is the number of concurrent repair goroutines.
	Workers int
}

// Default returns a production-sensible configuration.
func Default() Config {
	return Config{
		DataDir:    "./data/shards",
		IndexDir:   "./data/index",
		ListenAddr: ":8080",
		EC: ECConfig{
			DataShards:   8,
			ParityShards: 4,
		},
		Scrub: ScrubConfig{
			Interval: 4 * time.Hour,
			Workers:  2,
		},
	}
}
