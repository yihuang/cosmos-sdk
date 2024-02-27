package simapp

import (
	"context"
	"io"
	"sync/atomic"

	"cosmossdk.io/store/cachekv"
	"cosmossdk.io/store/cachemulti"
	"cosmossdk.io/store/tracekv"
	"cosmossdk.io/store/types"
	storetypes "cosmossdk.io/store/types"
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	block_stm "github.com/yihuang/go-block-stm"
)

func STMTxExecutor(stores []storetypes.StoreKey, workers int) baseapp.TxExecutor {
	return func(
		ctx context.Context,
		txs [][]byte,
		ms storetypes.MultiStore,
		deliverTxWithMultiStore func([]byte, storetypes.MultiStore) *abci.ExecTxResult,
	) ([]*abci.ExecTxResult, error) {
		atomicResults := make([]atomic.Pointer[abci.ExecTxResult], len(txs))
		if err := block_stm.ExecuteBlock(
			ctx,
			len(txs),
			stores,
			stmMultiStoreWrapper{ms},
			workers,
			func(txn block_stm.TxnIndex, ms block_stm.MultiStore) {
				result := deliverTxWithMultiStore(txs[txn], newMultiStoreWrapper(ms, stores))
				atomicResults[txn].Store(result)
			},
		); err != nil {
			return nil, err
		}
		results := make([]*abci.ExecTxResult, len(txs))
		for i := range atomicResults {
			results[i] = atomicResults[i].Load()
		}
		return results, nil
	}
}

type storeWrapper struct {
	block_stm.KVStore
}

var (
	_ storetypes.Store   = storeWrapper{}
	_ storetypes.KVStore = storeWrapper{}
)

func (s storeWrapper) GetStoreType() storetypes.StoreType {
	return storetypes.StoreTypeIAVL
}

func (s storeWrapper) CacheWrap() storetypes.CacheWrap {
	return cachekv.NewStore(storetypes.KVStore(s))
}

func (s storeWrapper) CacheWrapWithTrace(w io.Writer, tc storetypes.TraceContext) storetypes.CacheWrap {
	return cachekv.NewStore(tracekv.NewStore(s, w, tc))
}

type msWrapper struct {
	block_stm.MultiStore
	stores     []storetypes.StoreKey
	keysByName map[string]storetypes.StoreKey
}

var _ storetypes.MultiStore = msWrapper{}

func newMultiStoreWrapper(ms block_stm.MultiStore, stores []storetypes.StoreKey) msWrapper {
	keysByName := make(map[string]storetypes.StoreKey)
	for _, k := range stores {
		keysByName[k.Name()] = k
	}
	return msWrapper{ms, stores, keysByName}
}

func (ms msWrapper) GetStore(key storetypes.StoreKey) storetypes.Store {
	return storetypes.Store(ms.GetKVStore(key))
}

func (ms msWrapper) GetKVStore(key storetypes.StoreKey) storetypes.KVStore {
	return storeWrapper{ms.MultiStore.GetKVStore(key)}
}

func (ms msWrapper) CacheMultiStore() storetypes.CacheMultiStore {
	stores := make(map[storetypes.StoreKey]storetypes.CacheWrapper)
	for _, k := range ms.stores {
		store := ms.GetKVStore(k)
		stores[k] = store
	}
	return cachemulti.NewStore(nil, stores, ms.keysByName, nil, nil)
}

func (ms msWrapper) CacheMultiStoreWithVersion(_ int64) (storetypes.CacheMultiStore, error) {
	panic("cannot branch cached multi-store with a version")
}

// Implements CacheWrapper.
func (ms msWrapper) CacheWrap() storetypes.CacheWrap {
	return ms.CacheMultiStore().(storetypes.CacheWrap)
}

// CacheWrapWithTrace implements the CacheWrapper interface.
func (ms msWrapper) CacheWrapWithTrace(_ io.Writer, _ storetypes.TraceContext) storetypes.CacheWrap {
	return ms.CacheWrap()
}

// GetStoreType returns the type of the store.
func (ms msWrapper) GetStoreType() storetypes.StoreType {
	return storetypes.StoreTypeMulti
}

// LatestVersion returns the branch version of the store
func (ms msWrapper) LatestVersion() int64 {
	panic("cannot get latest version from branch cached multi-store")
}

// Implements interface MultiStore
func (ms msWrapper) SetTracer(w io.Writer) types.MultiStore {
	return nil
}

// Implements interface MultiStore
func (ms msWrapper) SetTracingContext(types.TraceContext) types.MultiStore {
	return nil
}

// Implements interface MultiStore
func (ms msWrapper) TracingEnabled() bool {
	return false
}

type stmMultiStoreWrapper struct {
	inner storetypes.MultiStore
}

var _ block_stm.MultiStore = stmMultiStoreWrapper{}

func (ms stmMultiStoreWrapper) GetKVStore(key storetypes.StoreKey) block_stm.KVStore {
	return block_stm.KVStore(ms.inner.GetKVStore(key))
}
