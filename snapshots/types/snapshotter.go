package types

import (
	protoio "github.com/gogo/protobuf/io"
)

// Snapshotter is something that can create and restore snapshots, consisting of streamed binary
// chunks - all of which must be read from the channel and closed. If an unsupported format is
// given, it must return ErrUnknownFormat (possibly wrapped with fmt.Errorf).
type Snapshotter interface {
	// Snapshot creates a state snapshot, returning a channel of snapshot chunk readers.
	Snapshot(height uint64, protoWriter protoio.WriteCloser) error

	// Restore restores a state snapshot, taking snapshot chunk readers as input.
	// If the ready channel is non-nil, it returns a ready signal (by being closed) once the
	// restorer is ready to accept chunks.
	Restore(height uint64, format uint32, protoReader protoio.ReadCloser, chReady chan<- struct{}) (SnapshotItem, error)
}

// NamedSnapshotter is a Snapshotter with an unique name.
type NamedSnapshotter interface {
	Snapshotter

	// SnapshotterName returns the name of snapshotter, it should be unique in the manager.
	SnapshotterName() string
}
