package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/filecoin-project/go-address"
	log "github.com/sirupsen/logrus"
)

func RequestDeal(spid address.Address, pieceCid string, cfg EvergreenDealbotConfig) (*RequestDealResponse, error) {
	req, err := http.NewRequest("GET", "https://api.evergreen.filecoin.io/sp/request_piece/"+pieceCid, nil)
	if err != nil {
		return nil, err
	}

	authCode, err := evergreenFilSPID(spid, cfg)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", strings.TrimSuffix(authCode, "\n"))
	persistentClient := PersistentHeaderHttpClient(req)

	resp, err := persistentClient.Do(req)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	result, err := UnmarshalRequestDealResponse(body)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func GetPendingProposals(spid address.Address, cfg EvergreenDealbotConfig) (*PendingProposalsResponse, error) {
	req, err := http.NewRequest("GET", "https://api.evergreen.filecoin.io/sp/pending_proposals", nil)
	if err != nil {
		return nil, err
	}

	authCode, err := evergreenFilSPID(spid, cfg)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", strings.TrimSuffix(authCode, "\n"))
	persistentClient := PersistentHeaderHttpClient(req)
	resp, err := persistentClient.Do(req)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	result, err := UnmarshalPendingProposalsResponse(body)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func QueryAvailableDeals(spid address.Address, cfg EvergreenDealbotConfig) (*AvailableDeals, error) {
	log.Debug("Querying for available deals...")

	req, err := http.NewRequest("GET", "https://api.evergreen.filecoin.io/sp/eligible_pieces?limit=100000", nil)
	if err != nil {
		return nil, err
	}

	authCode, err := evergreenFilSPID(spid, cfg)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", strings.TrimSuffix(authCode, "\n"))
	persistentClient := PersistentHeaderHttpClient(req)
	resp, err := persistentClient.Do(req)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	result, err := UnmarshalAvailableDeals(body)
	if err != nil {
		return nil, err
	}

	return &result, err

}

// ########### TYPES

func UnmarshalRequestDealResponse(data []byte) (RequestDealResponse, error) {
	var r RequestDealResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *RequestDealResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type RequestDealResponse struct {
	RequestID    string       `json:"request_id"`
	ResponseCode int64        `json:"response_code"`
	InfoLines    []string     `json:"info_lines"`
	Response     DealResponse `json:"response"`
}

type DealResponse struct {
	TentativeReplicaCounts map[string]int64 `json:"tentative_replica_counts"`
	BytesPendingCurrent    int64            `json:"bytes_pending_current"`
	BytesPendingMax        int64            `json:"bytes_pending_max"`
}

func UnmarshalPendingProposalsResponse(data []byte) (PendingProposalsResponse, error) {
	var r PendingProposalsResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *PendingProposalsResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type PendingProposalsResponse struct {
	RequestID          string           `json:"request_id"`
	ResponseCode       int64            `json:"response_code"`
	ResponseTimestamp  string           `json:"response_timestamp"`
	ResponseStateEpoch int64            `json:"response_state_epoch"`
	InfoLines          []string         `json:"info_lines"`
	Response           ProposalResponse `json:"response"`
}

type ProposalResponse struct {
	PendingProposals []PendingProposal `json:"pending_proposals"`
}

type PendingProposal struct {
	DealProposalId  string   `json:"deal_proposal_id"`
	DealProposalCid string   `json:"deal_proposal_cid"`
	HoursRemaining  int64    `json:"hours_remaining"`
	PieceSize       int64    `json:"piece_size"`
	PieceCid        string   `json:"piece_cid"`
	TenantID        int64    `json:"tenant_id"`
	TenantClientId  string   `json:"tenant_client_id"`
	DealStartTime   string   `json:"deal_start_time"`
	DealStartEpoch  int64    `json:"deal_start_epoch"`
	SampleImportCmd string   `json:"sample_import_cmd"`
	Sources         []Source `json:"sources"`
}

func UnmarshalAvailableDeals(data []byte) (AvailableDeals, error) {
	var r AvailableDeals
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *AvailableDeals) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type AvailableDeals struct {
	RequestID          string          `json:"request_id"`
	ResponseTimestamp  string          `json:"response_timestamp"`
	ResponseStateEpoch int64           `json:"response_state_epoch"`
	ResponseCode       int64           `json:"response_code"`
	InfoLines          []string        `json:"info_lines"`
	ResponseEntries    int64           `json:"response_entries"`
	Response           []EvergreenDeal `json:"response"`
}

type EvergreenDeal struct {
	PieceCid         string   `json:"piece_cid"`
	Tenants          []int64  `json:"tenants"`
	PaddedPieceSize  int64    `json:"padded_piece_size"`
	Sources          []Source `json:"sources"`
	SampleRequestCmd *string  `json:"sample_request_cmd,omitempty"`
}

type Source struct {
	SourceType         string `json:"source_type"`
	ProviderID         string `json:"provider_id"`
	DealID             int64  `json:"deal_id"`
	OriginalPayloadCid string `json:"original_payload_cid"`
	DealExpiration     string `json:"deal_expiration"`
	IsFilplus          bool   `json:"is_filplus"`
	SampleRetrieveCmd  string `json:"sample_retrieve_cmd"`
}
