package scrub

import (
	"context"
	"log"
	"time"

	"github.com/zevlion/vaultron/config"
	"github.com/zevlion/vaultron/internal/store"
	"github.com/zevlion/vaultron/pkg/hash"
)

// Scrubber walks every known object on a fixed interval, re-hashing
// each shard and enqueuing repair jobs when a mismatch is detected.
type Scrubber struct {
	cfg     config.ScrubConfig
	st      *store.Store
	repairC chan<- RepairJob
}

// New creates a Scrubber. repairC receives jobs when corruption is found.
func New(cfg config.ScrubConfig, st *store.Store, repairC chan<- RepairJob) *Scrubber {
	return &Scrubber{cfg: cfg, st: st, repairC: repairC}
}

// Run blocks until ctx is cancelled, running scrub passes on the configured interval.
func (s *Scrubber) Run(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.Interval)
	defer ticker.Stop()

	log.Printf("scrubber: starting, interval=%s", s.cfg.Interval)

	// Run an initial pass immediately on startup.
	s.scrubAll(ctx)

	for {
		select {
		case <-ticker.C:
			s.scrubAll(ctx)
		case <-ctx.Done():
			log.Println("scrubber: shutting down")
			return
		}
	}
}

func (s *Scrubber) scrubAll(ctx context.Context) {
	log.Println("scrubber: starting pass")
	var checked, corrupt int

	err := s.st.Iterate(func(meta store.ObjectMeta) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		bad := s.checkObject(meta)
		checked++
		if len(bad) > 0 {
			corrupt++
			select {
			case s.repairC <- RepairJob{Meta: meta, BadShards: bad}:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	})

	if err != nil && ctx.Err() == nil {
		log.Printf("scrubber: pass error: %v", err)
	}
	log.Printf("scrubber: pass complete — %d checked, %d corrupt", checked, corrupt)
}

// checkObject verifies each shard's checksum and returns the indices of bad shards.
func (s *Scrubber) checkObject(meta store.ObjectMeta) []int {
	var bad []int
	total := meta.DataShards + meta.ParityShards

	for i := range total {
		expected := meta.ShardChecksums[i]
		actual, err := hash.OfFile(store.ShardPath(s.st.DataDir(), meta.ContentHash, i))
		if err != nil || actual != expected {
			bad = append(bad, i)
		}
	}
	return bad
}
