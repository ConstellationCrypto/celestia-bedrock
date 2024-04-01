package kmssigner

import (
	"context"
	"log"
	"math/big"
	"testing"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// In order to run these tests, you must be authenticated on the sandbox
// AWS account (or change the keyId to a valid key)
// Address: 0x041449B070d13A2ef5B6483C8093F337ECD22F62
const keyId = "mrk-27a1189981c94f6b8771df0c57759a1e"

const anotherEthAddr = "0x0000000000000000000000000000000000000000"
const ethAddr = "https://eth-goerli.g.alchemy.com/v2/R-2IK-_U4uoJKpr3zO9VqUl3upMmZsPh"

func TestAddress(t *testing.T) {
	awsCfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	kmsSvc := kms.NewFromConfig(awsCfg)
	address, err := GetAddress(kmsSvc, keyId)
	if err != nil {
		log.Fatal(err)
	}
	log.Print(address)
}

func TestSigning(t *testing.T) {
	ctx := context.Background()
	awsCfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	kmsSvc := kms.NewFromConfig(awsCfg)

	client, err := ethclient.Dial(ethAddr)
	if err != nil {
		log.Fatal(err)
	}

	clChainId, _ := client.ChainID(ctx)

	transactOpts, err := NewAwsKmsTransactorWithChainIDCtx(ctx, kmsSvc, keyId, clChainId)
	if err != nil {
		log.Fatalf("can not sign: %s", err)
	}

	nonce, err := client.PendingNonceAt(context.Background(), transactOpts.From)
	if err != nil {
		log.Fatal(err)
	}

	toAddress := common.HexToAddress(anotherEthAddr)

	suggestedGasPrice, _ := client.SuggestGasPrice(ctx)
	suggestedGasLimit, err := client.EstimateGas(ctx, ethereum.CallMsg{To: &toAddress, Data: nil})
	if err != nil {
		log.Fatal(err)
	}
	value := big.NewInt(10)
	gasLimit := suggestedGasLimit
	gasPrice := suggestedGasPrice

	tx := types.NewTransaction(nonce, toAddress, value, gasLimit, gasPrice, nil)

	signedTx, err := transactOpts.Signer(transactOpts.From, tx)
	if err != nil {
		log.Fatal(err)
	}

	err = client.SendTransaction(ctx, signedTx)
	if err != nil {
		log.Fatalf("can not send tx %s", err)
	}
}
