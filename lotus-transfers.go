package main

import (
	"context"
	"fmt"
	"time"

	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/lotus/api"
	"github.com/ipfs/go-cid"
	log "github.com/sirupsen/logrus"
)

func CancelRetrieval(payloadCidToCancel string, cfg EvergreenDealbotConfig) error {
	ctx := context.Background()

	cid, err := cid.Parse(payloadCidToCancel)
	if err != nil {
		return fmt.Errorf("failed parsing cid: %s", err)
	}

	api, closer, err := LotusConnection(cfg.Lotus.FullNodeApiInfo)
	defer closer()
	if err != nil {
		return fmt.Errorf("error creating lotus connection %s", err)
	}

	retrievals, err := api.ClientListRetrievals(ctx)
	if err != nil {
		return fmt.Errorf("listing retrievals failed: %s", err)
	}

	found := false

	var rid retrievalmarket.DealID

	for _, retrieval := range retrievals {
		if retrieval.PayloadCID == cid {
			if retrieval.Status == retrievalmarket.DealStatusCancelled || retrieval.Status == retrievalmarket.DealStatusCancelling {
				continue
			}
			api.ClientCancelRetrievalDeal(ctx, rid)
			found = true
		}
	}

	// Sometimes the transfer cancelling will block, so let's time it out
	cancelTimeoutChan := make(chan bool, 1)
	go func() {
		res := cancelTransfersForCid(ctx, api, cid)
		cancelTimeoutChan <- res
	}()

	select {
	case <-cancelTimeoutChan:
		break
	case <-time.After(10 * time.Second):
		log.Debug("cancelling transfers timed out")
		break
	}

	if !found {
		return fmt.Errorf("unable to find matching retrieval")
	}

	return nil
}

func cancelTransfersForCid(ctx context.Context, api api.FullNode, cid cid.Cid) bool {
	transfers, err := api.ClientListDataTransfers(ctx)
	if err != nil {
		log.Errorf("listing transfers failed: %s", err)
		return false
	}

	for _, xfer := range transfers {
		if xfer.BaseCID == cid {
			if xfer.Status == datatransfer.Cancelled || xfer.Status == datatransfer.Cancelling {
				continue
			}
			api.ClientCancelDataTransfer(ctx, xfer.TransferID, xfer.OtherPeer, xfer.IsInitiator)
			log.Debugf("cancelling data transfer channel %v", xfer.TransferID)
		}
	}

	return true
}

func CancelAllRetrievals(cfg EvergreenDealbotConfig) error {
	ctx := context.Background()
	count := 0

	api, closer, err := LotusConnection(cfg.Lotus.FullNodeApiInfo)
	defer closer()
	if err != nil {
		return fmt.Errorf("error creating lotus connection %s", err)
	}

	retrievals, err := api.ClientListRetrievals(ctx)
	if err != nil {
		return fmt.Errorf("listing channels failed: %s", err)
	}

	fmt.Printf("found a total of %v transfers\n", len(retrievals))

	for _, retr := range retrievals {
		if retr.Status == retrievalmarket.DealStatusNew || retr.Status == retrievalmarket.DealStatusWaitForAcceptance || retr.Status == retrievalmarket.DealStatusWaitForAcceptanceLegacy {

			time.Sleep(time.Millisecond * 250)
			fmt.Printf("cancelling retrieval with %v\n", retr.ID)
			er := api.ClientCancelRetrievalDeal(ctx, retr.ID)
			if er != nil {
				fmt.Printf("error cancelling retrieval: %s \n", er)
				continue
			}
			count++
			fmt.Printf("cancelled retrieval %v \n", retr.ID)
		}
	}

	fmt.Printf("cancelled %v retrievals \n", count)
	return nil
}

func CancelAllTransfers(cfg EvergreenDealbotConfig) error {
	ctx := context.TODO()
	api, closer, err := LotusConnection(cfg.Lotus.FullNodeApiInfo)
	defer closer()

	if err != nil {
		return fmt.Errorf("error creating lotus connection %s", err)
	}

	transfers, err := api.ClientListDataTransfers(ctx)
	if err != nil {
		return fmt.Errorf("listing transfers failed: %s", err)
	}

	for _, xfer := range transfers {
		if xfer.Status == datatransfer.Cancelled || xfer.Status == datatransfer.Cancelling {
			continue
		}
		api.ClientCancelDataTransfer(ctx, xfer.TransferID, xfer.OtherPeer, xfer.IsInitiator)
		log.Debugf("cancelling data transfer channel %v", xfer.TransferID)
	}
	return nil
}
