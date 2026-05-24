package scrub

import (
	"context"
	"log"
	"sync"

	"github.com/zevlion/vaultron/internal/ec"
	"github.com/zevlion/vaultron/internal/store"
	"github.com/zevlion/vaultron/pkg/hash"
)

// RepairJob describes a single object that needs shard reconstruction.
type RepairJob struct {
	Meta      store.ObjectMeta
	BadShards []int
}

// Worker consumes RepairJobs from jobs, reconstructs corrupted shards,
// and writes the repaired shards back to disk.
type Worker struct {
	st  *store.Store
	enc *ec.Encoder
}

// NewWorkerPool launches n repair workers and returns them.
// All workers share the same jobs channel.
func NewWorkerPool(
	ctx context.Context,
	n int,
	st *store.Store,
	enc *ec.Encoder,
	jobs <-chan RepairJob,
) *sync.WaitGroup {
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := &Worker{st: st, enc: enc}
			w.run(ctx, jobs)
		}()
	}
	return &wg
}

func (w *Worker) run(ctx context.Context, jobs <-chan RepairJob) {
	for {
		select {
		case job, ok := <-jobs:
			if !ok {
				return
			}
			if err := w.repair(job); err != nil {
				log.Printf("repair worker: %s: %v", job.Meta.ContentHash, err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (w *Worker) repair(job RepairJob) error {
	meta := job.Meta
	total := meta.DataShards + meta.ParityShards

	log.Printf("repair: starting %s (bad shards: %v)", meta.ContentHash, job.BadShards)

	// Load all shards; missing/bad ones come back as nil.
	shards, err := store.ReadAllShards(w.st.DataDir(), meta.ContentHash, total)
	if err != nil {
		return err
	}

	// Nil out the shards we know are bad so Reconstruct treats them as lost.
	ec.NilBadShards(shards, job.BadShards)

	// Reconstruct in-place.
	repaired, err := w.enc.Repair(shards)
	if err != nil {
		return err
	}

	// Write back only the shards that were bad.
	for _, i := range job.BadShards {
		if err := store.ReplaceShardAtomic(w.st.DataDir(), meta.ContentHash, i, repaired[i]); err != nil {
			return err
		}
		log.Printf("repair: wrote shard %d for %s", i, meta.ContentHash)
	}

	// Update the index checksums after repair.
	checksums := make([]string, total)
	for i, s := range repaired {
		checksums[i] = hash.OfBytes(s)
	}
	meta.ShardChecksums = checksums
	return w.st.UpdateMeta(meta)
}
