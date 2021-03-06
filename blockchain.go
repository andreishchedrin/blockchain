package main

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/dgraph-io/badger/v3"
	"log"
	"os"
)

const database = "b_%s.db"
const prefix = "blocks"
const genesisCoinbaseData = "The Times 03/Jan/2009 Chancellor on brink of second bailout for banks"

type Blockchain struct {
	tip []byte
	db  *badger.DB
}

func NewBlockchain(nodeID string) *Blockchain {
	dbFile := fmt.Sprintf(database, nodeID)
	if dbExists(dbFile) == false {
		fmt.Println("No existing blockchain found. Create one first.")
		os.Exit(1)
	}

	var tip []byte
	db, _ := badger.Open(badger.DefaultOptions(dbFile))

	db.Update(func(txn *badger.Txn) error {
		//b := txn.Bucket([]byte(bucket))
		item, _ := txn.Get([]byte(prefix + "l"))

		_ = item.Value(func(val []byte) error {
			tip = val
			return nil
		})

		return nil
	})

	bc := Blockchain{tip, db}

	return &bc
}

func CreateBlockchain(address, nodeID string) *Blockchain {
	dbFile := fmt.Sprintf(database, nodeID)
	if dbExists(dbFile) {
		fmt.Println("Blockchain already exists.")
		os.Exit(1)
	}

	var tip []byte
	cbtx := NewCoinbaseTX(address, genesisCoinbaseData)
	genesis := NewGenesisBlock(cbtx)

	db, _ := badger.Open(badger.DefaultOptions(dbFile))

	db.Update(func(txn *badger.Txn) error {
		//b, _ := tx.CreateBucket([]byte(bucket))
		p := []byte(prefix)
		txn.Set(append(p, genesis.Hash...), genesis.Serialize())
		txn.Set([]byte(prefix+"l"), genesis.Hash)

		tip = genesis.Hash

		return nil
	})

	bc := Blockchain{tip, db}
	return &bc
}

func (bc *Blockchain) AddBlock(block *Block) {
	bc.db.Update(func(txn *badger.Txn) error {
		//b := tx.Bucket([]byte(bucket))
		p := []byte(prefix)
		item, _ := txn.Get(append(p, block.Hash...))

		var blockInDb []byte
		_ = item.Value(func(val []byte) error {
			blockInDb = val
			return nil
		})

		if blockInDb != nil {
			return nil
		}

		blockData := block.Serialize()
		err := txn.Set(append(p, block.Hash...), blockData)
		if err != nil {
			log.Panic(err)
		}

		item, _ = txn.Get([]byte(prefix + "l"))

		var lastHash []byte
		_ = item.Value(func(val []byte) error {
			lastHash = val
			return nil
		})

		item, _ = txn.Get(append(p, lastHash...))

		var lastBlockData []byte
		_ = item.Value(func(val []byte) error {
			lastBlockData = val
			return nil
		})
		lastBlock := DeserializeBlock(lastBlockData)

		if block.Height > lastBlock.Height {
			err = txn.Set([]byte(prefix+"l"), block.Hash)
			if err != nil {
				log.Panic(err)
			}
			bc.tip = block.Hash
		}

		return nil
	})
}

func (bc *Blockchain) GetBestHeight() int {
	var lastBlock Block

	bc.db.View(func(txn *badger.Txn) error {
		//b := tx.Bucket([]byte(bucket))
		p := []byte(prefix)
		item, _ := txn.Get([]byte(prefix + "l"))

		var lastHash []byte
		_ = item.Value(func(val []byte) error {
			lastHash = val
			return nil
		})

		item, _ = txn.Get(append(p, lastHash...))

		var blockData []byte
		_ = item.Value(func(val []byte) error {
			blockData = val
			return nil
		})

		lastBlock = *DeserializeBlock(blockData)

		return nil
	})

	return lastBlock.Height
}

func (bc *Blockchain) GetBlock(blockHash []byte) (Block, error) {
	var block Block

	bc.db.View(func(txn *badger.Txn) error {
		//b := tx.Bucket([]byte(bucket))
		p := []byte(prefix)
		item, _ := txn.Get(append(p, blockHash...))

		var blockData []byte
		_ = item.Value(func(val []byte) error {
			blockData = val
			return nil
		})

		if blockData == nil {
			return errors.New("Block is not found.")
		}

		block = *DeserializeBlock(blockData)

		return nil
	})

	return block, nil
}

func (bc *Blockchain) GetBlockHashes() [][]byte {
	var blocks [][]byte
	bci := bc.Iterator()

	for {
		block := bci.Next()

		blocks = append(blocks, block.Hash)

		if len(block.PrevBlockHash) == 0 {
			break
		}
	}

	return blocks
}

func (bc *Blockchain) MineBlock(transactions []*Transaction) *Block {
	var lastHash []byte
	var lastHeight int

	for _, tx := range transactions {
		if bc.VerifyTransaction(tx) != true {
			log.Panic("ERROR: Invalid transaction")
		}
	}

	bc.db.View(func(txn *badger.Txn) error {
		//b := tx.Bucket([]byte(bucket))
		p := []byte(prefix)
		item, _ := txn.Get([]byte(prefix + "l"))

		_ = item.Value(func(val []byte) error {
			lastHash = val
			return nil
		})

		item, _ = txn.Get(append(p, lastHash...))

		var blockData []byte
		_ = item.Value(func(val []byte) error {
			blockData = val
			return nil
		})

		block := DeserializeBlock(blockData)

		lastHeight = block.Height

		return nil
	})

	newBlock := NewBlock(transactions, lastHash, lastHeight)

	bc.db.Update(func(txn *badger.Txn) error {
		//b := tx.Bucket([]byte(bucket))
		p := []byte(prefix)
		txn.Set(append(p, newBlock.Hash...), newBlock.Serialize())
		txn.Set([]byte(prefix+"l"), newBlock.Hash)
		bc.tip = newBlock.Hash

		return nil
	})

	return newBlock
}

func (bc *Blockchain) Iterator() *BlockchainIterator {
	bci := &BlockchainIterator{bc.tip, bc.db}

	return bci
}

func (bc *Blockchain) FindUTXO() map[string]TXOutputs {
	UTXO := make(map[string]TXOutputs)
	spentTXOs := make(map[string][]int)
	bci := bc.Iterator()

	for {
		block := bci.Next()

		for _, tx := range block.Transactions {
			txID := hex.EncodeToString(tx.ID)

		Outputs:
			for outIdx, out := range tx.Vout {
				// Was the output spent?
				if spentTXOs[txID] != nil {
					for _, spentOutIdx := range spentTXOs[txID] {
						if spentOutIdx == outIdx {
							continue Outputs
						}
					}
				}

				outs := UTXO[txID]
				outs.Outputs = append(outs.Outputs, out)
				UTXO[txID] = outs
			}

			if tx.IsCoinbase() == false {
				for _, in := range tx.Vin {
					inTxID := hex.EncodeToString(in.Txid)
					spentTXOs[inTxID] = append(spentTXOs[inTxID], in.Vout)
				}
			}
		}

		if len(block.PrevBlockHash) == 0 {
			break
		}
	}

	return UTXO
}

func (bc *Blockchain) FindTransaction(ID []byte) (Transaction, error) {
	bci := bc.Iterator()

	for {
		block := bci.Next()

		for _, tx := range block.Transactions {
			if bytes.Compare(tx.ID, ID) == 0 {
				return *tx, nil
			}
		}

		if len(block.PrevBlockHash) == 0 {
			break
		}
	}

	return Transaction{}, errors.New("Transaction is not found")
}

func (bc *Blockchain) SignTransaction(tx *Transaction, privKey ecdsa.PrivateKey) {
	prevTXs := make(map[string]Transaction)

	for _, vin := range tx.Vin {
		prevTX, _ := bc.FindTransaction(vin.Txid)
		prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX
	}

	tx.Sign(privKey, prevTXs)
}

func (bc *Blockchain) VerifyTransaction(tx *Transaction) bool {
	if tx.IsCoinbase() {
		return true
	}

	prevTXs := make(map[string]Transaction)

	for _, vin := range tx.Vin {
		prevTX, err := bc.FindTransaction(vin.Txid)
		if err != nil {
			log.Panic(err)
		}
		prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX
	}

	return tx.Verify(prevTXs)
}

func dbExists(dbFile string) bool {
	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		return false
	}

	return true
}

type BlockchainIterator struct {
	currentHash []byte
	db          *badger.DB
}

func (i *BlockchainIterator) Next() *Block {
	var block *Block

	i.db.View(func(txn *badger.Txn) error {
		//b := tx.Bucket([]byte(bucket))
		p := []byte(prefix)
		item, _ := txn.Get(append(p, i.currentHash...))

		var encodedBlock []byte
		_ = item.Value(func(val []byte) error {
			encodedBlock = val
			return nil
		})
		block = DeserializeBlock(encodedBlock)

		return nil
	})

	i.currentHash = block.PrevBlockHash

	return block
}
