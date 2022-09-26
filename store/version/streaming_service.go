package version

import (
	"sort"
	"strings"
	"sync"

	abci "github.com/tendermint/tendermint/abci/types"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ baseapp.StreamingService = &StreamingService{}

// FlattenListener listens to the state writes and flatten them in memory.
// One listener only listens to a single storeKey.
type FlattenListener struct {
	stateCache map[string]types.StoreKVPair
}

func NewFlattenListener() *FlattenListener {
	return &FlattenListener{
		stateCache: make(map[string]types.StoreKVPair),
	}
}

func (fl *FlattenListener) OnWrite(storeKey types.StoreKey, key []byte, value []byte, delete bool) error {
	fl.stateCache[string(key)] = types.StoreKVPair{
		StoreKey: storeKey.Name(),
		Delete:   delete,
		Key:      key,
		Value:    value,
	}
	return nil
}

// StreamingService is a concrete implementation of StreamingService that accumulate the state changes in current block,
// writes the ordered changeset out to version storage.
type StreamingService struct {
	listeners          map[types.StoreKey]*FlattenListener // the listeners that will be initialized with BaseApp
	versionStore       types.VersionStore
	currentBlockNumber int64 // the current block number
}

// NewStreamingService creates a new StreamingService for the provided writeDir, (optional) filePrefix, and storeKeys
func NewStreamingService(versionStore types.VersionStore, storeKeys []types.StoreKey) *StreamingService {
	listeners := make(map[types.StoreKey]*FlattenListener, len(storeKeys))
	// in this case, we are using the same listener for each Store
	for _, key := range storeKeys {
		listeners[key] = NewFlattenListener()
	}
	return &StreamingService{listeners, versionStore, 0}
}

// Listeners satisfies the baseapp.StreamingService interface
func (fss *StreamingService) Listeners() map[types.StoreKey][]types.WriteListener {
	listeners := make(map[types.StoreKey][]types.WriteListener, len(fss.listeners))
	for storeKey, listener := range fss.listeners {
		listeners[storeKey] = []types.WriteListener{listener}
	}
	return listeners
}

// ListenBeginBlock satisfies the baseapp.ABCIListener interface
// It sets the currentBlockNumber.
func (fss *StreamingService) ListenBeginBlock(ctx sdk.Context, req abci.RequestBeginBlock, res abci.ResponseBeginBlock) error {
	fss.currentBlockNumber = req.GetHeader().Height
	return nil
}

// ListenDeliverTx satisfies the baseapp.ABCIListener interface
func (fss *StreamingService) ListenDeliverTx(ctx sdk.Context, req abci.RequestDeliverTx, res abci.ResponseDeliverTx) error {
	return nil
}

// ListenEndBlock satisfies the baseapp.ABCIListener interface
// It merge the state caches of all the listeners together, and write out to the versionStore.
func (fss *StreamingService) ListenEndBlock(ctx sdk.Context, req abci.RequestEndBlock, res abci.ResponseEndBlock) error {
	// sort by the storeKeys first
	storeKeys := make([]types.StoreKey, 0, len(fss.listeners))
	for storeKey := range fss.listeners {
		storeKeys = append(storeKeys, storeKey)
	}
	sort.SliceStable(storeKeys, func(i, j int) bool {
		return strings.Compare(storeKeys[i].Name(), storeKeys[j].Name()) < 0
	})

	// concat the state caches
	var changeSet []types.StoreKVPair
	for _, storeKey := range storeKeys {
		cache := fss.listeners[storeKey].stateCache
		fss.listeners[storeKey].stateCache = make(map[string]types.StoreKVPair)

		// sort the cache by key
		keys := make([]string, 0, len(cache))
		for key := range cache {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		for _, key := range keys {
			changeSet = append(changeSet, cache[key])
		}
	}

	return fss.versionStore.PutAtVersion(fss.currentBlockNumber, changeSet)
}

// Stream satisfies the baseapp.StreamingService interface
func (fss *StreamingService) Stream(wg *sync.WaitGroup) error {
	return nil
}

// Close satisfies the io.Closer interface, which satisfies the baseapp.StreamingService interface
func (fss *StreamingService) Close() error {
	return nil
}
