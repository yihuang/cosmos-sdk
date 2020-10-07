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

// Service is a structure which wraps the staking handler
// and provides methods useful in tests
type Service struct {
	ctx sdk.Context
	h   sdk.Handler
}

// NewService creates staking Handler wrapper for tests
func NewService(ctx sdk.Context, k keeper.Keeper) *Service {
	return &Service{ctx, staking.NewHandler(k)}
}

// CreateValidator calls handler to create a new staking validator
func (sh *Service) CreateValidator(t *testing.T, addr sdk.ValAddress, pk crypto.PubKey, stakeAmount int64, cr stakingtypes.CommissionRates) {
	coin := sdk.NewCoin(sdk.DefaultBondDenom, sdk.NewInt(stakeAmount))
	sh.createValidator(t, addr, pk, coin, cr)

}

// CreateValidatorWithValPower calls handler to create a new staking validator
func (sh *Service) CreateValidatorWithValPower(t *testing.T, addr sdk.ValAddress, pk crypto.PubKey, valPower int64, cr stakingtypes.CommissionRates) sdk.Int {
	amount := sdk.TokensFromConsensusPower(valPower)
	coin := sdk.NewCoin(sdk.DefaultBondDenom, amount)
	sh.createValidator(t, addr, pk, coin, cr)
	return amount
}

func (sh *Service) createValidator(t *testing.T, addr sdk.ValAddress, pk crypto.PubKey, coin sdk.Coin, cr stakingtypes.CommissionRates) {
	msg, err := stakingtypes.NewMsgCreateValidator(addr, pk, coin, stakingtypes.Description{}, cr, sdk.OneInt())
	require.NoError(t, err)
	sh.Handle(t, msg)
}

// Delegate calls handler to delegate stake for a validator
func (sh *Service) Delegate(t *testing.T, delAddr sdk.AccAddress, valAddr sdk.ValAddress, amount int64) {
	coin := sdk.NewCoin(sdk.DefaultBondDenom, sdk.NewInt(amount))
	msg := stakingtypes.NewMsgDelegate(delAddr, valAddr, coin)
	sh.Handle(t, msg)
}

// DelegateWithPower calls handler to delegate stake for a validator
func (sh *Service) DelegateWithPower(t *testing.T, delAddr sdk.AccAddress, valAddr sdk.ValAddress, power int64) {
	coin := sdk.NewCoin(sdk.DefaultBondDenom, sdk.TokensFromConsensusPower(power))
	msg := stakingtypes.NewMsgDelegate(delAddr, valAddr, coin)
	sh.Handle(t, msg)
}

// Handle calls staking handler on a given message
func (sh *Service) Handle(t *testing.T, msg sdk.Msg) {
	res, err := sh.h(sh.ctx, msg)
	require.NoError(t, err)
	require.NotNil(t, res)
}
