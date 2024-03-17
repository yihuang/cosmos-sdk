package cachekv

import (
	"bytes"
	"io"
	"sort"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/puzpuzpuz/xsync/v3"

	"cosmossdk.io/math"
	"cosmossdk.io/store/cachekv/internal"
	"cosmossdk.io/store/internal/conv"
	"cosmossdk.io/store/internal/kv"
	"cosmossdk.io/store/tracekv"
	"cosmossdk.io/store/types"
)

// cValue represents a cached value.
// If dirty is true, it indicates the cached value is different from the underlying value.
type cValue struct {
	value []byte
	dirty bool
}

// Store wraps an in-memory cache around an underlying types.KVStore.
type Store struct {
	cache         *xsync.Map
	unsortedCache *xsync.Map
	sortedCache   internal.BTree // always ascending sorted
	parent        types.KVStore
}

var _ types.CacheKVStore = (*Store)(nil)

// NewStore creates a new Store object
func NewStore(parent types.KVStore) *Store {
	return &Store{
		cache:         xsync.NewMap(),
		unsortedCache: xsync.NewMap(),
		sortedCache:   internal.NewBTree(),
		parent:        parent,
	}
}

// GetStoreType implements Store.
func (store *Store) GetStoreType() types.StoreType {
	return store.parent.GetStoreType()
}

// Get implements types.KVStore.
func (store *Store) Get(key []byte) (value []byte) {
	types.AssertValidKey(key)

	cacheValue, ok := store.cache.Load(conv.UnsafeBytesToStr(key))
	if !ok {
		value = store.parent.Get(key)
		store.setCacheValue(key, value, false)
	} else {
		value = cacheValue.(*cValue).value
	}

	return value
}

// Set implements types.KVStore.
func (store *Store) Set(key, value []byte) {
	types.AssertValidKey(key)
	types.AssertValidValue(value)
	store.setCacheValue(key, value, true)
}

// Has implements types.KVStore.
func (store *Store) Has(key []byte) bool {
	value := store.Get(key)
	return value != nil
}

// Delete implements types.KVStore.
func (store *Store) Delete(key []byte) {
	types.AssertValidKey(key)
	store.setCacheValue(key, nil, true)
}

func (store *Store) resetCaches() {
	if store.cache.Size() > 100_000 {
		// Cache is too large. We likely did something linear time
		// (e.g. Epoch block, Genesis block, etc). Free the old caches from memory, and let them get re-allocated.
		// TODO: In a future CacheKV redesign, such linear workloads should get into a different cache instantiation.
		// 100_000 is arbitrarily chosen as it solved Osmosis' InitGenesis RAM problem.
		store.cache.Clear()
		store.unsortedCache.Clear()
	} else {
		store.cache.Clear()
		store.unsortedCache.Clear()
	}
	store.sortedCache = internal.NewBTree()
}

// Implements Cachetypes.KVStore.
func (store *Store) Write() {
	if store.cache.Size() == 0 && store.unsortedCache.Size() == 0 {
		store.sortedCache = internal.NewBTree()
		return
	}

	type cEntry struct {
		key string
		val *cValue
	}

	// We need a copy of all of the keys.
	// Not the best. To reduce RAM pressure, we copy the values as well
	// and clear out the old caches right after the copy.
	sortedCache := make([]cEntry, 0, store.cache.Size())

	store.cache.Range(func(key string, value interface{}) bool {
		dbValue := value.(*cValue)
		if dbValue.dirty {
			sortedCache = append(sortedCache, cEntry{key, dbValue})
		}
		return true
	})
	store.resetCaches()
	sort.Slice(sortedCache, func(i, j int) bool {
		return sortedCache[i].key < sortedCache[j].key
	})

	// TODO: Consider allowing usage of Batch, which would allow the write to
	// at least happen atomically.
	for _, obj := range sortedCache {
		// We use []byte(key) instead of conv.UnsafeStrToBytes because we cannot
		// be sure if the underlying store might do a save with the byteslice or
		// not. Once we get confirmation that .Delete is guaranteed not to
		// save the byteslice, then we can assume only a read-only copy is sufficient.
		if obj.val.value != nil {
			// It already exists in the parent, hence update it.
			store.parent.Set([]byte(obj.key), obj.val.value)
		} else {
			store.parent.Delete([]byte(obj.key))
		}
	}
}

// CacheWrap implements CacheWrapper.
func (store *Store) CacheWrap() types.CacheWrap {
	return NewStore(store)
}

// CacheWrapWithTrace implements the CacheWrapper interface.
func (store *Store) CacheWrapWithTrace(w io.Writer, tc types.TraceContext) types.CacheWrap {
	return NewStore(tracekv.NewStore(store, w, tc))
}

//----------------------------------------
// Iteration

// Iterator implements types.KVStore.
func (store *Store) Iterator(start, end []byte) types.Iterator {
	return store.iterator(start, end, true)
}

// ReverseIterator implements types.KVStore.
func (store *Store) ReverseIterator(start, end []byte) types.Iterator {
	return store.iterator(start, end, false)
}

func (store *Store) iterator(start, end []byte, ascending bool) types.Iterator {
	store.dirtyItems(start, end)
	isoSortedCache := store.sortedCache.Copy()

	var (
		err           error
		parent, cache types.Iterator
	)

	if ascending {
		parent = store.parent.Iterator(start, end)
		cache, err = isoSortedCache.Iterator(start, end)
	} else {
		parent = store.parent.ReverseIterator(start, end)
		cache, err = isoSortedCache.ReverseIterator(start, end)
	}
	if err != nil {
		panic(err)
	}

	return internal.NewCacheMergeIterator(parent, cache, ascending)
}

func findStartIndex(strL []string, startQ string) int {
	// Modified binary search to find the very first element in >=startQ.
	if len(strL) == 0 {
		return -1
	}

	var left, right, mid int
	right = len(strL) - 1
	for left <= right {
		mid = (left + right) >> 1
		midStr := strL[mid]
		if midStr == startQ {
			// Handle condition where there might be multiple values equal to startQ.
			// We are looking for the very first value < midStL, that i+1 will be the first
			// element >= midStr.
			for i := mid - 1; i >= 0; i-- {
				if strL[i] != midStr {
					return i + 1
				}
			}
			return 0
		}
		if midStr < startQ {
			left = mid + 1
		} else { // midStrL > startQ
			right = mid - 1
		}
	}
	if left >= 0 && left < len(strL) && strL[left] >= startQ {
		return left
	}
	return -1
}

func findEndIndex(strL []string, endQ string) int {
	if len(strL) == 0 {
		return -1
	}

	// Modified binary search to find the very first element <endQ.
	var left, right, mid int
	right = len(strL) - 1
	for left <= right {
		mid = (left + right) >> 1
		midStr := strL[mid]
		if midStr == endQ {
			// Handle condition where there might be multiple values equal to startQ.
			// We are looking for the very first value < midStL, that i+1 will be the first
			// element >= midStr.
			for i := mid - 1; i >= 0; i-- {
				if strL[i] < midStr {
					return i + 1
				}
			}
			return 0
		}
		if midStr < endQ {
			left = mid + 1
		} else { // midStrL > startQ
			right = mid - 1
		}
	}

	// Binary search failed, now let's find a value less than endQ.
	for i := right; i >= 0; i-- {
		if strL[i] < endQ {
			return i
		}
	}

	return -1
}

type sortState int

const (
	stateUnsorted sortState = iota
	stateAlreadySorted
)

const minSortSize = 1024

// Constructs a slice of dirty items, to use w/ memIterator.
func (store *Store) dirtyItems(start, end []byte) {
	startStr, endStr := conv.UnsafeBytesToStr(start), conv.UnsafeBytesToStr(end)
	if end != nil && startStr > endStr {
		// Nothing to do here.
		return
	}

	n := store.unsortedCache.Size()
	unsorted := make([]*kv.Pair, 0)
	// If the unsortedCache is too big, its costs too much to determine
	// whats in the subset we are concerned about.
	// If you are interleaving iterator calls with writes, this can easily become an
	// O(N^2) overhead.
	// Even without that, too many range checks eventually becomes more expensive
	// than just not having the cache.
	if n < minSortSize {
		store.unsortedCache.Range(func(key string, _ interface{}) bool {
			// dbm.IsKeyInDomain is nil safe and returns true iff key is greater than start
			if dbm.IsKeyInDomain(conv.UnsafeStrToBytes(key), start, end) {
				cacheValue, _ := store.cache.Load(key)
				unsorted = append(unsorted, &kv.Pair{Key: []byte(key), Value: cacheValue.(*cValue).value})
			}
			return true
		})
		store.clearUnsortedCacheSubset(unsorted, stateUnsorted)
		return
	}

	// Otherwise it is large so perform a modified binary search to find
	// the target ranges for the keys that we should be looking for.
	strL := make([]string, 0, n)
	store.unsortedCache.Range(func(key string, _ interface{}) bool {
		strL = append(strL, key)
		return true
	})
	sort.Strings(strL)

	// Now find the values within the domain
	//  [start, end)
	startIndex := findStartIndex(strL, startStr)
	if startIndex < 0 {
		startIndex = 0
	}

	var endIndex int
	if end == nil {
		endIndex = len(strL) - 1
	} else {
		endIndex = findEndIndex(strL, endStr)
	}
	if endIndex < 0 {
		endIndex = len(strL) - 1
	}

	// Since we spent cycles to sort the values, we should process and remove a reasonable amount
	// ensure start to end is at least minSortSize in size
	// if below minSortSize, expand it to cover additional values
	// this amortizes the cost of processing elements across multiple calls
	if endIndex-startIndex < minSortSize {
		endIndex = math.Min(startIndex+minSortSize, len(strL)-1)
		if endIndex-startIndex < minSortSize {
			startIndex = math.Max(endIndex-minSortSize, 0)
		}
	}

	kvL := make([]*kv.Pair, 0, 1+endIndex-startIndex)
	for i := startIndex; i <= endIndex; i++ {
		key := strL[i]
		cacheValue, _ := store.cache.Load(key)
		kvL = append(kvL, &kv.Pair{Key: []byte(key), Value: cacheValue.(*cValue).value})
	}

	// kvL was already sorted so pass it in as is.
	store.clearUnsortedCacheSubset(kvL, stateAlreadySorted)
}

func (store *Store) clearUnsortedCacheSubset(unsorted []*kv.Pair, sortState sortState) {
	n := store.unsortedCache.Size()
	if len(unsorted) == n { // This pattern allows the Go compiler to emit the map clearing idiom for the entire map.
		store.unsortedCache.Clear()
	} else { // Otherwise, normally delete the unsorted keys from the map.
		for _, kv := range unsorted {
			store.unsortedCache.Delete(conv.UnsafeBytesToStr(kv.Key))
		}
	}

	if sortState == stateUnsorted {
		sort.Slice(unsorted, func(i, j int) bool {
			return bytes.Compare(unsorted[i].Key, unsorted[j].Key) < 0
		})
	}

	for _, item := range unsorted {
		// sortedCache is able to store `nil` value to represent deleted items.
		store.sortedCache.Set(item.Key, item.Value)
	}
}

//----------------------------------------
// etc

// Only entrypoint to mutate store.cache.
// A `nil` value means a deletion.
func (store *Store) setCacheValue(key, value []byte, dirty bool) {
	keyStr := conv.UnsafeBytesToStr(key)
	if !dirty {
		// set if not exists
		store.cache.LoadOrStore(keyStr, &cValue{value: value})
	}

	// force update
	store.cache.Store(keyStr, &cValue{
		value: value,
		dirty: dirty,
	})
	store.unsortedCache.Store(keyStr, struct{}{})
}
