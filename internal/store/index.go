package store

import (
	"encoding/json"
	"fmt"

	badger "github.com/dgraph-io/badger/v4"
)

// ObjectMeta is persisted in BadgerDB for every stored object.
type ObjectMeta struct {
	// ContentHash is the SHA-256 hex digest of the original object data.
	ContentHash string `json:"content_hash"`

	// Size is the original object size in bytes (before padding/sharding).
	Size int64 `json:"size"`

	// DataShards and ParityShards record the EC geometry used at write time.
	DataShards   int `json:"data_shards"`
	ParityShards int `json:"parity_shards"`

	// ShardChecksums holds the SHA-256 hex digest of each shard file
	// (data shards first, then parity). Used by the scrubber.
	ShardChecksums []string `json:"shard_checksums"`
}

// index wraps a BadgerDB instance with typed get/put/delete helpers.
type index struct {
	db *badger.DB
}

func openIndex(dir string) (*index, error) {
	opts := badger.DefaultOptions(dir).
		WithLogger(nil) // suppress Badger's internal logs
	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("open badger at %s: %w", dir, err)
	}
	return &index{db: db}, nil
}

func (idx *index) close() error {
	return idx.db.Close()
}

func (idx *index) put(meta ObjectMeta) error {
	val, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}
	return idx.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(meta.ContentHash), val)
	})
}

func (idx *index) get(contentHash string) (ObjectMeta, error) {
	var meta ObjectMeta
	err := idx.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(contentHash))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &meta)
		})
	})
	if err == badger.ErrKeyNotFound {
		return ObjectMeta{}, ErrNotFound
	}
	return meta, err
}

func (idx *index) delete(contentHash string) error {
	return idx.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(contentHash))
	})
}

// iterate calls fn for every ObjectMeta in the index.
// Used by the scrubber to enumerate all known objects.
func (idx *index) iterate(fn func(ObjectMeta) error) error {
	return idx.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			var meta ObjectMeta
			if err := it.Item().Value(func(val []byte) error {
				return json.Unmarshal(val, &meta)
			}); err != nil {
				return err
			}
			if err := fn(meta); err != nil {
				return err
			}
		}
		return nil
	})
}
