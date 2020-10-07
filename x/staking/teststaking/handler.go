package teststaking

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/staking"
	"github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// HandlerT is a structure which wraps the staking handler
// and provides methods useful in tests
type HandlerT struct {
	ctx sdk.Context
	h   sdk.Handler
}

// NewHandlerT creates staking Handler wrapper for tests
func NewHandlerT(ctx sdk.Context, k keeper.Keeper) *HandlerT {
	return &HandlerT{ctx, staking.NewHandler(k)}
}

// CreateValidator calls handler to create a new staking validator
func (sk *HandlerT) CreateValidator(t *testing.T, addr sdk.ValAddress, pk crypto.PubKey, stakeAmount int64, cr stakingtypes.CommissionRates) {
	coin := sdk.NewCoin(sdk.DefaultBondDenom, sdk.NewInt(stakeAmount))
	sk.createValidator(t, addr, pk, coin, cr)

}

// CreateValidatorWithValPower calls handler to create a new staking validator
func (sk *HandlerT) CreateValidatorWithValPower(t *testing.T, addr sdk.ValAddress, pk crypto.PubKey, valPower int64, cr stakingtypes.CommissionRates) sdk.Int {
	amount := sdk.TokensFromConsensusPower(valPower)
	coin := sdk.NewCoin(sdk.DefaultBondDenom, amount)
	sk.createValidator(t, addr, pk, coin, cr)
	return amount
}

func (sk *HandlerT) createValidator(t *testing.T, addr sdk.ValAddress, pk crypto.PubKey, coin sdk.Coin, cr stakingtypes.CommissionRates) {
	msg, err := stakingtypes.NewMsgCreateValidator(addr, pk, coin, stakingtypes.Description{}, cr, sdk.OneInt())
	require.NoError(t, err)
	sk.handle(t, msg)
}

func (sk *HandlerT) delegate(t *testing.T, delAddr sdk.AccAddress, valAddr sdk.ValAddress, amount int64) {
	coin := sdk.NewCoin(sdk.DefaultBondDenom, sdk.NewInt(amount))
	msg := stakingtypes.NewMsgDelegate(delAddr, valAddr, coin)
	sk.handle(t, msg)
}

func (sk *HandlerT) delegateWithPower(t *testing.T, delAddr sdk.AccAddress, valAddr sdk.ValAddress, power int64) {
	coin := sdk.NewCoin(sdk.DefaultBondDenom, sdk.TokensFromConsensusPower(power))
	msg := stakingtypes.NewMsgDelegate(delAddr, valAddr, coin)
	sk.handle(t, msg)
}

func (sk *HandlerT) handle(t *testing.T, msg sdk.Msg) {
	res, err := sk.h(sk.ctx, msg)
	require.NoError(t, err)
	require.NotNil(t, res)
}
