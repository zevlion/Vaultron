package store

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/zevlion/vaultron/pkg/hash"
)

// shardPath returns the absolute path for shard i of contentHash.
// Layout: <dataDir>/<l1>/<l2>/<hash>.<i>.shard
func shardPath(dataDir, contentHash string, i int) string {
	l1, l2 := hash.ShardDir(contentHash)
	return filepath.Join(dataDir, l1, l2, fmt.Sprintf("%s.%d.shard", contentHash, i))
}

// ShardPath is the public accessor used by the scrubber.
func ShardPath(dataDir, contentHash string, i int) string {
	return shardPath(dataDir, contentHash, i)
}

func writeShards(dataDir, contentHash string, shards [][]byte) error {
	l1, l2 := hash.ShardDir(contentHash)
	dir := filepath.Join(dataDir, l1, l2)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	for i, shard := range shards {
		if err := writeShardAtomic(dataDir, contentHash, i, shard); err != nil {
			return err
		}
	}
	return nil
}

func writeShardAtomic(dataDir, contentHash string, i int, data []byte) error {
	final := shardPath(dataDir, contentHash, i)
	tmp := final + ".tmp"

	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open tmp shard: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("write shard %d: %w", i, err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("sync shard %d: %w", i, err)
	}
	f.Close()
	return os.Rename(tmp, final)
}

// ReplaceShardAtomic overwrites a single shard — used by the repair worker.
func ReplaceShardAtomic(dataDir, contentHash string, i int, data []byte) error {
	return writeShardAtomic(dataDir, contentHash, i, data)
}

func readShard(dataDir, contentHash string, i int) ([]byte, error) {
	path := shardPath(dataDir, contentHash, i)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read shard %d: %w", i, err)
	}
	return data, nil
}

// ReadAllShards loads every shard for contentHash into a slice.
// Missing files produce nil entries so the EC layer can reconstruct.
func ReadAllShards(dataDir, contentHash string, total int) ([][]byte, error) {
	shards := make([][]byte, total)
	for i := range total {
		s, err := readShard(dataDir, contentHash, i)
		if err != nil {
			return nil, err
		}
		shards[i] = s
	}
	return shards, nil
}

func deleteShards(dataDir, contentHash string, total int) error {
	for i := range total {
		path := shardPath(dataDir, contentHash, i)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("delete shard %d: %w", i, err)
		}
	}
	return nil
}

func streamObject(dataDir, contentHash string, dataShards int, size int64, w io.Writer) error {
	remaining := size
	for i := range dataShards {
		shard, err := readShard(dataDir, contentHash, i)
		if err != nil {
			return fmt.Errorf("read data shard %d: %w", i, err)
		}
		if shard == nil {
			return fmt.Errorf("data shard %d is missing", i)
		}
		toWrite := min(int64(len(shard)), remaining)
		if _, err := w.Write(shard[:toWrite]); err != nil {
			return fmt.Errorf("stream shard %d: %w", i, err)
		}
		remaining -= toWrite
	}
	return nil
}
