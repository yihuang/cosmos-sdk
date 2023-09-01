package tree

import (
	"crypto/sha256"

	"github.com/prysmaticlabs/gohashtree"
)

var (
	leafPrefix  = []byte{0}
	innerPrefix = []byte{1}
)

// HashFromByteSlices computes a Merkle tree where the leaves are the byte slice,
// in the provided order. It follows RFC-6962.
func HashFromByteSlices(items [][]byte) []byte {
	return hashFromByteSlices(items)
}

func hashFromByteSlices(items [][]byte) []byte {
	var err error

	if len(items) == 0 {
		return emptyHash()
	}

	chunks := make([][32]byte, len(items)*2)
	digests := make([][32]byte, len(items))
	for i, item := range items {
		// assume the item is less than 64 bytes
		copy(chunks[i*2][:], item[:32])
		copy(chunks[i*2+1][:], item[32:])
	}
	if err = gohashtree.Hash(digests, chunks); err != nil {
		panic(err)
	}

	// reuse the allocated buffers
	for len(digests) > 1 {
		// swap the buffers to hash the prevous digests
		chunks, digests = digests, chunks

		if len(chunks)%2 == 1 {
			digests = digests[:len(chunks)/2+1]
			err = gohashtree.Hash(digests, chunks[:len(chunks)-1])
			digests[len(digests)-1] = chunks[len(chunks)-1]
		} else {
			digests = digests[:len(chunks)/2]
			err = gohashtree.Hash(digests, chunks)
		}
		if err != nil {
			panic(err)
		}
	}

	return digests[0][:]
}

// returns tmhash(<empty>)
func emptyHash() []byte {
	h := sha256.Sum256([]byte{})
	return h[:]
}
