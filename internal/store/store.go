package store

import (
	"errors"
	"fmt"
	"io"

	"github.com/zevlion/vaultron/config"
	"github.com/zevlion/vaultron/internal/ec"
	internalhash "github.com/zevlion/vaultron/pkg/hash"
)

// ErrNotFound is returned when an object does not exist in the index.
var ErrNotFound = errors.New("object not found")

// Store is the top-level object storage engine.
type Store struct {
	cfg config.Config
	idx *index
	enc *ec.Encoder
}

// Open initialises the store, creating directories and opening the index.
func Open(cfg config.Config) (*Store, error) {
	idx, err := openIndex(cfg.IndexDir)
	if err != nil {
		return nil, fmt.Errorf("open index: %w", err)
	}
	encoder := ec.NewEncoder(cfg.EC.DataShards, cfg.EC.ParityShards)
	return &Store{cfg: cfg, idx: idx, enc: encoder}, nil
}

// Close flushes and closes the index.
func (s *Store) Close() error {
	return s.idx.close()
}

// Put reads all of r, erasure-codes it, persists shards, and returns the content hash.
func (s *Store) Put(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	contentHash := internalhash.OfBytes(data)

	// Idempotent: already stored.
	if _, err := s.idx.get(contentHash); err == nil {
		return contentHash, nil
	}

	shards, err := s.enc.Encode(data)
	if err != nil {
		return "", fmt.Errorf("erasure encode: %w", err)
	}

	if err := writeShards(s.cfg.DataDir, contentHash, shards); err != nil {
		return "", fmt.Errorf("write shards: %w", err)
	}

	checksums := make([]string, len(shards))
	for i, shard := range shards {
		checksums[i] = internalhash.OfBytes(shard)
	}

	meta := ObjectMeta{
		ContentHash:    contentHash,
		Size:           int64(len(data)),
		DataShards:     s.cfg.EC.DataShards,
		ParityShards:   s.cfg.EC.ParityShards,
		ShardChecksums: checksums,
	}
	if err := s.idx.put(meta); err != nil {
		return "", fmt.Errorf("index put: %w", err)
	}
	return contentHash, nil
}

// Get streams the object identified by contentHash into w.
func (s *Store) Get(contentHash string, w io.Writer) error {
	meta, err := s.idx.get(contentHash)
	if err != nil {
		return err
	}
	return streamObject(s.cfg.DataDir, contentHash, meta.DataShards, meta.Size, w)
}

// Delete removes all shards and the index entry for contentHash.
func (s *Store) Delete(contentHash string) error {
	meta, err := s.idx.get(contentHash)
	if err != nil {
		return err
	}
	total := meta.DataShards + meta.ParityShards
	if err := deleteShards(s.cfg.DataDir, contentHash, total); err != nil {
		return fmt.Errorf("delete shards: %w", err)
	}
	return s.idx.delete(contentHash)
}

// Stat returns metadata for an object without reading its data.
func (s *Store) Stat(contentHash string) (ObjectMeta, error) {
	return s.idx.get(contentHash)
}

// Iterate calls fn for every ObjectMeta in the index (used by the scrubber).
func (s *Store) Iterate(fn func(ObjectMeta) error) error {
	return s.idx.iterate(fn)
}

// UpdateMeta overwrites the index entry for an object (used by the repair worker).
func (s *Store) UpdateMeta(meta ObjectMeta) error {
	return s.idx.put(meta)
}

// DataDir exposes the shard root for the scrubber and repair worker.
func (s *Store) DataDir() string {
	return s.cfg.DataDir
}
