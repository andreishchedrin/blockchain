package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
)

type CLI struct {
	bc *Blockchain
}

const addBlock = "add"
const printChain = "print"

func (cli *CLI) Run() {
	addBlockCmd := flag.NewFlagSet(addBlock, flag.ExitOnError)
	printChainCmd := flag.NewFlagSet(printChain, flag.ExitOnError)
	addBlockData := addBlockCmd.String("data", "", "Block data")

	switch os.Args[1] {
	case addBlock:
		addBlockCmd.Parse(os.Args[2:])
	case printChain:
		printChainCmd.Parse(os.Args[2:])
	default:
		os.Exit(1)
	}

	if addBlockCmd.Parsed() {
		if *addBlockData == "" {
			addBlockCmd.Usage()
			os.Exit(1)
		}
		cli.addBlock(*addBlockData)
	}

	if printChainCmd.Parsed() {
		cli.printChain()
	}
}

func (cli *CLI) addBlock(data string) {
	cli.bc.AddBlock(data)
	fmt.Println("Success!")
}

func (cli *CLI) printChain() {
	bci := cli.bc.Iterator()

	for {
		block := bci.Next()

		fmt.Printf("Prev. hash: %x\n", block.PrevBlockHash)
		fmt.Printf("Data: %s\n", block.Data)
		fmt.Printf("Hash: %x\n", block.Hash)
		pow := NewProofOfWork(block)
		fmt.Printf("PoW: %s\n", strconv.FormatBool(pow.Validate()))
		fmt.Println()

		if len(block.PrevBlockHash) == 0 {
			break
		}
	}
}