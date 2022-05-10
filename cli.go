package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
)

const getBalance = "balance"
const createBlockchain = "createbc"
const createWallet = "createw"
const listAddresses = "list"
const reindexUTXO = "reindex"
const send = "send"
const printChain = "print"

type CLI struct{}

func (cli *CLI) Run() {
	cli.validateArgs()

	getBalanceCmd := flag.NewFlagSet(getBalance, flag.ExitOnError)
	createBlockchainCmd := flag.NewFlagSet(createBlockchain, flag.ExitOnError)
	createWalletCmd := flag.NewFlagSet(createWallet, flag.ExitOnError)
	listAddressesCmd := flag.NewFlagSet(listAddresses, flag.ExitOnError)
	reindexUTXOCmd := flag.NewFlagSet(reindexUTXO, flag.ExitOnError)
	sendCmd := flag.NewFlagSet(send, flag.ExitOnError)
	printChainCmd := flag.NewFlagSet(printChain, flag.ExitOnError)

	getBalanceAddress := getBalanceCmd.String("address", "", "The address to get balance for")
	createBlockchainAddress := createBlockchainCmd.String("address", "", "The address to send genesis block reward to")
	sendFrom := sendCmd.String("from", "", "Source wallet address")
	sendTo := sendCmd.String("to", "", "Destination wallet address")
	sendAmount := sendCmd.Int("amount", 0, "Amount to send")

	switch os.Args[1] {
	case getBalance:
		getBalanceCmd.Parse(os.Args[2:])
	case createBlockchain:
		createBlockchainCmd.Parse(os.Args[2:])
	case createWallet:
		createWalletCmd.Parse(os.Args[2:])
	case listAddresses:
		listAddressesCmd.Parse(os.Args[2:])
	case reindexUTXO:
		reindexUTXOCmd.Parse(os.Args[2:])
	case send:
		sendCmd.Parse(os.Args[2:])
	case printChain:
		printChainCmd.Parse(os.Args[2:])
	default:
		os.Exit(1)
	}

	if getBalanceCmd.Parsed() {
		if *getBalanceAddress == "" {
			getBalanceCmd.Usage()
			os.Exit(1)
		}

		cli.getBalance(*getBalanceAddress)
	}

	if createBlockchainCmd.Parsed() {
		if *createBlockchainAddress == "" {
			createBlockchainCmd.Usage()
			os.Exit(1)
		}
		cli.createBlockchain(*createBlockchainAddress)
	}

	if createWalletCmd.Parsed() {
		cli.createWallet()
	}

	if listAddressesCmd.Parsed() {
		cli.listAddresses()
	}

	if reindexUTXOCmd.Parsed() {
		cli.reindexUTXO()
	}

	if sendCmd.Parsed() {
		if *sendFrom == "" || *sendTo == "" || *sendAmount <= 0 {
			sendCmd.Usage()
			os.Exit(1)
		}

		cli.send(*sendFrom, *sendTo, *sendAmount)
	}

	if printChainCmd.Parsed() {
		cli.printChain()
	}
}

func (cli *CLI) createBlockchain(address string) {
	if !ValidateAddress(address) {
		log.Panic("ERROR: Address is not valid")
	}
	bc := CreateBlockchain(address)
	defer bc.db.Close()

	UTXOSet := UTXOSet{bc}
	UTXOSet.Reindex()

	fmt.Println("Done!")
}

func (cli *CLI) createWallet() {
	wallets, _ := NewWallets()
	address := wallets.CreateWallet()
	wallets.SaveToFile()

	fmt.Printf("Your new address: %s\n", address)
}

func (cli *CLI) listAddresses() {
	wallets, _ := NewWallets()
	addresses := wallets.GetAddresses()

	for _, address := range addresses {
		fmt.Println(address)
	}
}

func (cli *CLI) getBalance(address string) {
	if !ValidateAddress(address) {
		log.Panic("ERROR: Address is not valid")
	}
	bc := NewBlockchain()
	defer bc.db.Close()

	UTXOSet := UTXOSet{bc}

	balance := 0
	pubKeyHash := Base58Decode([]byte(address))
	pubKeyHash = pubKeyHash[1 : len(pubKeyHash)-4]
	UTXOs := UTXOSet.FindUTXO(pubKeyHash)

	for _, out := range UTXOs {
		balance += out.Value
	}

	fmt.Printf("Balance of '%s': %d\n", address, balance)
}

func (cli *CLI) printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  createbc -address ADDRESS - Create a blockchain and send genesis block reward to ADDRESS")
	fmt.Println("  createw - Generates a new key-pair and saves it into the wallet file")
	fmt.Println("  balance -address ADDRESS - Get balance of ADDRESS")
	fmt.Println("  list - Lists all addresses from the wallet file")
	fmt.Println("  print - Print all the blocks of the blockchain")
	fmt.Println("  reindex - Rebuilds the UTXO set")
	fmt.Println("  send -from FROM -to TO -amount AMOUNT - Send AMOUNT of coins from FROM address to TO")
}

func (cli *CLI) printChain() {
	bc := NewBlockchain()
	defer bc.db.Close()

	bci := bc.Iterator()

	for {
		block := bci.Next()

		fmt.Printf("============ Block %x ============\n", block.Hash)
		fmt.Printf("Prev. block: %x\n", block.PrevBlockHash)
		pow := NewProofOfWork(block)
		fmt.Printf("PoW: %s\n\n", strconv.FormatBool(pow.Validate()))
		for _, tx := range block.Transactions {
			fmt.Println(tx)
		}
		fmt.Printf("\n\n")

		if len(block.PrevBlockHash) == 0 {
			break
		}
	}
}

func (cli *CLI) validateArgs() {
	if len(os.Args) < 2 {
		cli.printUsage()
		os.Exit(1)
	}
}

func (cli *CLI) reindexUTXO() {
	bc := NewBlockchain()
	UTXOSet := UTXOSet{bc}
	UTXOSet.Reindex()

	count := UTXOSet.CountTransactions()
	fmt.Printf("Done! There are %d transactions in the UTXO set.\n", count)
}

func (cli *CLI) send(from, to string, amount int) {
	if !ValidateAddress(from) {
		log.Panic("ERROR: Sender address is not valid")
	}
	if !ValidateAddress(to) {
		log.Panic("ERROR: Recipient address is not valid")
	}

	bc := NewBlockchain()
	defer bc.db.Close()

	UTXOSet := UTXOSet{bc}

	tx := NewUTXOTransaction(from, to, amount, &UTXOSet)
	cbTx := NewCoinbaseTX(from, "")
	txs := []*Transaction{cbTx, tx}

	newBlock := bc.MineBlock(txs)
	UTXOSet.Update(newBlock)

	fmt.Println("Success!")
}
