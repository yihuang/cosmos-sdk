package version

import (
	"bytes"
	"errors"

	"github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	dbm "github.com/tendermint/tm-db"
)

var _ types.VersionStore = (*Store)(nil)

// Store implements `VersionStore`.
type Store struct {
	// latest key-value pairs
	plainDB dbm.DB
	// history bitmap index of keys
	historyDB dbm.DB
	// changesets of each blocks
	changesetDB dbm.DB
}

func NewStore(plainDB, historyDB, changesetDB dbm.DB) *Store {
	return &Store{plainDB, historyDB, changesetDB}
}

// PutAtVersion implements VersionStore interface
// TODO reduce allocation within iterations.
func (s *Store) PutAtVersion(version int64, changeSet []types.StoreKVPair) error {
	plainBatch := s.plainDB.NewBatch()
	defer plainBatch.Close() // nolint: errcheck
	historyBatch := s.historyDB.NewBatch()
	defer historyBatch.Close() // nolint: errcheck
	changesetBatch := s.changesetDB.NewBatch()
	defer changesetBatch.Close() // nolint: errcheck

	for _, pair := range changeSet {
		key := prependStoreKey(pair.StoreKey, pair.Key)

		if version == 0 {
			// genesis state is written into plain state directly
			if pair.Delete {
				return errors.New("can't delete at genesis")
			} else {
				if err := plainBatch.Set(key, pair.Value); err != nil {
					return err
				}
			}
			continue
		}

		original, err := s.plainDB.Get(key)
		if err != nil {
			return err
		}
		if bytes.Equal(original, pair.Value) {
			// do nothing if the value is not changed
			continue
		}

		// write history index
		if err := WriteHistoryIndex(s.historyDB, historyBatch, key, uint64(version)); err != nil {
			return err
		}

		// write the old value to changeset
		if len(original) > 0 {
			changesetKey := append(sdk.Uint64ToBigEndian(uint64(version)), key...)
			if err := changesetBatch.Set(changesetKey, original); err != nil {
				return err
			}
		}

		// write the new value to plain state
		if pair.Delete {
			if err := plainBatch.Delete(key); err != nil {
				return err
			}
		} else {
			if err := plainBatch.Set(key, pair.Value); err != nil {
				return err
			}
		}
	}

	if err := changesetBatch.WriteSync(); err != nil {
		return err
	}
	if err := historyBatch.WriteSync(); err != nil {
		return err
	}
	return plainBatch.WriteSync()
}

// GetAtVersion implements VersionStore interface
func (s *Store) GetAtVersion(storeKey string, key []byte, version *int64) ([]byte, error) {
	rawKey := prependStoreKey(storeKey, key)
	if version == nil {
		return s.plainDB.Get(rawKey)
	}
	height := uint64(*version)
	found, err := SeekHistoryIndex(s.historyDB, rawKey, height)
	if err != nil {
		return nil, err
	}
	if found < 0 {
		// there's no change records found after the target version, query the latest state.
		return s.plainDB.Get(rawKey)
	}
	// get from changeset
	changesetKey := ChangesetKey(uint64(found), rawKey)
	return s.changesetDB.Get(changesetKey)
}

// HasAtVersion implements VersionStore interface
func (s *Store) HasAtVersion(storeKey string, key []byte, version *int64) (bool, error) {
	rawKey := prependStoreKey(storeKey, key)
	if version == nil {
		return s.plainDB.Has(rawKey)
	}
	height := uint64(*version)
	found, err := SeekHistoryIndex(s.historyDB, rawKey, height)
	if err != nil {
		return false, err
	}
	if found < 0 {
		// there's no change records after the target version, query the latest state.
		return s.plainDB.Has(rawKey)
	}
	// get from changeset
	changesetKey := ChangesetKey(uint64(found), rawKey)
	return s.changesetDB.Has(changesetKey)
}

// IteratorAtVersion implements VersionStore interface
func (s *Store) IteratorAtVersion(storeKey string, start, end []byte, version *int64) types.Iterator {
	// TODO
	return nil
}

// ReverseIteratorAtVersion implements VersionStore interface
func (s *Store) ReverseIteratorAtVersion(storeKey string, start, end []byte, version *int64) types.Iterator {
	// TODO
	return nil
}

// ChangesetKey build key changeset db
func ChangesetKey(version uint64, key []byte) []byte {
	return append(sdk.Uint64ToBigEndian(version), key...)
}

// prependStoreKey prepends storeKey to the key
func prependStoreKey(storeKey string, key []byte) []byte {
	return append([]byte(storeKey), key...)
}
