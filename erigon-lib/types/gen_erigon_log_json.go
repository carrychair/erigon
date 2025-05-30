// Code generated by github.com/fjl/gencodec. DO NOT EDIT.

package types

import (
	"encoding/json"
	"errors"

	"github.com/erigontech/erigon-lib/common"
	"github.com/erigontech/erigon-lib/common/hexutil"
)

var _ = (*logMarshaling)(nil)

// MarshalJSON marshals as JSON.
func (l ErigonLog) MarshalJSON() ([]byte, error) {
	type ErigonLog struct {
		Address     common.Address `json:"address" gencodec:"required"`
		Topics      []common.Hash  `json:"topics" gencodec:"required"`
		Data        hexutil.Bytes  `json:"data" gencodec:"required"`
		BlockNumber hexutil.Uint64 `json:"blockNumber"`
		TxHash      common.Hash    `json:"transactionHash" gencodec:"required"`
		TxIndex     hexutil.Uint   `json:"transactionIndex"`
		BlockHash   common.Hash    `json:"blockHash"`
		Index       hexutil.Uint   `json:"logIndex"`
		Removed     bool           `json:"removed"`
		Timestamp   hexutil.Uint64 `json:"timestamp"`
	}

	var enc ErigonLog
	enc.Address = l.Address
	enc.Topics = l.Topics
	enc.Data = l.Data
	enc.BlockNumber = hexutil.Uint64(l.BlockNumber)
	enc.TxHash = l.TxHash
	enc.TxIndex = hexutil.Uint(l.TxIndex)
	enc.BlockHash = l.BlockHash
	enc.Index = hexutil.Uint(l.Index)
	enc.Removed = l.Removed
	enc.Timestamp = hexutil.Uint64(l.Timestamp)

	return json.Marshal(&enc)
}

// UnmarshalJSON unmarshals from JSON.
func (l *ErigonLog) UnmarshalJSON(input []byte) error {
	type ErigonLog struct {
		Address     *common.Address `json:"address" gencodec:"required"`
		Topics      []common.Hash   `json:"topics" gencodec:"required"`
		Data        *hexutil.Bytes  `json:"data" gencodec:"required"`
		BlockNumber *hexutil.Uint64 `json:"blockNumber"`
		TxHash      *common.Hash    `json:"transactionHash" gencodec:"required"`
		TxIndex     *hexutil.Uint   `json:"transactionIndex"`
		BlockHash   *common.Hash    `json:"blockHash"`
		Index       *hexutil.Uint   `json:"logIndex"`
		Removed     *bool           `json:"removed"`
		Timestamp   *hexutil.Uint64 `json:"timestamp"`
	}

	var dec ErigonLog
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	if dec.Address == nil {
		return errors.New("missing required field 'address' for ErigonLog")
	}
	l.Address = *dec.Address
	if dec.Topics == nil {
		return errors.New("missing required field 'topics' for ErigonLog")
	}
	l.Topics = dec.Topics
	if dec.Data == nil {
		return errors.New("missing required field 'data' for ErigonLog")
	}
	l.Data = *dec.Data
	if dec.BlockNumber != nil {
		l.BlockNumber = uint64(*dec.BlockNumber)
	}

	if dec.TxHash == nil {
		return errors.New("missing required field 'transactionHash' for ErigonLog")
	}
	l.TxHash = *dec.TxHash
	if dec.TxIndex != nil {
		l.TxIndex = uint(*dec.TxIndex)
	}
	if dec.BlockHash != nil {
		l.BlockHash = *dec.BlockHash
	}
	if dec.Index != nil {
		l.Index = uint(*dec.Index)
	}
	if dec.Removed != nil {
		l.Removed = *dec.Removed
	}

	if dec.Timestamp != nil {
		l.Timestamp = uint64(*dec.Timestamp)
	}

	return nil
}
