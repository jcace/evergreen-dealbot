package main

import (
	"context"
	"net/http"
	"time"

	bapi "github.com/filecoin-project/boost/api"
	jsonrpc "github.com/filecoin-project/go-jsonrpc"
	lapi "github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/api/client"
	"github.com/filecoin-project/lotus/api/v1api"
	cliutil "github.com/filecoin-project/lotus/cli/util"
	log "github.com/sirupsen/logrus"
)

func LotusConnection(fullNodeApiInfo string) (v1api.FullNode, jsonrpc.ClientCloser, error) {
	info := cliutil.ParseApiInfo(fullNodeApiInfo)

	var api lapi.FullNode
	var closer jsonrpc.ClientCloser
	addr, err := info.DialArgs("v1")
	if err != nil {
		log.Errorf("Error getting v1 API address %s", err)
		return nil, nil, err
	}

	for {
		api, closer, err = client.NewFullNodeRPCV1(context.Background(), addr, info.AuthHeader())
		if err != nil {
			log.Warningf("connecting with lotus failed. Retrying after 10 minutes: %s", err)
			time.Sleep(10 * time.Minute)
		} else {
			break
		}
	}

	return api, closer, nil
}

func StorageMinerConnection(marketApiInfo string) (lapi.StorageMiner, jsonrpc.ClientCloser, error) {
	ctx := context.Background()
	info := cliutil.ParseApiInfo(marketApiInfo)

	addr, err := info.DialArgs("v0")
	if err != nil {
		log.Errorf("Error getting v0 Storage Miner API address %s", err)
		return nil, nil, err
	}

	var storageMinerApi lapi.StorageMiner
	var storageMinerCloser jsonrpc.ClientCloser

	for {
		storageMinerApi, storageMinerCloser, err = client.NewStorageMinerRPCV0(ctx, addr, info.AuthHeader())
		if err != nil {
			log.Warningf("Connecting with Storage Miner API failed. Retrying after 10 minutes: %s", err)
			time.Sleep(10 * time.Minute)
		} else {
			break
		}
	}
	return storageMinerApi, storageMinerCloser, nil
}

func BoostJsonRpcConnection(boostUrl string, boostAuthToken string) (*bapi.BoostStruct, jsonrpc.ClientCloser, error) {
	headers := http.Header{"Authorization": []string{"Bearer " + boostAuthToken}}
	ctx := context.Background()

	var api bapi.BoostStruct
	closer, err := jsonrpc.NewMergeClient(ctx, "http://"+boostUrl+"/rpc/v0", "Filecoin", []interface{}{&api.Internal, &api.CommonStruct.Internal}, headers)
	if err != nil {
		return nil, nil, err
	}

	return &api, closer, nil
}
