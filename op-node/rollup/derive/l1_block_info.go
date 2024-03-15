package derive

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ethereum-optimism/optimism/op-bindings/predeploys"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/solabi"
)

const (
	L1InfoFuncBedrockSignature = "setL1BlockValues(uint64,uint64,uint256,bytes32,uint64,bytes32,uint256,uint256)"
	L1InfoFuncEcotoneSignature = "setL1BlockValuesEcotone()"
	L1InfoFuncInteropSignature = "setL1BlockValuesInterop()"
	L1InfoArguments            = 8
	L1InfoBedrockLen           = 4 + 32*L1InfoArguments
	L1InfoEcotoneLen           = 4 + 32*5 // after Ecotone upgrade, args are packed into 5 32-byte slots
)

var (
	L1InfoFuncBedrockBytes4 = crypto.Keccak256([]byte(L1InfoFuncBedrockSignature))[:4]
	L1InfoFuncEcotoneBytes4 = crypto.Keccak256([]byte(L1InfoFuncEcotoneSignature))[:4]
	L1InfoFuncInteropBytes4 = crypto.Keccak256([]byte(L1InfoFuncInteropSignature))[:4]
	L1InfoDepositerAddress  = common.HexToAddress("0xdeaddeaddeaddeaddeaddeaddeaddeaddead0001")
	L1BlockAddress          = predeploys.L1BlockAddr
)

const (
	RegolithSystemTxGas = 1_000_000
)

// The following hardcoded chain IDs are for the first devnet release
var (
	InteropDependencySet = []*big.Int{big.NewInt(10666), big.NewInt(10777), big.NewInt(10888)}
)

// L1BlockInfo presents the information stored in a L1Block.setL1BlockValues call
type L1BlockInfo struct {
	Number    uint64
	Time      uint64
	BaseFee   *big.Int
	BlockHash common.Hash
	// Not strictly a piece of L1 information. Represents the number of L2 blocks since the start of the epoch,
	// i.e. when the actual L1 info was first introduced.
	SequenceNumber uint64
	// BatcherHash version 0 is just the address with 0 padding to the left.
	BatcherAddr common.Address

	L1FeeOverhead eth.Bytes32 // ignored after Ecotone upgrade
	L1FeeScalar   eth.Bytes32 // ignored after Ecotone upgrade

	BlobBaseFee       *big.Int // added by Ecotone upgrade
	BaseFeeScalar     uint32   // added by Ecotone upgrade
	BlobBaseFeeScalar uint32   // added by Ecotone upgrade

	DependencySet []*big.Int // added by Interop upgrade
}

// Bedrock Binary Format
// +---------+--------------------------+
// | Bytes   | Field                    |
// +---------+--------------------------+
// | 4       | Function signature       |
// | 32      | Number                   |
// | 32      | Time                     |
// | 32      | BaseFee                  |
// | 32      | BlockHash                |
// | 32      | SequenceNumber           |
// | 32      | BatcherHash              |
// | 32      | L1FeeOverhead            |
// | 32      | L1FeeScalar              |
// +---------+--------------------------+

func (info *L1BlockInfo) marshalBinaryBedrock() ([]byte, error) {
	w := bytes.NewBuffer(make([]byte, 0, L1InfoBedrockLen))
	if err := solabi.WriteSignature(w, L1InfoFuncBedrockBytes4); err != nil {
		return nil, err
	}
	if err := solabi.WriteUint64(w, info.Number); err != nil {
		return nil, err
	}
	if err := solabi.WriteUint64(w, info.Time); err != nil {
		return nil, err
	}
	if err := solabi.WriteUint256(w, info.BaseFee); err != nil {
		return nil, err
	}
	if err := solabi.WriteHash(w, info.BlockHash); err != nil {
		return nil, err
	}
	if err := solabi.WriteUint64(w, info.SequenceNumber); err != nil {
		return nil, err
	}
	if err := solabi.WriteAddress(w, info.BatcherAddr); err != nil {
		return nil, err
	}
	if err := solabi.WriteEthBytes32(w, info.L1FeeOverhead); err != nil {
		return nil, err
	}
	if err := solabi.WriteEthBytes32(w, info.L1FeeScalar); err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}

func (info *L1BlockInfo) unmarshalBinaryBedrock(data []byte) error {
	if len(data) != L1InfoBedrockLen {
		return fmt.Errorf("data is unexpected length: got %d, expected %d", len(data), L1InfoBedrockLen)
	}
	reader := bytes.NewReader(data)

	var err error
	if _, err := solabi.ReadAndValidateSignature(reader, L1InfoFuncBedrockBytes4); err != nil {
		return err
	}
	if info.Number, err = solabi.ReadUint64(reader); err != nil {
		return err
	}
	if info.Time, err = solabi.ReadUint64(reader); err != nil {
		return err
	}
	if info.BaseFee, err = solabi.ReadUint256(reader); err != nil {
		return err
	}
	if info.BlockHash, err = solabi.ReadHash(reader); err != nil {
		return err
	}
	if info.SequenceNumber, err = solabi.ReadUint64(reader); err != nil {
		return err
	}
	if info.BatcherAddr, err = solabi.ReadAddress(reader); err != nil {
		return err
	}
	if info.L1FeeOverhead, err = solabi.ReadEthBytes32(reader); err != nil {
		return err
	}
	if info.L1FeeScalar, err = solabi.ReadEthBytes32(reader); err != nil {
		return err
	}
	if !solabi.EmptyReader(reader) {
		return errors.New("too many bytes")
	}
	return nil
}

// Ecotone Binary Format
// +---------+--------------------------+
// | Bytes   | Field                    |
// +---------+--------------------------+
// | 4       | Function signature       |
// | 4       | BaseFeeScalar            |
// | 4       | BlobBaseFeeScalar        |
// | 8       | SequenceNumber           |
// | 8       | Timestamp                |
// | 8       | L1BlockNumber            |
// | 32      | BaseFee                  |
// | 32      | BlobBaseFee              |
// | 32      | BlockHash                |
// | 32      | BatcherHash              |
// +---------+--------------------------+

func (info *L1BlockInfo) marshalBinaryEcotone() ([]byte, error) {
	w := bytes.NewBuffer(make([]byte, 0, L1InfoEcotoneLen))
	if err := solabi.WriteSignature(w, L1InfoFuncEcotoneBytes4); err != nil {
		return nil, err
	}
	if err := binary.Write(w, binary.BigEndian, info.BaseFeeScalar); err != nil {
		return nil, err
	}
	if err := binary.Write(w, binary.BigEndian, info.BlobBaseFeeScalar); err != nil {
		return nil, err
	}
	if err := binary.Write(w, binary.BigEndian, info.SequenceNumber); err != nil {
		return nil, err
	}
	if err := binary.Write(w, binary.BigEndian, info.Time); err != nil {
		return nil, err
	}
	if err := binary.Write(w, binary.BigEndian, info.Number); err != nil {
		return nil, err
	}
	if err := solabi.WriteUint256(w, info.BaseFee); err != nil {
		return nil, err
	}
	blobBasefee := info.BlobBaseFee
	if blobBasefee == nil {
		blobBasefee = big.NewInt(1) // set to 1, to match the min blob basefee as defined in EIP-4844
	}
	if err := solabi.WriteUint256(w, blobBasefee); err != nil {
		return nil, err
	}
	if err := solabi.WriteHash(w, info.BlockHash); err != nil {
		return nil, err
	}
	// ABI encoding will perform the left-padding with zeroes to 32 bytes, matching the "batcherHash" SystemConfig format and version 0 byte.
	if err := solabi.WriteAddress(w, info.BatcherAddr); err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}

func (info *L1BlockInfo) unmarshalBinaryEcotone(data []byte) error {
	if len(data) != L1InfoEcotoneLen {
		return fmt.Errorf("data is unexpected length: got %d, expected %d", len(data), L1InfoBedrockLen)
	}
	r := bytes.NewReader(data)

	var err error
	if _, err := solabi.ReadAndValidateSignature(r, L1InfoFuncEcotoneBytes4); err != nil {
		return err
	}
	if err := binary.Read(r, binary.BigEndian, &info.BaseFeeScalar); err != nil {
		return fmt.Errorf("invalid ecotone l1 block info format")
	}
	if err := binary.Read(r, binary.BigEndian, &info.BlobBaseFeeScalar); err != nil {
		return fmt.Errorf("invalid ecotone l1 block info format")
	}
	if err := binary.Read(r, binary.BigEndian, &info.SequenceNumber); err != nil {
		return fmt.Errorf("invalid ecotone l1 block info format")
	}
	if err := binary.Read(r, binary.BigEndian, &info.Time); err != nil {
		return fmt.Errorf("invalid ecotone l1 block info format")
	}
	if err := binary.Read(r, binary.BigEndian, &info.Number); err != nil {
		return fmt.Errorf("invalid ecotone l1 block info format")
	}
	if info.BaseFee, err = solabi.ReadUint256(r); err != nil {
		return err
	}
	if info.BlobBaseFee, err = solabi.ReadUint256(r); err != nil {
		return err
	}
	if info.BlockHash, err = solabi.ReadHash(r); err != nil {
		return err
	}
	// The "batcherHash" will be correctly parsed as address, since the version 0 and left-padding matches the ABI encoding format.
	if info.BatcherAddr, err = solabi.ReadAddress(r); err != nil {
		return err
	}
	if !solabi.EmptyReader(r) {
		return errors.New("too many bytes")
	}
	return nil
}

// Interop Binary Format
// +----------------------+----------------------+
// | Bytes                | Field                |
// +----------------------+----------------------+
// | 4                    | Function signature   |
// | 4                    | BaseFeeScalar        |
// | 4                    | BlobBaseFeeScalar    |
// | 8                    | SequenceNumber       |
// | 8                    | Timestamp            |
// | 8                    | L1BlockNumber        |
// | 32                   | BaseFee              |
// | 32                   | BlobBaseFee          |
// | 32                   | BlockHash            |
// | 32                   | BatcherHash          |
// | 1       		      | DependencySetSize    |
// | 32*DependencySetSize | DependencySet        |
// +----------------------+----------------------+

func (info *L1BlockInfo) marshalBinaryInterop() ([]byte, error) {
	w := bytes.NewBuffer(make([]byte, 0, L1InfoInteropLen()))
	if err := solabi.WriteSignature(w, L1InfoFuncInteropBytes4); err != nil {
		return nil, err
	}
	if err := binary.Write(w, binary.BigEndian, info.BaseFeeScalar); err != nil {
		return nil, err
	}
	if err := binary.Write(w, binary.BigEndian, info.BlobBaseFeeScalar); err != nil {
		return nil, err
	}
	if err := binary.Write(w, binary.BigEndian, info.SequenceNumber); err != nil {
		return nil, err
	}
	if err := binary.Write(w, binary.BigEndian, info.Time); err != nil {
		return nil, err
	}
	if err := binary.Write(w, binary.BigEndian, info.Number); err != nil {
		return nil, err
	}
	if err := solabi.WriteUint256(w, info.BaseFee); err != nil {
		return nil, err
	}
	blobBasefee := info.BlobBaseFee
	if blobBasefee == nil {
		blobBasefee = big.NewInt(1) // set to 1, to match the min blob basefee as defined in EIP-4844
	}
	if err := solabi.WriteUint256(w, blobBasefee); err != nil {
		return nil, err
	}
	if err := solabi.WriteHash(w, info.BlockHash); err != nil {
		return nil, err
	}
	// ABI encoding will perform the left-padding with zeroes to 32 bytes, matching the "batcherHash" SystemConfig format and version 0 byte.
	if err := solabi.WriteAddress(w, info.BatcherAddr); err != nil {
		return nil, err
	}
	if err := binary.Write(w, binary.BigEndian, info.DependencySetSize()); err != nil {
		return nil, err
	}
	for _, chainID := range info.DependencySet {
		if err := solabi.WriteUint256(w, chainID); err != nil {
			return nil, err
		}
	}

	return w.Bytes(), nil
}

func (info *L1BlockInfo) unmarshalBinaryInterop(data []byte) error {
	r := bytes.NewReader(data)

	var err error
	if _, err := solabi.ReadAndValidateSignature(r, L1InfoFuncInteropBytes4); err != nil {
		return err
	}
	if err := binary.Read(r, binary.BigEndian, &info.BaseFeeScalar); err != nil {
		return fmt.Errorf("invalid interop l1 block info format")
	}
	if err := binary.Read(r, binary.BigEndian, &info.BlobBaseFeeScalar); err != nil {
		return fmt.Errorf("invalid interop l1 block info format")
	}
	if err := binary.Read(r, binary.BigEndian, &info.SequenceNumber); err != nil {
		return fmt.Errorf("invalid interop l1 block info format")
	}
	if err := binary.Read(r, binary.BigEndian, &info.Time); err != nil {
		return fmt.Errorf("invalid interop l1 block info format")
	}
	if err := binary.Read(r, binary.BigEndian, &info.Number); err != nil {
		return fmt.Errorf("invalid interop l1 block info format")
	}
	if info.BaseFee, err = solabi.ReadUint256(r); err != nil {
		return err
	}
	if info.BlobBaseFee, err = solabi.ReadUint256(r); err != nil {
		return err
	}
	if info.BlockHash, err = solabi.ReadHash(r); err != nil {
		return err
	}
	// The "batcherHash" will be correctly parsed as address, since the version 0 and left-padding matches the ABI encoding format.
	if info.BatcherAddr, err = solabi.ReadAddress(r); err != nil {
		return err
	}

	var dependencySetSize uint8

	if err := binary.Read(r, binary.BigEndian, &dependencySetSize); err != nil {
		return fmt.Errorf("invalid interop l1 block info format")
	}

	// we make the check here because it's the soonest InteroptSetSize is available, which is needed to calculate the expected length
	if len(data) != L1InfoInteropLen() {
		return fmt.Errorf("data is unexpected length: got %d, expected %d", len(data), L1InfoBedrockLen)
	}

	info.DependencySet = make([]*big.Int, dependencySetSize)
	for i := uint8(0); i < dependencySetSize; i++ {
		if info.DependencySet[i], err = solabi.ReadUint256(r); err != nil {
			return err
		}
	}

	if !solabi.EmptyReader(r) {
		return errors.New("too many bytes")
	}
	return nil
}

func (info *L1BlockInfo) DependencySetSize() uint8 {
	return uint8(len(info.DependencySet))
}

// isEcotoneButNotFirstBlock returns whether the specified block is subject to the Ecotone upgrade,
// but is not the actiation block itself.
func isEcotoneButNotFirstBlock(rollupCfg *rollup.Config, l2BlockTime uint64) bool {
	return rollupCfg.IsEcotone(l2BlockTime) && !rollupCfg.IsEcotoneActivationBlock(l2BlockTime)
}

// isInteropButNotFirstBlock returns whether the specified block is subject to the Interop upgrade,
// but is not the activation block itself.
func isInteropButNotFirstBlock(rollupCfg *rollup.Config, l2BlockTime uint64) bool {
	return rollupCfg.IsInterop(l2BlockTime) && !rollupCfg.IsInteropActivationBlock(l2BlockTime)
}

// L1BlockInfoFromBytes is the inverse of L1InfoDeposit, to see where the L2 chain is derived from
func L1BlockInfoFromBytes(rollupCfg *rollup.Config, l2BlockTime uint64, data []byte) (*L1BlockInfo, error) {
	var info L1BlockInfo
	if isInteropButNotFirstBlock(rollupCfg, l2BlockTime) {
		return &info, info.unmarshalBinaryInterop(data)
	} else if isEcotoneButNotFirstBlock(rollupCfg, l2BlockTime) {
		return &info, info.unmarshalBinaryEcotone(data)
	}
	return &info, info.unmarshalBinaryBedrock(data)
}

// L1InfoDeposit creates a L1 Info deposit transaction based on the L1 block,
// and the L2 block-height difference with the start of the epoch.
func L1InfoDeposit(rollupCfg *rollup.Config, sysCfg eth.SystemConfig, seqNumber uint64, block eth.BlockInfo, l2BlockTime uint64) (*types.DepositTx, error) {
	l1BlockInfo := L1BlockInfo{
		Number:         block.NumberU64(),
		Time:           block.Time(),
		BaseFee:        block.BaseFee(),
		BlockHash:      block.Hash(),
		SequenceNumber: seqNumber,
		BatcherAddr:    sysCfg.BatcherAddr,
	}
	var data []byte
	if isInteropButNotFirstBlock(rollupCfg, l2BlockTime) {
		l1BlockInfo.BlobBaseFee = block.BlobBaseFee()
		if l1BlockInfo.BlobBaseFee == nil {
			// The L2 spec states to use the MIN_BLOB_GASPRICE from EIP-4844 if not yet active on L1.
			l1BlockInfo.BlobBaseFee = big.NewInt(1)
		}
		blobBaseFeeScalar, baseFeeScalar, err := sysCfg.EcotoneScalars()
		if err != nil {
			return nil, err
		}
		l1BlockInfo.BlobBaseFeeScalar = blobBaseFeeScalar
		l1BlockInfo.BaseFeeScalar = baseFeeScalar
		l1BlockInfo.DependencySet = InteropDependencySet
		out, err := l1BlockInfo.marshalBinaryInterop()
		if err != nil {
			return nil, fmt.Errorf("failed to marshal Interop l1 block info: %w", err)
		}
		data = out
	} else if isEcotoneButNotFirstBlock(rollupCfg, l2BlockTime) {
		l1BlockInfo.BlobBaseFee = block.BlobBaseFee()
		if l1BlockInfo.BlobBaseFee == nil {
			// The L2 spec states to use the MIN_BLOB_GASPRICE from EIP-4844 if not yet active on L1.
			l1BlockInfo.BlobBaseFee = big.NewInt(1)
		}
		blobBaseFeeScalar, baseFeeScalar, err := sysCfg.EcotoneScalars()
		if err != nil {
			return nil, err
		}
		l1BlockInfo.BlobBaseFeeScalar = blobBaseFeeScalar
		l1BlockInfo.BaseFeeScalar = baseFeeScalar
		out, err := l1BlockInfo.marshalBinaryEcotone()
		if err != nil {
			return nil, fmt.Errorf("failed to marshal Ecotone l1 block info: %w", err)
		}
		data = out
	} else {
		l1BlockInfo.L1FeeOverhead = sysCfg.Overhead
		l1BlockInfo.L1FeeScalar = sysCfg.Scalar
		out, err := l1BlockInfo.marshalBinaryBedrock()
		if err != nil {
			return nil, fmt.Errorf("failed to marshal Bedrock l1 block info: %w", err)
		}
		data = out
	}

	source := L1InfoDepositSource{
		L1BlockHash: block.Hash(),
		SeqNumber:   seqNumber,
	}
	// Set a very large gas limit with `IsSystemTransaction` to ensure
	// that the L1 Attributes Transaction does not run out of gas.
	out := &types.DepositTx{
		SourceHash:          source.SourceHash(),
		From:                L1InfoDepositerAddress,
		To:                  &L1BlockAddress,
		Mint:                nil,
		Value:               big.NewInt(0),
		Gas:                 150_000_000,
		IsSystemTransaction: true,
		Data:                data,
	}
	// With the regolith fork we disable the IsSystemTx functionality, and allocate real gas
	if rollupCfg.IsRegolith(l2BlockTime) {
		out.IsSystemTransaction = false
		out.Gas = RegolithSystemTxGas
	}
	return out, nil
}

// L1InfoDepositBytes returns a serialized L1-info attributes transaction.
func L1InfoDepositBytes(rollupCfg *rollup.Config, sysCfg eth.SystemConfig, seqNumber uint64, l1Info eth.BlockInfo, l2BlockTime uint64) ([]byte, error) {
	dep, err := L1InfoDeposit(rollupCfg, sysCfg, seqNumber, l1Info, l2BlockTime)
	if err != nil {
		return nil, fmt.Errorf("failed to create L1 info tx: %w", err)
	}
	l1Tx := types.NewTx(dep)
	opaqueL1Tx, err := l1Tx.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to encode L1 info tx: %w", err)
	}
	return opaqueL1Tx, nil
}

func L1InfoInteropLen() int {
	return 4 + 32*5 + 1 + 32*int(len(InteropDependencySet))
}