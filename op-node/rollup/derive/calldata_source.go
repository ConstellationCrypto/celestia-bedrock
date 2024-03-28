package derive

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"

	celestia "github.com/ethereum-optimism/optimism/op-celestia"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

var celestiaLegacyMode = os.Getenv("CELESTIA_LEGACY_MODE") == "true"

var daClient *celestia.DAClient

func SetDAClient(c *celestia.DAClient) error {
	if daClient != nil {
		return errors.New("da client already configured")
	}
	daClient = c
	return nil
}

// CalldataSource is a fault tolerant approach to fetching data.
// The constructor will never fail & it will instead re-attempt the fetcher
// at a later point.
type CalldataSource struct {
	// Internal state + data
	open bool
	data []eth.Data
	// Required to re-attempt fetching
	ref     eth.L1BlockRef
	dsCfg   DataSourceConfig
	fetcher L1TransactionFetcher
	log     log.Logger

	batcherAddr common.Address
}

// NewCalldataSource creates a new calldata source. It suppresses errors in fetching the L1 block if they occur.
// If there is an error, it will attempt to fetch the result on the next call to `Next`.
func NewCalldataSource(ctx context.Context, log log.Logger, dsCfg DataSourceConfig, fetcher L1TransactionFetcher, ref eth.L1BlockRef, batcherAddr common.Address) (DataIter, error) {
	_, txs, err := fetcher.InfoAndTxsByHash(ctx, ref.Hash)
	if err != nil {
		return &CalldataSource{
			open:        false,
			ref:         ref,
			dsCfg:       dsCfg,
			fetcher:     fetcher,
			log:         log,
			batcherAddr: batcherAddr,
		}, nil
	}
	data, err := DataFromEVMTransactions(ctx, dsCfg, batcherAddr, txs, log.New("origin", ref))
	if err != nil {
		return &CalldataSource{
			open:        false,
			ref:         ref,
			dsCfg:       dsCfg,
			fetcher:     fetcher,
			log:         log,
			batcherAddr: batcherAddr,
		}, err
	}
	return &CalldataSource{
		open: true,
		data: data,
	}, nil
}

// Next returns the next piece of data if it has it. If the constructor failed, this
// will attempt to reinitialize itself. If it cannot find the block it returns a ResetError
// otherwise it returns a temporary error if fetching the block returns an error.
func (ds *CalldataSource) Next(ctx context.Context) (eth.Data, error) {
	if !ds.open {
		if _, txs, err := ds.fetcher.InfoAndTxsByHash(ctx, ds.ref.Hash); err == nil {
			ds.open = true
			ds.data, err = DataFromEVMTransactions(ctx, ds.dsCfg, ds.batcherAddr, txs, ds.log)
			if err != nil {
				// already wrapped
				return nil, err
			}
		} else if errors.Is(err, ethereum.NotFound) {
			return nil, NewResetError(fmt.Errorf("failed to open calldata source: %w", err))
		} else {
			return nil, NewTemporaryError(fmt.Errorf("failed to open calldata source: %w", err))
		}
	}
	if len(ds.data) == 0 {
		return nil, io.EOF
	} else {
		data := ds.data[0]
		ds.data = ds.data[1:]
		return data, nil
	}
}

func downloadS3Data(ctx context.Context, frameRefData []byte) ([]byte, error) {
	ctx2, cancel := context.WithTimeout(ctx, daClient.GetTimeout)
	resp, err := daClient.S3Client.GetObject(ctx2, &s3.GetObjectInput{
		Bucket: &daClient.S3Bucket,
		Key:    aws.String(fmt.Sprintf("%x/%x", daClient.Namespace, frameRefData)),
	})
	cancel()
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}

// DataFromEVMTransactions filters all of the transactions and returns the calldata from transactions
// that are sent to the batch inbox address from the batch sender address.
// This will return an empty array if no valid transactions are found.
func DataFromEVMTransactions(ctx context.Context, dsCfg DataSourceConfig, batcherAddr common.Address, txs types.Transactions, log log.Logger) ([]eth.Data, error) {
	out := []eth.Data{}
	for _, tx := range txs {
		if isValidBatchTx(tx, dsCfg.l1Signer, dsCfg.batchInboxAddress, batcherAddr) {
			data := tx.Data()
			switch len(data) {
			case 0:
				out = append(out, data)
			default:
				if celestiaLegacyMode {
					if data[0] == 1 { // legacy eth data
						data = data[1:]
					}
					if data[0] == 2 { // legacy celestia data
						data[0] = celestia.DerivationVersionCelestia
					}
				}
				switch data[0] {
				case celestia.DerivationVersionCelestia:
					if len(data) != 41 {
						return nil, NewCriticalError(fmt.Errorf("celestia: invalid calldata length: %d", len(data)))
					}
					log.Info("celestia: blob request", "id", hex.EncodeToString(data[1:]))
					blob, err := downloadS3Data(ctx, data)
					if err != nil {
						log.Error("aws request failed", "err", err)
						ctx2, cancel := context.WithTimeout(ctx, daClient.GetTimeout)
						blobs, err := daClient.Client.Get(ctx2, [][]byte{data[1:]}, daClient.Namespace)
						cancel()
						if err != nil || len(blobs) != 1 {
							return nil, NewTemporaryError(fmt.Errorf("celestia: failed to resolve frame: %w, len=", err, len(blobs)))
						}
						blob = blobs[0]
					}
					commit, err := celestia.CreateCommitment(blob, daClient.Namespace)
					if err != nil || !bytes.Equal(commit, data[9:]) {
						return nil, NewTemporaryError(fmt.Errorf("celestia: invalid commitment: calldata=%x commit=%x err=%w", data, commit, err))
					}
					out = append(out, blob)
				default: // case 0
					out = append(out, data)
				}
			}
		}
	}
	return out, nil
}
