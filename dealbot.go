package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"time"

	lapi "github.com/filecoin-project/lotus/api"
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	log "github.com/sirupsen/logrus"
)

type syncMap struct {
	mu sync.RWMutex
	m  map[string]uint
}

type syncDealsList struct {
	mu          sync.RWMutex
	lastQueried time.Time
	m           []EvergreenDeal
}

var spUsageTracker = &syncMap{m: make(map[string]uint)}
var cidsBeingQueried = &syncMap{m: make(map[string]uint)}
var dealList = &syncDealsList{}

func (s *syncMap) setValue(k string, v uint) {
	s.mu.Lock()
	s.m[k] = v
	s.mu.Unlock()
}

func (s *syncMap) getValue(k string) uint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.m[k]
}

func DealbotThread(done chan bool, cfg EvergreenDealbotConfig) {
	availableDeals := getAvailableDeals_Cached(cfg)

	if len(availableDeals) < 1 {
		log.Error("available deals list malformed!")
		return
	}

	attemptedCount := 0

threadLoop:
	for {
		// Only try a certain amount of deals before expiring the thread, so a new one can start
		if attemptedCount == 1 {
			break threadLoop
		}
		attemptedCount++
		i := randomIndex(len(availableDeals))
		d := availableDeals[i]

		if d.PaddedPieceSize < cfg.Lotus.MinPieceSize {
			// Deal is too small
			continue
		}

		// Make sure that only one thread is querying a given CID
		cidIsBeingQueried := cidsBeingQueried.getValue(d.PieceCid)

		// If that CID is being queried, then go to the next one
		if cidIsBeingQueried == 1 {
			log.Debugf("CID is already being queried: %v", d.PieceCid)
			continue
		}

		// Mark the CID as being queried
		cidsBeingQueried.setValue(d.PieceCid, 1)

		// This should never happen, but just in case
		if len(d.Sources) < 1 {
			log.Errorf("no sources for deal %v", d.PieceCid)
			continue
		}

		pieceCid := d.PieceCid
		payloadCid := d.Sources[0].OriginalPayloadCid // Should all be the same payloadCid

		log.Trace("thread is querying for " + pieceCid)

		localImportSuccess := attemptDeal_Local(pieceCid, payloadCid, cfg)
		if localImportSuccess {
			log.Debug("successfully acquired deal" + d.PieceCid)
			cidsBeingQueried.setValue(d.PieceCid, 0)
			break threadLoop
		}

		retrievalSuccess := false
		// Try all the different sources (SPs) for a deal
		for _, source := range d.Sources {
			providerId := source.ProviderID

			// Separate Lock / Unlock calls here, to ensure value does not change while we check MaxConcurrent
			spUsageTracker.mu.Lock()
			spCount := spUsageTracker.m[providerId]

			if spCount >= cfg.Evergreen.MaxConcurrentRetrievalsPerSp {
				log.Debug("reached max concurrent queries for SP " + providerId)
				spUsageTracker.mu.Unlock()
				continue
			}
			// Mark the SP as being queried
			spUsageTracker.m[providerId] = spCount + 1
			spUsageTracker.mu.Unlock()

			log.Debug("trying SP " + providerId)

			retrievalSuccess = attemptDeal_Retrieval(pieceCid, payloadCid, providerId, cfg)

			if !retrievalSuccess {
				log.Debug("failed to retrieve deal from SP " + providerId)

				spCount = spUsageTracker.getValue(providerId)
				spUsageTracker.setValue(providerId, spCount-1)
				continue
			}

			if retrievalSuccess {
				spCount = spUsageTracker.getValue(providerId)
				spUsageTracker.setValue(providerId, spCount-1)
				cidsBeingQueried.setValue(d.PieceCid, 0)
				break threadLoop
			}
		}

		cidsBeingQueried.setValue(d.PieceCid, 0)
	}

	done <- true
}

// Periodically checks a directory for CAR files, creating a deal for any CARs that also have available deals
func WatcherThread(cfg EvergreenDealbotConfig) {
	for {
		carFiles, err := getCARFilesInDir(cfg.Common.CarLocationLongterm)
		if err != nil {
			log.Error(err)
		}

		log.Trace("watcherThread found %d CAR files", len(carFiles))

		availableDeals := getAvailableDeals_Cached(cfg)
		adMap := make(map[string]EvergreenDeal)

		// Put deals in a map, indexed by PieceCid for faster lookup below
		for _, deal := range availableDeals {
			adMap[deal.PieceCid] = deal
		}

		for _, car := range carFiles {
			pieceCid := strings.Split(car, ".")[0]

			deal, found := adMap[pieceCid]
			if found {
				// Mark the CID as being queried
				cidsBeingQueried.setValue(pieceCid, 1)

				// Matching deal found!
				// TODO: Potentially run on a separate thread to avoid blocking this one

				// log.Debugf("watcher thread found an open deal for %v", pieceCid)
				attemptDeal_Local(pieceCid, deal.Sources[0].OriginalPayloadCid, cfg)

				cidsBeingQueried.setValue(pieceCid, 0)
			}
		}

		time.Sleep(10 * time.Minute)
	}
}

// Attempts to import the file from long-term CAR storage
// Returns true if import was successful, false if file not found or proposal failed
func attemptDeal_Local(pieceCid string, payloadCid string, cfg EvergreenDealbotConfig) bool {
	destinationFile := GenerateCarFileName(cfg.Common.CarLocationLongterm, pieceCid)
	carExists := FileExists(destinationFile)

	if carExists == false {
		return false
	}
	log.Debugf("attempting to import CAR file locally from %v", destinationFile)

	storageMinerApi, smCloser, err := StorageMinerConnection(cfg.Lotus.MinerApiInfo)
	defer smCloser()
	if err != nil {
		// Lotus connection error
		log.Error(err)
	}

	proposalCid, err := proposeDeal(pieceCid, storageMinerApi, cfg)
	if err != nil {
		log.Debug(err)
		return false
	}

	importDeal(proposalCid, destinationFile, cfg)

	return true
}

// Attempts to retrieve the CAR file from the peer SP
// Returns true if import was successful, false if not
func attemptDeal_Retrieval(pieceCid string, payloadCid string, sourceSp string, cfg EvergreenDealbotConfig) bool {
	destinationFile := GenerateCarFileName(cfg.Common.CarLocationDownload, pieceCid)
	res, err := RetrieveCar(payloadCid, sourceSp, destinationFile, cfg)

	if !res || err != nil {
		// CAR retrieve failed
		log.Debugf("cancelling transfer due to error: %s", err)
		time.Sleep(time.Second * 30) // Wait 30 seconds, transfer may take some time to show up
		err := CancelRetrieval(payloadCid, cfg)
		if err != nil {
			log.Debugf("cancel failed: %s \n", err)
		} else {
			log.Debugf("successfully cancelled retrieval %v", payloadCid)
		}
		log.Debugln(err)

		return false
	}

	log.Debugf("successfully retrieved CAR %v", pieceCid)

	storageMinerApi, smCloser, err := StorageMinerConnection(cfg.Lotus.MinerApiInfo)
	defer smCloser()
	if err != nil {
		// Lotus connection error
		log.Error(err)
	}

	proposalCid, err := proposeDeal(pieceCid, storageMinerApi, cfg)
	if err != nil {
		log.Debug(err)
		return false
	}

	importDeal(proposalCid, destinationFile, cfg)

	// Move the CAR to long term storage
	// Wait a bit to make sure import doesn't need it any more
	// time.Sleep(time.Second * 30)
	// destinationFileLongterm := GenerateCarFileName(cfg.Common.CarLocationLongterm, pieceCid)
	// err = MoveFile(destinationFile, destinationFileLongterm)
	// if err != nil {
	// 	log.Errorf("could not move file to longterm storage: %s", err)
	// }
	return true
}

// Requests a deal, then queries Evergreen until it's accepted and returns the Proposal CID
func proposeDeal(pieceCid string, storageMinerApi lapi.StorageMiner, cfg EvergreenDealbotConfig) (string, error) {
	spid, err := storageMinerApi.ActorAddress(context.Background())
	if err != nil {
		return "", fmt.Errorf("failed getting SPID: %s", err)
	}

	rDealResponse, err := RequestDeal(spid, pieceCid, cfg)
	if err != nil || rDealResponse.ResponseCode != 200 {
		// If this happens it's likely the deal was taken by someone else while we were downloading
		return "", fmt.Errorf("failed requesting deal %s\n", err)
	}

	proposalCid := ""
	retries := 0

	// Repeatedly try to find the Deal Proposal CID
	for {
		if retries >= 15 {
			return "", fmt.Errorf("could not find deal after 15 retries")
		}

		// Wait a minute in between tries, it may take time for it to show up on Evergreen API
		time.Sleep(60 * time.Second)
		retries += 1

		response, err := GetPendingProposals(spid, cfg)

		if err != nil {
			log.Debug(err)
			continue
		} else {
			var success bool
			success, proposalCid = findDealProposalCid(response.Response.PendingProposals, pieceCid)

			if !success {
				continue
			} else {
				log.Debug("successfully got deal proposal")
				break
			}
		}
	}
	return proposalCid, nil
}

// Search through a list of pending proposals for a given pieceCid
// returns (true, proposalCid) if found, (false, "") otherwise
func findDealProposalCid(p []PendingProposal, pieceCid string) (bool, string) {
	// bafyreifkzrbx5lrognvw7jgpviuhnw2z7vtvx242rdkgpyibd3yutcodma
	for _, v := range p {
		if v.PieceCid == pieceCid {
			return true, v.DealProposalCid
		}
	}
	return false, ""
}

// Gets available deals, using the cached version first if available or querying and updating the cache if not
func getAvailableDeals_Cached(cfg EvergreenDealbotConfig) []EvergreenDeal {
	// TODO: queryInterval shouldn't be passed in as a param here
	qi := time.Duration(cfg.Evergreen.DealRequeryInterval) * time.Minute

	dealList.mu.Lock()
	lastQueried := dealList.lastQueried
	deals := dealList.m

	if time.Since(lastQueried) > qi {
		storageMinerApi, smCloser, err := StorageMinerConnection(cfg.Lotus.MinerApiInfo)
		defer smCloser()
		if err != nil {
			// Lotus connection error
			log.Error(err)
		}

		spid, err := storageMinerApi.ActorAddress(context.Background())
		if err != nil {
			log.Errorf("failed getting SPID: %s\n", err)
		}

		newDeals, err := QueryAvailableDeals(spid, cfg)

		if err != nil {
			log.Errorf("Unable to retrieve Available Deals list. %s", err)
			return deals
		}

		dealList.lastQueried = time.Now()
		dealList.m = newDeals.Response
		dealList.mu.Unlock()

		log.Debugf("found %d open deals", len(newDeals.Response))
		return newDeals.Response
	} else {
		dealList.mu.Unlock()
		return deals
	}
}

// Imports a deal to using Boost API
// https://github.com/filecoin-project/boost/blob/main/cmd/boostd/import_data.go#L18
func importDeal(pCid string, carFile string, cfg EvergreenDealbotConfig) bool {
	ctx := context.TODO()
	log.Debug("importing deal using boost API...")
	bapi, closer, err := BoostJsonRpcConnection(cfg.Lotus.BoostUrl, cfg.Lotus.BoostAuthToken)
	defer closer()
	if err != nil {
		log.Errorln(err)
		return false
	}

	_, err = os.Stat(carFile)
	if err != nil {
		log.Errorf("opening file %s: %w", carFile, err)
		return false
	}

	// Parse a deal UUID or a proposal CID
	var proposalCid *cid.Cid
	dealUuid, err := uuid.Parse(pCid)
	if err != nil {
		propCid, err := cid.Decode(pCid)
		if err != nil {
			log.Errorf("could not parse '%s' as deal uuid or proposal cid", pCid)
			return false
		}
		proposalCid = &propCid
	}

	// Look up the deal in the boost database
	deal, err := bapi.BoostDealBySignedProposalCid(ctx, *proposalCid)

	if err != nil {
		if !strings.Contains(err.Error(), "not found") {
			log.Error(err)
			return false
		}

		// The deal is not in the boost database, try the legacy
		// markets datastore (v1.1.0 deal)
		err := bapi.MarketImportDealData(ctx, *proposalCid, carFile)
		if err != nil {
			log.Errorf("couldnt import v1.1.0 deal, or find boost deal: %w", err)
			return false
		}
		log.Debugf("Offline deal import for v1.1.0 deal %s scheduled for execution\n", proposalCid.String())
		return true
	}
	// Get the deal UUID from the deal
	dealUuid = deal.DealUuid

	// Deal proposal by deal uuid (v1.2.0 deal)
	rej, err := bapi.BoostOfflineDealWithData(ctx, dealUuid, carFile)
	if err != nil {
		log.Errorf("failed to execute offline deal: %w", err)
		return false
	}
	if rej != nil && rej.Reason != "" {
		log.Errorf("offline deal %s rejected: %s", dealUuid, rej.Reason)
		return false
	}

	log.Debugf("Offline deal import for v1.2.0 deal %s scheduled for execution \n", dealUuid)

	return true
}

// Looks in the specified directory, returning name of any .car files that exist in there
// Note: dir argument must have a trailing "/" for the path
func getCARFilesInDir(dir string) ([]string, error) {
	var result []string

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return result, err
	}

	for _, file := range files {
		if file.IsDir() == false {
			name := file.Name()

			if strings.Contains(name, ".car") {
				result = append(result, name)
			}
		}
	}

	return result, nil
}
