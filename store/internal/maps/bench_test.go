package maps

import (
	"crypto/rand"
	"testing"

	"cosmossdk.io/store/internal/tree"
	"github.com/stretchr/testify/require"
)

func BenchmarkKVPairBytes(b *testing.B) {
	kvp := NewKVPair(make([]byte, 128), make([]byte, 1e6))
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		b.SetBytes(int64(len(kvp.Bytes())))
	}
}

func BenchmarkHashSimpeMap(b *testing.B) {
	sm := newSimpleMap()
	stores := []string{
		"accounts",
		"auth",
		"authz",
		"bank",
		"circuit",
		"consensus",
		"crisis",
		"distribution",
		"evidence",
		"feegrant",
		"genutil",
		"gov",
		"group",
		"mint",
		"nft",
		"params",
		"simulation",
		"slashing",
		"staking",
		"tx",
		"upgrade",
	}
	for _, store := range stores {
		hash := make([]byte, 32)
		_, err := rand.Read(hash)
		require.NoError(b, err)
		sm.Set(store, hash)
	}
	sm.Sort()
	kvsH := make([][]byte, len(sm.Kvs.Pairs))
	for i, kvp := range sm.Kvs.Pairs {
		kvsH[i] = KVPair(kvp).Bytes()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tree.HashFromByteSlices(kvsH)
	}
}
