package version

import (
	"bytes"

	"github.com/RoaringBitmap/roaring/roaring64"

	sdk "github.com/cosmos/cosmos-sdk/types"
	dbm "github.com/tendermint/tm-db"
)

const LastChunkId = ^uint64(0)

func HistoryIndexKey(key []byte, height uint64) []byte {
	return append(key, sdk.Uint64ToBigEndian(height)...)
}

// GetHistoryIndex returns the history index bitmap chunk which covers the target version.
func GetHistoryIndex(db dbm.DB, key []byte, height uint64) (*roaring64.Bitmap, error) {
	// try to seek the first chunk whose maximum is bigger or equal to the target height.
	it, err := db.Iterator(
		HistoryIndexKey(key, height),
		sdk.PrefixEndBytes(key),
	)
	if err != nil {
		return nil, err
	}
	defer it.Close() // nolint: errcheck

	if !it.Valid() {
		return nil, nil
	}

	m := roaring64.New()
	_, err = m.ReadFrom(bytes.NewReader(it.Value()))
	if err != nil {
		return nil, err
	}
	return m, nil
}

// SeekHistoryIndex locate the minimal version that changed the key and is larger than the target version,
// using the returned version can find the value for the target version in changeset table.
// If not found, return -1
func SeekHistoryIndex(db dbm.DB, key []byte, version uint64) (int64, error) {
	// either m.Maximum() >= version + 1, or is the last chunk.
	m, err := GetHistoryIndex(db, key, version+1)
	if err != nil {
		return -1, err
	}
	found, ok := SeekInBitmap64(m, version+1)
	if !ok {
		return -1, nil
	}
	return int64(found), nil
}

// WriteHistoryIndex set the block height to the history bitmap.
// it try to set to the last chunk, if the last chunk exceeds chunk limit, split it.
func WriteHistoryIndex(db dbm.DB, batch dbm.Batch, key []byte, height uint64) error {
	lastKey := HistoryIndexKey(key, LastChunkId)
	bz, err := db.Get(lastKey)
	if err != nil {
		return err
	}

	m := roaring64.New()
	if len(bz) > 0 {
		_, err = m.ReadFrom(bytes.NewReader(bz))
		if err != nil {
			return err
		}
	}
	m.Add(height)

	// chunking
	if err = WalkChunks64(m, ChunkLimit, func(chunk *roaring64.Bitmap, isLast bool) error {
		chunkKey := lastKey
		if !isLast {
			chunkKey = HistoryIndexKey(key, chunk.Maximum())
		}
		bz, err := chunk.ToBytes()
		if err != nil {
			return err
		}
		return batch.Set(chunkKey, bz)
	}); err != nil {
		return err
	}

	return nil
}
