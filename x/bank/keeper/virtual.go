package keeper

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"

	errorsmod "cosmossdk.io/errors"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/bank/types"
)

func (k BaseSendKeeper) SendCoinsFromAccountToModuleVirtual(
	ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins,
) error {
	recipientAcc := k.ak.GetModuleAccount(ctx, recipientModule)
	if recipientAcc == nil {
		return errorsmod.Wrapf(sdkerrors.ErrUnknownAddress, "module account %s does not exist", recipientModule)
	}

	return k.SendCoinsVirtual(ctx, senderAddr, recipientAcc.GetAddress(), amt)
}

// SendCoinsVirtual accumulate the recipient's coins in a per-transaction transient state,
// which are sumed up and added to the real account at the end of block.
// Events are emiited the same as normal send.
func (k BaseSendKeeper) SendCoinsVirtual(ctx context.Context, fromAddr, toAddr sdk.AccAddress, amt sdk.Coins) error {
	var err error
	err = k.subUnlockedCoins(ctx, fromAddr, amt)
	if err != nil {
		return err
	}

	toAddr, err = k.sendRestriction.apply(ctx, fromAddr, toAddr, amt)
	if err != nil {
		return err
	}

	k.addVirtualCoins(ctx, toAddr, amt)

	// bech32 encoding is expensive! Only do it once for fromAddr
	fromAddrString := fromAddr.String()
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			types.EventTypeTransfer,
			sdk.NewAttribute(types.AttributeKeyRecipient, toAddr.String()),
			sdk.NewAttribute(types.AttributeKeySender, fromAddrString),
			sdk.NewAttribute(sdk.AttributeKeyAmount, amt.String()),
		),
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(types.AttributeKeySender, fromAddrString),
		),
	})

	return nil
}

func (k BaseSendKeeper) addVirtualCoins(ctx context.Context, addr sdk.AccAddress, amt sdk.Coins) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	store := sdkCtx.ObjectStore(k.objStoreKey)

	key := make([]byte, len(addr)+8)
	copy(key, addr)
	binary.BigEndian.PutUint64(key[len(addr):], uint64(sdkCtx.TxIndex()))

	var coins sdk.Coins
	value := store.Get(key)
	if value != nil {
		coins = value.(sdk.Coins)
	}
	coins = coins.Add(amt...)
	store.Set(key, coins)
}

// CreditVirtualAccounts sum up the transient coins and add them to the real account,
// should be called at end blocker.
func (k BaseSendKeeper) CreditVirtualAccounts(ctx context.Context) error {
	store := sdk.UnwrapSDKContext(ctx).ObjectStore(k.objStoreKey)

	var toAddr sdk.AccAddress
	var sum sdk.Coins
	flushCurrentAddr := func() {
		if sum.IsZero() {
			return
		}

		k.addCoins(ctx, toAddr, sum)
		sum = sum[:0]

		// Create account if recipient does not exist.
		//
		// NOTE: This should ultimately be removed in favor a more flexible approach
		// such as delegated fee messages.
		accExists := k.ak.HasAccount(ctx, toAddr)
		if !accExists {
			defer telemetry.IncrCounter(1, "new", "account")
			k.ak.SetAccount(ctx, k.ak.NewAccountWithAddress(ctx, toAddr))
		}
	}

	it := store.Iterator(nil, nil)
	defer it.Close()
	for ; it.Valid(); it.Next() {
		if len(it.Key()) <= 8 {
			return fmt.Errorf("unexpected key length: %s", hex.EncodeToString(it.Key()))
		}

		addr := it.Key()[:len(it.Key())-8]
		if !bytes.Equal(toAddr, addr) {
			flushCurrentAddr()
			toAddr = addr
		}

		coins := it.Value().(sdk.Coins)
		// TODO more efficient coins sum
		sum = sum.Add(coins...)
	}

	flushCurrentAddr()
	return nil
}
