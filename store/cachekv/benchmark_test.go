package cachekv_test

import (
	fmt "fmt"
	"testing"

	"github.com/cosmos/cosmos-sdk/store"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	dbm "github.com/tendermint/tm-db"
)

func DoBenchmarkDeepContextStack(b *testing.B, depth int) {
	key := storetypes.NewKVStoreKey("test")

	db := dbm.NewMemDB()
	cms := store.NewCommitMultiStore(db)
	cms.MountStoreWithDB(key, storetypes.StoreTypeIAVL, db)
	cms.LoadLatestVersion()

	nItems := 20
	store := cms.GetKVStore(key)
	for i := 0; i < nItems; i++ {
		store.Set([]byte(fmt.Sprintf("hello%03d", i)), []byte("world"))
	}
	cms.Commit()

	ms := cms.CacheMultiStore()
	ctx := sdk.NewContext(ms, tmproto.Header{}, false, log.NewNopLogger())

	var stack ContextStack
	stack.Reset(ctx)

	for i := 0; i < depth; i++ {
		stack.Snapshot()

		store := stack.Context().KVStore(key)
		store.Set([]byte(fmt.Sprintf("hello%03d", i)), []byte("modified"))
	}

	store = stack.Context().KVStore(key)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		it := store.Iterator(nil, nil)
		items := make([][]byte, 0, nItems)
		for ; it.Valid(); it.Next() {
			items = append(items, it.Key())
			it.Value()
		}
		it.Close()
		require.Equal(b, nItems, len(items))
	}
}

func BenchmarkDeepContextStack1(b *testing.B) {
	DoBenchmarkDeepContextStack(b, 1)
}

func BenchmarkDeepContextStack3(b *testing.B) {
	DoBenchmarkDeepContextStack(b, 3)
}
func BenchmarkDeepContextStack10(b *testing.B) {
	DoBenchmarkDeepContextStack(b, 10)
}

func BenchmarkDeepContextStack13(b *testing.B) {
	DoBenchmarkDeepContextStack(b, 13)
}

// ContextStack manages the initial context and a stack of cached contexts,
// to support the `StateDB.Snapshot` and `StateDB.RevertToSnapshot` methods.
//
// Copied from an old version of ethermint
type ContextStack struct {
	// Context of the initial state before transaction execution.
	// It's the context used by `StateDB.CommitedState`.
	ctx       sdk.Context
	snapshots []storetypes.CacheMultiStore
}

// CurrentContext returns the top context of cached stack,
// if the stack is empty, returns the initial context.
func (cs *ContextStack) Context() sdk.Context {
	return cs.ctx
}

// Reset sets the initial context and clear the cache context stack.
func (cs *ContextStack) Reset(ctx sdk.Context) {
	cs.ctx = ctx
	cs.snapshots = []storetypes.CacheMultiStore{}
}

// IsEmpty returns true if the cache context stack is empty.
func (cs *ContextStack) IsEmpty() bool {
	return len(cs.snapshots) == 0
}

// Snapshot pushes a new cached context to the stack,
// and returns the index of it.
func (cs *ContextStack) Snapshot() int {
	cs.snapshots = append(cs.snapshots, cs.ctx.MultiStore().Clone())
	return len(cs.snapshots) - 1
}

// RevertToSnapshot pops all the cached contexts after the target index (inclusive).
// the target should be snapshot index returned by `Snapshot`.
// This function panics if the index is out of bounds.
func (cs *ContextStack) RevertToSnapshot(target int) {
	if target < 0 || target >= len(cs.snapshots) {
		panic(fmt.Errorf("snapshot index %d out of bound [%d..%d)", target, 0, len(cs.snapshots)))
	}
	cs.ctx.MultiStore().Restore(cs.snapshots[target])
	cs.snapshots = cs.snapshots[:target]
}

// RevertAll discards all the cache contexts.
func (cs *ContextStack) RevertAll() {
	if len(cs.snapshots) > 0 {
		cs.RevertToSnapshot(0)
	}
}
