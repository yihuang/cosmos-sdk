package cachekv

import (
	"strconv"
	"testing"

	"cosmossdk.io/store/cachekv/internal"
	"github.com/puzpuzpuz/xsync/v3"
)

func BenchmarkLargeUnsortedMisses(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		store := generateStore()
		b.StartTimer()

		for k := 0; k < 10000; k++ {
			// cache has A + Z values
			// these are within range, but match nothing
			store.dirtyItems([]byte("B1"), []byte("B2"))
		}
	}
}

func generateStore() *Store {
	cache := xsync.NewMap()
	unsorted := xsync.NewMap()
	for i := 0; i < 5000; i++ {
		key := "A" + strconv.Itoa(i)
		unsorted.Store(key, struct{}{})
		cache.Store(key, &cValue{})
	}

	for i := 0; i < 5000; i++ {
		key := "Z" + strconv.Itoa(i)
		unsorted.Store(key, struct{}{})
		cache.Store(key, &cValue{})
	}

	return &Store{
		cache:         cache,
		unsortedCache: unsorted,
		sortedCache:   internal.NewBTree(),
	}
}
