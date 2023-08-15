package database

import (
	"errors"
	"math/big"

	"gorm.io/gorm"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	"github.com/google/uuid"
)

/**
 * Types
 */

type ContractEvent struct {
	GUID uuid.UUID `gorm:"primaryKey"`

	// Some useful derived fields
	BlockHash       common.Hash    `gorm:"serializer:json"`
	ContractAddress common.Address `gorm:"serializer:json"`
	TransactionHash common.Hash    `gorm:"serializer:json"`
	LogIndex        uint64

	EventSignature common.Hash `gorm:"serializer:json"`
	Timestamp      uint64

	// NOTE: NOT ALL THE DERIVED FIELDS ON `types.Log` ARE
	// AVAILABLE. FIELDS LISTED ABOVE ARE FILLED IN
	RLPLog *types.Log `gorm:"serializer:rlp;column:rlp_bytes"`
}

func ContractEventFromLog(log *types.Log, timestamp uint64) ContractEvent {
	eventSig := common.Hash{}
	if len(log.Topics) > 0 {
		eventSig = log.Topics[0]
	}

	return ContractEvent{
		GUID: uuid.New(),

		BlockHash:       log.BlockHash,
		TransactionHash: log.TxHash,
		ContractAddress: log.Address,

		EventSignature: eventSig,
		LogIndex:       uint64(log.Index),

		Timestamp: timestamp,

		RLPLog: log,
	}
}

func (c *ContractEvent) AfterFind(tx *gorm.DB) error {
	// Fill in some of the derived fields that are not
	// populated when decoding the RLPLog from RLP
	c.RLPLog.BlockHash = c.BlockHash
	c.RLPLog.TxHash = c.TransactionHash
	c.RLPLog.Index = uint(c.LogIndex)
	return nil
}

type L1ContractEvent struct {
	ContractEvent `gorm:"embedded"`
}

type L2ContractEvent struct {
	ContractEvent `gorm:"embedded"`
}

type ContractEventsView interface {
	L1ContractEvent(uuid.UUID) (*L1ContractEvent, error)
	L1ContractEventByTxLogIndex(common.Hash, uint64) (*L1ContractEvent, error)
	L1ContractEventsWithFilter(ContractEvent, *big.Int, *big.Int) ([]L1ContractEvent, error)

	L2ContractEvent(uuid.UUID) (*L2ContractEvent, error)
	L2ContractEventByTxLogIndex(common.Hash, uint64) (*L2ContractEvent, error)
	L2ContractEventsWithFilter(ContractEvent, *big.Int, *big.Int) ([]L2ContractEvent, error)

	// TEMP -- will clean up this code
	ContractEventsWithFilter(ContractEvent, string, *big.Int, *big.Int) ([]ContractEvent, error)
}

type ContractEventsDB interface {
	ContractEventsView

	StoreL1ContractEvents([]*L1ContractEvent) error
	StoreL2ContractEvents([]*L2ContractEvent) error
}

/**
 * Implementation
 */

type contractEventsDB struct {
	gorm *gorm.DB
}

func newContractEventsDB(db *gorm.DB) ContractEventsDB {
	return &contractEventsDB{gorm: db}
}

// L1

func (db *contractEventsDB) StoreL1ContractEvents(events []*L1ContractEvent) error {
	result := db.gorm.Create(&events)
	return result.Error
}

func (db *contractEventsDB) L1ContractEvent(uuid uuid.UUID) (*L1ContractEvent, error) {
	var l1ContractEvent L1ContractEvent
	result := db.gorm.Where(&ContractEvent{GUID: uuid}).Take(&l1ContractEvent)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, result.Error
	}

	return &l1ContractEvent, nil
}

func (db *contractEventsDB) L1ContractEventByTxLogIndex(txHash common.Hash, logIndex uint64) (*L1ContractEvent, error) {
	var l1ContractEvent L1ContractEvent
	result := db.gorm.Where(&ContractEvent{TransactionHash: txHash, LogIndex: logIndex}).Take(&l1ContractEvent)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, result.Error
	}

	return &l1ContractEvent, nil
}

func (db *contractEventsDB) L1ContractEventsWithFilter(filter ContractEvent, fromHeight, toHeight *big.Int) ([]L1ContractEvent, error) {
	if fromHeight == nil {
		fromHeight = big.NewInt(0)
	}

	query := db.gorm.Table("l1_contract_events").Where(&filter)
	query = query.Joins("INNER JOIN l1_block_headers ON l1_contract_events.block_hash = l1_block_headers.hash")
	query = query.Where("l1_block_headers.number >= ? AND l1_block_headers.number <= ?", fromHeight, toHeight)
	query = query.Order("l1_block_headers.number ASC").Select("l1_contract_events.*")

	// NOTE: We use `Find` here instead of `Scan` since `Scan` doesn't not support
	// model hooks like `ContractEvent#AfterFind`. Functionally they are the same
	var events []L1ContractEvent
	result := query.Find(&events)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, result.Error
	}

	return events, nil
}

// L2

func (db *contractEventsDB) StoreL2ContractEvents(events []*L2ContractEvent) error {
	result := db.gorm.Create(&events)
	return result.Error
}

func (db *contractEventsDB) L2ContractEvent(uuid uuid.UUID) (*L2ContractEvent, error) {
	var l2ContractEvent L2ContractEvent
	result := db.gorm.Where(&ContractEvent{GUID: uuid}).Take(&l2ContractEvent)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, result.Error
	}

	return &l2ContractEvent, nil
}

func (db *contractEventsDB) L2ContractEventByTxLogIndex(txHash common.Hash, logIndex uint64) (*L2ContractEvent, error) {
	var l2ContractEvent L2ContractEvent
	result := db.gorm.Where(&ContractEvent{TransactionHash: txHash, LogIndex: logIndex}).Take(&l2ContractEvent)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, result.Error
	}

	return &l2ContractEvent, nil
}

func (db *contractEventsDB) L2ContractEventsWithFilter(filter ContractEvent, fromHeight, toHeight *big.Int) ([]L2ContractEvent, error) {
	if fromHeight == nil {
		fromHeight = big.NewInt(0)
	}

	query := db.gorm.Table("l2_contract_events").Where(&filter)
	query = query.Joins("INNER JOIN l2_block_headers ON l2_contract_events.block_hash = l2_block_headers.hash")
	query = query.Where("l2_block_headers.number >= ? AND l2_block_headers.number <= ?", fromHeight, toHeight)
	query = query.Order("l2_block_headers.number ASC").Select("l2_contract_events.*")

	// NOTE: We use `Find` here instead of `Scan` since `Scan` doesn't not support
	// model hooks like `ContractEvent#AfterFind`. Functionally they are the same
	var events []L2ContractEvent
	result := query.Find(&events)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, result.Error
	}

	return events, nil
}

// Auxilliary methods for both L1 and L2

func (db *contractEventsDB) ContractEventsWithFilter(filter ContractEvent, chain string, fromHeight, toHeight *big.Int) ([]ContractEvent, error) {
	switch chain {
	case "l1":
		l1Events, err := db.L1ContractEventsWithFilter(filter, fromHeight, toHeight)
		if err != nil {
			return nil, err
		}
		events := make([]ContractEvent, len(l1Events))
		for i := range l1Events {
			events[i] = l1Events[i].ContractEvent
		}
		return events, nil
	case "l2":
		l2Events, err := db.L2ContractEventsWithFilter(filter, fromHeight, toHeight)
		if err != nil {
			return nil, err
		}
		events := make([]ContractEvent, len(l2Events))
		for i := range l2Events {
			events[i] = l2Events[i].ContractEvent
		}
		return events, nil
	default:
		return nil, errors.New("expected 'l1' or 'l2' for chain type")
	}
}
