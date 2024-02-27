package baseapp

import (
	"context"

	storetypes "cosmossdk.io/store/types"
	abci "github.com/cometbft/cometbft/abci/types"
)

type TxExecutor func(
	ctx context.Context,
	txs [][]byte,
	cms storetypes.MultiStore,
	deliverTxWithMultiStore func([]byte, storetypes.MultiStore) *abci.ExecTxResult,
) ([]*abci.ExecTxResult, error)
