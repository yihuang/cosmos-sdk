package memiavl

import (
	corelog "cosmossdk.io/core/log"
	"cosmossdk.io/store/v2/metrics"
	"cosmossdk.io/store/v2/proof"
)

type RootStore struct {
	logger corelog.Logger

	// stateCommitment reflects the state commitment (SC) backend
	stateCommitment *CommitStore

	// lastCommitInfo reflects the last version/hash that has been committed
	lastCommitInfo *proof.CommitInfo

	// telemetry reflects a telemetry agent responsible for emitting metrics (if any)
	telemetry metrics.StoreMetrics
}
