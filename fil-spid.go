package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/lotus/chain/types"
)

// Generates a FIL SPID auth token for use with Evergreen APIs
// Based on fil-spid.bash
// https://github.com/filecoin-project/evergreen-dealer/blob/master/misc/fil-spid.bash
// Example FIL-SPID-V0 2205910;f012345;2;jKxW4olDvm+y+03uPyW+7ugoZ1tO6Bh3zdHF6B13tYUBV+c8uzmqJVfk5IK77CHrE/oRwBpizgsohdMQuFvflz9YMOYcTZ/zAeTUyswpwQ+/k011LL07PpjRE4fqniRI
func evergreenFilSPID(spidAddr address.Address, cfg EvergreenDealbotConfig) (string, error) {
	filAuthAddr := "FIL-SPID-V0"
	var filGenesisUnix int64 = 1598306400
	nowUnix := time.Now().Unix()
	ctx := context.Background()

	api, closer, err := LotusConnection(cfg.Lotus.FullNodeApiInfo)
	defer closer()
	if err != nil {
		return "", fmt.Errorf("error creating lotus connection %s", err)
	}

	b64SpacePad := "ICAg" // use this to pefix the random beacon, lest it becomes valid CBOR
	filCurrentEpoch := (nowUnix - 1 - filGenesisUnix) / 30

	filFinalizedTipset, err := api.ChainGetTipSetByHeight(ctx, abi.ChainEpoch(filCurrentEpoch-900), types.NewTipSetKey())
	if err != nil {
		return "", err
	}
	finalizedTsk := types.NewTipSetKey(filFinalizedTipset.Cids()...)

	filFinalizedWorkerId, err := api.StateMinerInfo(ctx, spidAddr, finalizedTsk)
	if err != nil {
		return "", err
	}

	filCurrentDrandB64, err := api.StateGetBeaconEntry(ctx, abi.ChainEpoch(filCurrentEpoch))
	if err != nil {
		return "", err
	}

	// Space Pad and DRAND must be appended as Base64, then converted back to []byte before being signed
	messgeToSign, err := base64.StdEncoding.DecodeString(b64SpacePad + base64.StdEncoding.EncodeToString(filCurrentDrandB64.Data))
	if err != nil {
		return "", err
	}

	filAuthSig, err := api.WalletSign(ctx, filFinalizedWorkerId.Worker, messgeToSign)
	if err != nil {
		return "", err
	}

	encodedSig := base64.StdEncoding.EncodeToString(filAuthSig.Data)

	result := fmt.Sprintf("%v %v;%v;%v;%v", filAuthAddr, filCurrentEpoch, spidAddr, filAuthSig.Type, encodedSig)

	return result, nil
}
