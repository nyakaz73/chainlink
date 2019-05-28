package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/smartcontractkit/chainlink/core/utils"
	"github.com/tidwall/gjson"
	"go.uber.org/multierr"
)

//go:generate gencodec -type Log -field-override logMarshaling -out gen_log_json.go

// Log represents a contract log event. These events are generated by the LOG opcode and
// stored/indexed by the node.
type Log struct {
	// Consensus fields:
	// address of the contract that generated the event
	Address common.Address `json:"address" gencodec:"required"`
	// list of topics provided by the contract.
	Topics []common.Hash `json:"topics" gencodec:"required"`
	// supplied by the contract, usually ABI-encoded
	Data []byte `json:"data" gencodec:"required"`

	// Derived fields. These fields are filled in by the node
	// but not secured by consensus.
	// block in which the transaction was included
	BlockNumber uint64 `json:"blockNumber"`
	// hash of the transaction
	TxHash common.Hash `json:"transactionHash"`
	// index of the transaction in the block
	TxIndex uint `json:"transactionIndex"`
	// hash of the block in which the transaction was included
	BlockHash common.Hash `json:"blockHash"`
	// index of the log in the receipt
	Index uint `json:"logIndex"`

	// The Removed field is true if this log was reverted due to a chain reorganisation.
	// You must pay attention to this field if you receive logs through a filter query.
	Removed bool `json:"removed"`
}

func (log Log) getTopic(idx uint) (common.Hash, error) {
	if len(log.Topics) <= int(idx) {
		return common.Hash{}, fmt.Errorf("Log: Unable to get topic #%v for %v", idx, log)
	}

	return log.Topics[idx], nil
}

type logMarshaling struct {
	Data        hexutil.Bytes
	BlockNumber hexutil.Uint64
	TxIndex     hexutil.Uint
	Index       hexutil.Uint
}

// Tx contains fields necessary for an Ethereum transaction with
// an additional field for the TxAttempt.
type Tx struct {
	ID       uint64         `gorm:"primary_key;auto_increment"`
	From     common.Address `gorm:"index;not null"`
	To       common.Address `gorm:"not null"`
	Data     []byte
	Nonce    uint64 `gorm:"index"`
	Value    *Big   `gorm:"type:varchar(255)"`
	GasLimit uint64
	// TxAttempt fields manually included; can't embed another primary_key
	Hash      common.Hash
	GasPrice  *Big `gorm:"type:varchar(255)"`
	Confirmed bool
	Hex       string `gorm:"type:text"`
	SentAt    uint64
}

// EthTx creates a new Ethereum transaction with a given gasPrice in wei
// that is ready to be signed.
func (tx Tx) EthTx(gasPriceWei *big.Int) *types.Transaction {
	return types.NewTransaction(
		tx.Nonce,
		tx.To,
		tx.Value.ToInt(),
		tx.GasLimit,
		gasPriceWei,
		tx.Data,
	)
}

// AssignTxAttempt assigns the values of the attempt to the top level Tx.
func (tx *Tx) AssignTxAttempt(txat *TxAttempt) {
	tx.Hash = txat.Hash
	tx.GasPrice = txat.GasPrice
	tx.Confirmed = txat.Confirmed
	tx.Hex = txat.Hex
	tx.SentAt = txat.SentAt
}

// TxAttempt is used for keeping track of transactions that
// have been written to the Ethereum blockchain. This makes
// it so that if the network is busy, a transaction can be
// resubmitted with a higher GasPrice.
type TxAttempt struct {
	Hash      common.Hash `gorm:"primary_key;not null"`
	TxID      uint64      `gorm:"index"`
	GasPrice  *Big        `gorm:"type:varchar(255)"`
	Confirmed bool
	Hex       string `gorm:"type:text"`
	SentAt    uint64
	CreatedAt time.Time `gorm:"index"`
}

// GetID returns the ID of this structure for jsonapi serialization.
func (txa TxAttempt) GetID() string {
	return txa.Hash.Hex()
}

// GetName returns the pluralized "type" of this structure for jsonapi serialization.
func (txa TxAttempt) GetName() string {
	return "txattempts"
}

// SetID is used to set the ID of this structure when deserializing from jsonapi documents.
func (txa *TxAttempt) SetID(value string) error {
	txa.Hash = common.HexToHash(value)
	return nil
}

// FunctionSelector is the first four bytes of the call data for a
// function call and specifies the function to be called.
type FunctionSelector [FunctionSelectorLength]byte

// FunctionSelectorLength should always be a length of 4 as a byte.
const FunctionSelectorLength = 4

// BytesToFunctionSelector converts the given bytes to a FunctionSelector.
func BytesToFunctionSelector(b []byte) FunctionSelector {
	var f FunctionSelector
	f.SetBytes(b)
	return f
}

// HexToFunctionSelector converts the given string to a FunctionSelector.
func HexToFunctionSelector(s string) FunctionSelector {
	return BytesToFunctionSelector(common.FromHex(s))
}

// String returns the FunctionSelector as a string type.
func (f FunctionSelector) String() string { return hexutil.Encode(f[:]) }

// Bytes returns the FunctionSelector as a byte slice
func (f FunctionSelector) Bytes() []byte { return f[:] }

// WithoutPrefix returns the FunctionSelector as a string without the '0x' prefix.
func (f FunctionSelector) WithoutPrefix() string { return f.String()[2:] }

// SetBytes sets the FunctionSelector to that of the given bytes (will trim).
func (f *FunctionSelector) SetBytes(b []byte) { copy(f[:], b[:FunctionSelectorLength]) }

// UnmarshalJSON parses the raw FunctionSelector and sets the FunctionSelector
// type to the given input.
func (f *FunctionSelector) UnmarshalJSON(input []byte) error {
	var s string
	err := json.Unmarshal(input, &s)
	if err != nil {
		return err
	}

	if utils.HasHexPrefix(s) {
		bytes := common.FromHex(s)
		if len(bytes) != FunctionSelectorLength {
			return errors.New("Function ID must be 4 bytes in length")
		}
		f.SetBytes(bytes)
	} else {
		bytes, err := utils.Keccak256([]byte(s))
		if err != nil {
			return err
		}
		f.SetBytes(bytes[0:4])
	}

	return nil
}

// BlockHeader represents a block header in the Ethereum blockchain.
// Deliberately does not have required fields because some fields aren't
// present depending on the Ethereum node.
// i.e. Parity does not always send mixHash
type BlockHeader struct {
	ParentHash  common.Hash      `json:"parentHash"`
	UncleHash   common.Hash      `json:"sha3Uncles"`
	Coinbase    common.Address   `json:"miner"`
	Root        common.Hash      `json:"stateRoot"`
	TxHash      common.Hash      `json:"transactionsRoot"`
	ReceiptHash common.Hash      `json:"receiptsRoot"`
	Bloom       types.Bloom      `json:"logsBloom"`
	Difficulty  hexutil.Big      `json:"difficulty"`
	Number      hexutil.Big      `json:"number"`
	GasLimit    hexutil.Uint64   `json:"gasLimit"`
	GasUsed     hexutil.Uint64   `json:"gasUsed"`
	Time        hexutil.Big      `json:"timestamp"`
	Extra       hexutil.Bytes    `json:"extraData"`
	Nonce       types.BlockNonce `json:"nonce"`
	GethHash    common.Hash      `json:"mixHash"`
	ParityHash  common.Hash      `json:"hash"`
}

var emptyHash = common.Hash{}

// Hash will return GethHash if it exists otherwise it returns the ParityHash
func (h BlockHeader) Hash() common.Hash {
	if h.GethHash != emptyHash {
		return h.GethHash
	}
	return h.ParityHash
}

// ToHead converts a given BlockHeader to a Head instance.
func (h BlockHeader) ToHead() *Head {
	return NewHead(h.Number.ToInt(), h.Hash())
}

// Head represents a BlockNumber, BlockHash.
type Head struct {
	HashRaw string `gorm:"primary_key;type:varchar;column:hash"`
	Number  int64  `gorm:"index;type:bigint;not null"`
}

// NewHead returns a Head instance with a BlockNumber and BlockHash.
func NewHead(bigint *big.Int, hash common.Hash) *Head {
	if bigint == nil {
		return nil
	}

	return &Head{
		Number:  bigint.Int64(),
		HashRaw: hash.Hex()[2:], // remove 0x
	}
}

// Hash returns the Hash instance related to this block height.
func (l *Head) Hash() common.Hash {
	return common.HexToHash(l.HashRaw)
}

// String returns a string representation of this number.
func (l *Head) String() string {
	return l.ToInt().String()
}

// ToInt return the height as a *big.Int. Also handles nil by returning nil.
func (l *Head) ToInt() *big.Int {
	if l == nil {
		return nil
	}
	return big.NewInt(l.Number)
}

// GreaterThan compares BlockNumbers and returns true if the reciever BlockNumber is greater than
// the supplied BlockNumber
func (l *Head) GreaterThan(r *Head) bool {
	if l == nil {
		return false
	}
	if l != nil && r == nil {
		return true
	}
	return l.Number > r.Number
}

// NextInt returns the next BlockNumber as big.int, or nil if nil to represent latest.
func (l *Head) NextInt() *big.Int {
	if l == nil {
		return nil
	}
	return new(big.Int).Add(l.ToInt(), big.NewInt(1))
}

// EthSubscription should implement Err() <-chan error and Unsubscribe()
type EthSubscription interface {
	Err() <-chan error
	Unsubscribe()
}

// Key holds the private key metadata for a given address that is used to unlock
// said key when given a password.
type Key struct {
	Address EIP55Address `gorm:"primary_key;type:varchar(64)"`
	JSON    JSON         `gorm:"type:text"`
}

// NewKeyFromFile creates an instance in memory from a key file on disk.
func NewKeyFromFile(path string) (*Key, error) {
	dat, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	js := gjson.ParseBytes(dat)
	address, err := NewEIP55Address(common.HexToAddress(js.Get("address").String()).Hex())
	if err != nil {
		return nil, multierr.Append(errors.New("unable to create Key model"), err)
	}

	return &Key{
		Address: address,
		JSON:    JSON{Result: js},
	}, nil
}

// WriteToDisk writes this key to disk at the passed path.
func (k *Key) WriteToDisk(path string) error {
	return ioutil.WriteFile(path, []byte(k.JSON.String()), 0700)
}

// TxReceipt holds the block number and the transaction hash of a signed
// transaction that has been written to the blockchain.
type TxReceipt struct {
	BlockNumber *Big        `json:"blockNumber" gorm:"type:numeric"`
	Hash        common.Hash `json:"transactionHash"`
	Logs        []Log       `json:"logs"`
}

// Unconfirmed returns true if the transaction is not confirmed.
func (txr *TxReceipt) Unconfirmed() bool {
	return txr.Hash == emptyHash || txr.BlockNumber == nil
}

func (txr TxReceipt) FulfilledRunLog() bool {
	for _, log := range txr.Logs {
		if log.Topics[0] == ChainlinkFulfilledTopic {
			return true
		}
	}
	return false
}
