package main

import (
	"encoding/hex"
	"github.com/dgraph-io/badger/v3"
	"log"
)

const utxoPrefix = "chainstate"

type UTXOSet struct {
	Blockchain *Blockchain
}

func (u UTXOSet) FindSpendableOutputs(pubkeyHash []byte, amount int) (int, map[string][]int) {
	unspentOutputs := make(map[string][]int)
	accumulated := 0
	db := u.Blockchain.db

	db.View(func(txn *badger.Txn) error {
		//b := tx.Bucket([]byte(utxoBucket))
		p := []byte(utxoPrefix)
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()
		//c := b.Cursor()

		//for k, v := c.First(); k != nil; k, v = c.Next() {
		//	txID := hex.EncodeToString(k)
		//	outs := DeserializeOutputs(v)
		//
		//	for outIdx, out := range outs.Outputs {
		//		if out.IsLockedWithKey(pubkeyHash) && accumulated < amount {
		//			accumulated += out.Value
		//			unspentOutputs[txID] = append(unspentOutputs[txID], outIdx)
		//		}
		//	}
		//}

		for it.Seek(p); it.ValidForPrefix(p); it.Next() {
			item := it.Item()
			k := item.Key()
			_ = item.Value(func(v []byte) error {
				txID := hex.EncodeToString(k)
				outs := DeserializeOutputs(v)

				for outIdx, out := range outs.Outputs {
					if out.IsLockedWithKey(pubkeyHash) && accumulated < amount {
						accumulated += out.Value
						unspentOutputs[txID] = append(unspentOutputs[txID], outIdx)
					}
				}
				return nil
			})
		}

		return nil
	})

	return accumulated, unspentOutputs
}

func (u UTXOSet) FindUTXO(pubKeyHash []byte) []TXOutput {
	var UTXOs []TXOutput
	db := u.Blockchain.db

	db.View(func(txn *badger.Txn) error {
		//b := tx.Bucket([]byte(utxoBucket))
		p := []byte(utxoPrefix)
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()
		//c := b.Cursor()

		//for k, v := c.First(); k != nil; k, v = c.Next() {
		//	outs := DeserializeOutputs(v)
		//
		//	for _, out := range outs.Outputs {
		//		if out.IsLockedWithKey(pubKeyHash) {
		//			UTXOs = append(UTXOs, out)
		//		}
		//	}
		//}

		for it.Seek(p); it.ValidForPrefix(p); it.Next() {
			item := it.Item()
			//k := item.Key()
			_ = item.Value(func(v []byte) error {
				outs := DeserializeOutputs(v)

				for _, out := range outs.Outputs {
					if out.IsLockedWithKey(pubKeyHash) {
						UTXOs = append(UTXOs, out)
					}
				}
				return nil
			})
		}

		return nil
	})

	return UTXOs
}

func (u UTXOSet) CountTransactions() int {
	db := u.Blockchain.db
	counter := 0

	err := db.View(func(txn *badger.Txn) error {
		//b := tx.Bucket([]byte(utxoBucket))
		p := []byte(utxoPrefix)
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()
		//c := b.Cursor()

		//for k, _ := c.First(); k != nil; k, _ = c.Next() {
		//	counter++
		//}

		for it.Seek(p); it.ValidForPrefix(p); it.Next() {
			counter++
		}

		return nil
	})
	if err != nil {
		log.Panic(err)
	}

	return counter
}

func (u UTXOSet) Reindex() {
	db := u.Blockchain.db
	//bucketName := []byte(utxoBucket)
	p := []byte(utxoPrefix)

	db.Update(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(p); it.ValidForPrefix(p); it.Next() {
			item := it.Item()
			k := item.Key()
			txn.Delete(k)
		}

		return nil
	})

	UTXO := u.Blockchain.FindUTXO()

	db.Update(func(txn *badger.Txn) error {
		//b := tx.Bucket(bucketName)

		for txID, outs := range UTXO {
			key, _ := hex.DecodeString(txID)
			txn.Set(append(p, key...), outs.Serialize())
		}

		return nil
	})
}

func (u UTXOSet) Update(block *Block) {
	db := u.Blockchain.db

	db.Update(func(txn *badger.Txn) error {
		//b := tx.Bucket([]byte(utxoBucket))
		p := []byte(utxoPrefix)

		for _, tx := range block.Transactions {
			if tx.IsCoinbase() == false {
				for _, vin := range tx.Vin {
					updatedOuts := TXOutputs{}
					item, _ := txn.Get(vin.Txid)

					var outsBytes []byte
					_ = item.Value(func(val []byte) error {
						outsBytes = val
						return nil
					})
					outs := DeserializeOutputs(outsBytes)

					for outIdx, out := range outs.Outputs {
						if outIdx != vin.Vout {
							updatedOuts.Outputs = append(updatedOuts.Outputs, out)
						}
					}

					if len(updatedOuts.Outputs) == 0 {
						txn.Delete(append(p, vin.Txid...))
					} else {
						txn.Set(append(p, vin.Txid...), updatedOuts.Serialize())
					}

				}
			}

			newOutputs := TXOutputs{}
			for _, out := range tx.Vout {
				newOutputs.Outputs = append(newOutputs.Outputs, out)
			}

			txn.Set(append(p, tx.ID...), newOutputs.Serialize())
		}

		return nil
	})
}
