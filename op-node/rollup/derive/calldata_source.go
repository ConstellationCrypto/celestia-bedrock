package derive

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/rollkit/celestia-openrpc/types/blob"
	"github.com/rollkit/celestia-openrpc/types/share"

	"github.com/ethereum-optimism/optimism/op-celestia/celestia"
	"github.com/ethereum-optimism/optimism/op-node/eth"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
)

type DataIter interface {
	Next(ctx context.Context) (eth.Data, error)
}

type L1TransactionFetcher interface {
	InfoAndTxsByHash(ctx context.Context, hash common.Hash) (eth.BlockInfo, types.Transactions, error)
}

// DataSourceFactory readers raw transactions from a given block & then filters for
// batch submitter transactions.
// This is not a stage in the pipeline, but a wrapper for another stage in the pipeline
type DataSourceFactory struct {
	log     log.Logger
	cfg     *rollup.Config
	daCfg   *rollup.DAConfig
	fetcher L1TransactionFetcher
}

func NewDataSourceFactory(log log.Logger, cfg *rollup.Config, daCfg *rollup.DAConfig, fetcher L1TransactionFetcher) *DataSourceFactory {
	return &DataSourceFactory{log: log, cfg: cfg, daCfg: daCfg, fetcher: fetcher}
}

// OpenData returns a DataIter. This struct implements the `Next` function.
func (ds *DataSourceFactory) OpenData(ctx context.Context, id eth.BlockID, batcherAddr common.Address) (DataIter, error) {
	return NewDataSource(ctx, ds.log, ds.cfg, ds.daCfg, ds.fetcher, id, batcherAddr)
}

// DataSource is a fault tolerant approach to fetching data.
// The constructor will never fail & it will instead re-attempt the fetcher
// at a later point.
type DataSource struct {
	// Internal state + data
	open bool
	data []eth.Data
	// Required to re-attempt fetching
	id      eth.BlockID
	cfg     *rollup.Config // TODO: `DataFromEVMTransactions` should probably not take the full config
	daCfg   *rollup.DAConfig
	fetcher L1TransactionFetcher
	log     log.Logger

	batcherAddr common.Address
}

// NewDataSource creates a new calldata source. It suppresses errors in fetching the L1 block if they occur.
// If there is an error, it will attempt to fetch the result on the next call to `Next`.
func NewDataSource(ctx context.Context, log log.Logger, cfg *rollup.Config, daCfg *rollup.DAConfig, fetcher L1TransactionFetcher, block eth.BlockID, batcherAddr common.Address) (DataIter, error) {
	_, txs, err := fetcher.InfoAndTxsByHash(ctx, block.Hash)
	if err != nil {
		return &DataSource{
			open:        false,
			id:          block,
			cfg:         cfg,
			daCfg:       daCfg,
			fetcher:     fetcher,
			log:         log,
			batcherAddr: batcherAddr,
		}, nil
	} else {
		data, err := DataFromEVMTransactions(ctx, cfg, daCfg, batcherAddr, txs, log.New("origin", block))
		if err != nil {
			return &DataSource{
				open:        false,
				id:          block,
				cfg:         cfg,
				daCfg:       daCfg,
				fetcher:     fetcher,
				log:         log,
				batcherAddr: batcherAddr,
			}, err
		}
		return &DataSource{
			open: true,
			data: data,
		}, nil
	}
}

// Next returns the next piece of data if it has it. If the constructor failed, this
// will attempt to reinitialize itself. If it cannot find the block it returns a ResetError
// otherwise it returns a temporary error if fetching the block returns an error.
func (ds *DataSource) Next(ctx context.Context) (eth.Data, error) {
	if !ds.open {
		if _, txs, err := ds.fetcher.InfoAndTxsByHash(ctx, ds.id.Hash); err == nil {
			ds.open = true
			ds.data, err = DataFromEVMTransactions(ctx, ds.cfg, ds.daCfg, ds.batcherAddr, txs, log.New("origin", ds.id))
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

func downloadS3Data(ctx context.Context, daCfg *rollup.DAConfig, frameRefData []byte) ([]byte, error) {
	resp, err := daCfg.S3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(daCfg.S3Bucket),
		Key:    aws.String(fmt.Sprintf("%s/%x", daCfg.Namespace.String(), frameRefData)),
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}

// DataFromEVMTransactions filters all of the transactions and returns the calldata from transactions
// that are sent to the batch inbox address from the batch sender address.
// This will return an empty array if no valid transactions are found.
func DataFromEVMTransactions(ctx context.Context, config *rollup.Config, daCfg *rollup.DAConfig, batcherAddr common.Address, txs types.Transactions, log log.Logger) ([]eth.Data, error) {
	var out []eth.Data
	l1Signer := config.L1Signer()
	for j, tx := range txs {
		if to := tx.To(); to != nil && *to == config.BatchInboxAddress {
			seqDataSubmitter, err := l1Signer.Sender(tx) // optimization: only derive sender if To is correct
			if err != nil {
				log.Warn("tx in inbox with invalid signature", "index", j, "err", err)
				continue // bad signature, ignore
			}
			// some random L1 user might have sent a transaction to our batch inbox, ignore them
			if seqDataSubmitter != batcherAddr {
				log.Warn("tx in inbox with unauthorized submitter", "index", j, "err", err)
				continue // not an authorized batch submitter, ignore
			}
			if len(tx.Data()) == 0 {
				log.Error("empty tx in inbox", "index", j, "err", err)
				continue
			}

			switch tx.Data()[0] {

			// legacy hardfork code - remove case 0 for production
			case 0:
				if len(tx.Data()) != 12 {
					log.Error("celestia-legacy: invalid length", "len", len(tx.Data()))
					continue
				}
				buf := bytes.NewBuffer(tx.Data())
				var height uint64
				err := binary.Read(buf, binary.BigEndian, &height)
				if err != nil || height == 0 {
					log.Error("celestia-legacy: invalid height", "height", height)
					continue
				}
				var index uint32
				err = binary.Read(buf, binary.BigEndian, &index)
				if err != nil || index != 0 {
					log.Error("celestia-legacy: invalid index", "index", index)
					continue
				}
				log.Info("celestia-legacy: requesting block from s3")
				data, err := downloadS3Data(ctx, daCfg, tx.Data())
				if err != nil {
					log.Error("celestia-legacy: s3 request failed, requesting from celestia", "err", err)
					blobs, err := daCfg.Client.Blob.GetAll(ctx, height, []share.Namespace{daCfg.Namespace})
					if err != nil {
						log.Error("celestia-legacy: celestia request failed", err)
						return nil, NewTemporaryError(err)
					}
					data = blobs[0].Data
				}
				out = append(out, data)

			case 1:
				out = append(out, tx.Data()[1:])

			case celestia.CurrentVersion: // 2
				if daCfg == nil {
					log.Error("missing DA_RPC url")
					return nil, NewCriticalError(errors.New("missing DA_RPC url"))
				}

				frameRef := celestia.FrameRef{}
				frameRef.UnmarshalBinary(tx.Data())

				if err != nil {
					log.Error("unable to decode frame reference", "index", j, "err", err)
					return nil, NewCriticalError(err)
				}
				log.Info("requesting data from aws", "url", fmt.Sprintf("s3://%s/%s/%x", daCfg.S3Bucket, daCfg.Namespace.String(), tx.Data()))
				var txblob *blob.Blob
				data, err := downloadS3Data(ctx, daCfg, tx.Data())
				if err != nil {
					log.Error("aws request failed", "err", err)
					log.Info("requesting data from celestia", "namespace", hex.EncodeToString(daCfg.Namespace), "height", frameRef.BlockHeight, "commitment", hex.EncodeToString(frameRef.TxCommitment))
					txblob, err = daCfg.Client.Blob.Get(ctx, frameRef.BlockHeight, daCfg.Namespace, frameRef.TxCommitment)
					if err != nil {
						log.Error("celestia request failed", "err", err)
						return nil, NewTemporaryError(err)
					}
				} else {
					txblob, err = blob.NewBlobV0(daCfg.Namespace, data)
					if err != nil {
						log.Error("unable to create celestia blob", "err", err)
						return nil, NewTemporaryError(err)
					}
				}
				com, err := blob.CreateCommitment(txblob)
				if err != nil {
					log.Error("unable to create celestia commitment", "err", err)
					return nil, NewTemporaryError(err)
				}
				if !bytes.Equal(com, frameRef.TxCommitment) {
					log.Error("invalid celestia commitment")
					return nil, NewCriticalError(errors.New("invalid celestia commitment"))
				}
				out = append(out, txblob.Data)

			default:
				log.Error("invalid data type", "type", tx.Data()[0])
				continue
			}
		}
	}
	return out, nil
}
