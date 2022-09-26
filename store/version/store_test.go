package version

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func TestBasics(t *testing.T) {
	var v int64
	key1 := []byte("key1")
	key2 := []byte("key2")
	value1 := []byte("value1")
	value2 := []byte("value2")

	store := NewStore(dbm.NewMemDB(), dbm.NewMemDB(), dbm.NewMemDB())
	require.NoError(t, store.PutAtVersion(0, []types.StoreKVPair{
		{StoreKey: "bank", Key: key1, Value: value1},
		{StoreKey: "bank", Key: key2, Value: value2},
		{StoreKey: "staking", Key: key1, Value: value1},
		{StoreKey: "evm", Key: key1, Value: value1},
	}))
	require.NoError(t, store.PutAtVersion(1, []types.StoreKVPair{
		{StoreKey: "bank", Key: key1, Value: value2},
	}))
	require.NoError(t, store.PutAtVersion(2, []types.StoreKVPair{
		{StoreKey: "staking", Delete: true, Key: key1},
	}))
	require.NoError(t, store.PutAtVersion(3, []types.StoreKVPair{
		{StoreKey: "staking", Key: key1, Value: value2},
	}))

	value, err := store.GetAtVersion("staking", key1, nil)
	require.NoError(t, err)
	require.Equal(t, value, value2)

	v = 2
	ok, err := store.HasAtVersion("staking", key1, &v)
	require.NoError(t, err)
	require.False(t, ok)
	value, err = store.GetAtVersion("staking", key1, &v)
	require.NoError(t, err)
	require.Empty(t, value)

	v = 1
	ok, err = store.HasAtVersion("staking", key1, &v)
	require.NoError(t, err)
	require.True(t, ok)
	value, err = store.GetAtVersion("staking", key1, &v)
	require.NoError(t, err)
	require.Equal(t, value, value1)

	// never changed since genesis
	ok, err = store.HasAtVersion("bank", key2, nil)
	require.NoError(t, err)
	require.True(t, ok)
	value, err = store.GetAtVersion("bank", key2, nil)
	require.NoError(t, err)
	require.Equal(t, value2, value)
	for i := int64(1); i < 4; i++ {
		// never changed
		ok, err = store.HasAtVersion("bank", key2, &i)
		require.NoError(t, err)
		require.True(t, ok)

		value, err = store.GetAtVersion("bank", key2, &i)
		require.NoError(t, err)
		require.Equal(t, value2, value)
	}
}

func TestBitmapChunking(t *testing.T) {
	oldChunkLimit := ChunkLimit
	ChunkLimit = uint64(100)
	key1 := []byte("key1")
	store := NewStore(dbm.NewMemDB(), dbm.NewMemDB(), dbm.NewMemDB())
	for i := 0; i < 100000; i++ {
		require.NoError(t, store.PutAtVersion(uint64(i), []types.StoreKVPair{
			{StoreKey: "bank", Key: key1, Value: sdk.Uint64ToBigEndian(uint64(i))},
		}))
	}
	ChunkLimit = oldChunkLimit
}
