package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-state-types/big"
	lapi "github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/api/v1api"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/ipfs/go-cid"
	log "github.com/sirupsen/logrus"
)

func RetrieveCar(c string, peer string, path string, cfg EvergreenDealbotConfig) (bool, error) {
	ctx := context.Background()

	// ### The following code was taken from lotus client_retr.go, `retrieve()` function

	api, closer, err := LotusConnection(cfg.Lotus.FullNodeApiInfo)
	defer closer()
	if err != nil {
		return false, fmt.Errorf("error creating lotus connection %s", err)
	}

	// Wallet that will pay for the retrieval (not required for now)
	payer, err := api.WalletDefaultAddress(ctx)
	if err != nil {
		return false, fmt.Errorf("error getting default wallet address: %s", err)
	}

	// Cid of the thing we want to retrieve
	file, err := cid.Parse(c)
	if err != nil {
		return false, fmt.Errorf("parsing cid failed: %s", err)
	}

	// Handle the --allow-local flag, retrieve from local datastore if it exists
	imports, err := api.ClientListImports(ctx)
	if err != nil {
		return false, fmt.Errorf("error handling client list imports: %s", err)
	}

	var eref *lapi.ExportRef
	for _, i := range imports {
		if i.Root != nil && i.Root.Equals(file) {
			eref = &lapi.ExportRef{
				Root:         file,
				FromLocalCAR: i.CARPath,
			}
			break
		}
	}

	// Return the locally retrieved eref

	if eref != nil {
		exportCar(ctx, api, eref, path)
		return true, nil
	}

	minerAddr, err := address.NewFromString(peer)
	if err != nil {
		return false, err
	}

	offer, err := api.ClientMinerQueryOffer(ctx, minerAddr, file, nil)
	if err != nil {
		return false, err
	}

	if offer.Err != "" {
		return false, fmt.Errorf("offer error: %s", offer.Err)
	}

	maxPrice := types.MustParseFIL(cfg.Lotus.MaxRetrievalPrice)
	if offer.MinPrice.GreaterThan(big.Int(maxPrice)) {
		return false, fmt.Errorf("failed to find offer satisfying maxPrice: %s", maxPrice)
	}

	o := offer.Order(payer)
	o.DataSelector = nil

	subscribeEvents, err := api.ClientGetRetrievalUpdates(ctx)
	if err != nil {
		return false, fmt.Errorf("failure setting up retrieval updates: %w", err)
	}

	retrievalRes, err := api.ClientRetrieve(ctx, o)

	if err != nil {
		return false, fmt.Errorf("failure setting up retrieval: %w", err)
	}

	start := time.Now()
	lastEvt := time.Now()
	to := time.Duration(cfg.Lotus.RetrievalTimeout) * time.Minute

	ticker := time.NewTicker(10 * time.Second)
	quitTicker := make(chan struct{})

readEvents:
	for {
		var evt lapi.RetrievalInfo
		select {
		case <-ticker.C:
			if time.Since(lastEvt) > to {
				// Timeout has elapsed - end the retrieval
				ticker.Stop()
				close(quitTicker)
				return false, fmt.Errorf("retrieval timed out after %v minutes", cfg.Lotus.RetrievalTimeout)
			}
			continue
		case <-ctx.Done():
			return false, fmt.Errorf("lotus retrieval timed out")
		case evt = <-subscribeEvents:
			if evt.ID != retrievalRes.DealID {
				// we can't check the deal ID ahead of time because:
				// 1. We need to subscribe before retrieving.
				// 2. We won't know the deal ID until after retrieving.
				continue
			}
		}

		event := "New"
		if evt.Event != nil {
			event = retrievalmarket.ClientEvents[*evt.Event]
		}
		// Recv 0 B, Paid 0 FIL, DealAccepted (Accepted), 7.818s
		// If ClientEvent == "DealAccept" ? Or if first packet comes thru, then start the acquire thread

		// Recv 1.354 KiB, Paid 0 FIL, BlocksReceived (Ongoing), 232ms
		lastEvt = time.Now()
		log.Debugf("Recv %s, Paid %s, %s (%s), %s\n",
			types.SizeStr(types.NewInt(evt.BytesReceived)),
			types.FIL(evt.TotalPaid),
			strings.TrimPrefix(event, "ClientEvent"),
			strings.TrimPrefix(retrievalmarket.DealStatuses[evt.Status], "DealStatus"),
			time.Now().Sub(start).Truncate(time.Millisecond),
		)

		switch evt.Status {
		case retrievalmarket.DealStatusCompleted:
			break readEvents
		case retrievalmarket.DealStatusRejected:
			return false, fmt.Errorf("retrieval proposal rejected: %s", evt.Message)
		case retrievalmarket.DealStatusCancelled:
			return false, fmt.Errorf("retrieval proposal cancelled: %s", evt.Message)
		case
			retrievalmarket.DealStatusDealNotFound,
			retrievalmarket.DealStatusErrored:
			return false, fmt.Errorf("retrieval error: %s", evt.Message)
		}
	}

	eref = &lapi.ExportRef{
		Root:   file,
		DealID: retrievalRes.DealID,
	}

	// Export CAR
	err = exportCar(ctx, api, eref, path)
	if err != nil {
		return false, fmt.Errorf("error exporting CAR: %w", err)
	}

	return true, nil
}

func exportCar(ctx context.Context, api v1api.FullNode, eref *lapi.ExportRef, path string) error {
	log.Debugf("exporting CAR file %s", path)
	err := api.ClientExport(ctx, *eref, lapi.FileRef{
		Path:  path,
		IsCAR: true,
	})
	if err != nil {
		return err
	}

	return nil
}
