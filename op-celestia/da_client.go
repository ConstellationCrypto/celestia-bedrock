package celestia

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	txClient "github.com/celestiaorg/celestia-node/api/client"
	"github.com/celestiaorg/celestia-node/api/rpc/client"
	blobAPI "github.com/celestiaorg/celestia-node/nodebuilder/blob"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/ethereum/go-ethereum/log"
)

// heightLen is a length (in bytes) of serialized height.
//
// This is 8 as uint64 consist of 8 bytes.
const heightLen = 8

func MakeID(height uint64, commitment []byte) []byte {
	id := make([]byte, heightLen+len(commitment))
	binary.LittleEndian.PutUint64(id, height)
	copy(id[heightLen:], commitment)
	return id
}

func SplitID(id []byte) (uint64, []byte) {
	if len(id) <= heightLen {
		return 0, nil
	}
	commitment := id[heightLen:]
	return binary.LittleEndian.Uint64(id[:heightLen]), commitment
}

type TxClientConfig struct {
	DefaultKeyName     string
	KeyringPath        string
	CoreGRPCAddr       string
	CoreGRPCTLSEnabled bool
	CoreGRPCAuthToken  string
	P2PNetwork         string
}

func (c *TxClientConfig) String() string {
	return fmt.Sprintf("TxClientConfig{DefaultKeyName: %s, KeyringPath: %s, CoreGRPCAddr: %s, CoreGRPCTLSEnabled: %t, CoreGRPCAuthToken: %s, P2PNetwork: %s}",
		c.DefaultKeyName, c.KeyringPath, c.CoreGRPCAddr, c.CoreGRPCTLSEnabled, c.CoreGRPCAuthToken, c.P2PNetwork)
}

type RPCClientConfig struct {
	URL            string
	TLSEnabled     bool
	AuthToken      string
	Namespace      []byte
	FallbackMode   string
	GasPrice       float64
	TxClientConfig *TxClientConfig
	Auth           bool
	S3Region       string
	S3Client       *s3.Client
	S3Bucket       string
}

func (c RPCClientConfig) String() string {
	return fmt.Sprintf("RPCClientConfig{URL: %s, TLSEnabled: %t, AuthToken: %s, Namespace: %s, FallbackMode: %s, GasPrice: %f, TxClientConfig: %v, Auth: %t, S3Region: %s, S3Bucket: %s}",
		c.URL, c.TLSEnabled, c.AuthToken, c.Namespace, c.FallbackMode, c.GasPrice, c.TxClientConfig, c.Auth, c.S3Region, c.S3Bucket)
}

type DAClient struct {
	Client        blobAPI.Module
	GetTimeout    time.Duration
	SubmitTimeout time.Duration
	Namespace     []byte
	FallbackMode  string
	GasPrice      float64
	S3Client      *s3.Client
	S3Bucket      string
}

// initTxClient initializes a transaction client for Celestia.
func initTxClient(cfg RPCClientConfig) (blobAPI.Module, error) {
	keyname := cfg.TxClientConfig.DefaultKeyName
	if keyname == "" {
		keyname = "my_celes_key"
	}
	kr, err := txClient.KeyringWithNewKey(txClient.KeyringConfig{
		KeyName:     keyname,
		BackendName: keyring.BackendTest,
	}, cfg.TxClientConfig.KeyringPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create keyring: %w", err)
	}

	// Configure client
	config := txClient.Config{
		ReadConfig: txClient.ReadConfig{
			BridgeDAAddr: cfg.URL,
			DAAuthToken:  cfg.AuthToken,
			EnableDATLS:  cfg.TLSEnabled,
		},
		SubmitConfig: txClient.SubmitConfig{
			DefaultKeyName: cfg.TxClientConfig.DefaultKeyName,
			Network:        p2p.Network(cfg.TxClientConfig.P2PNetwork),
			CoreGRPCConfig: txClient.CoreGRPCConfig{
				Addr:       cfg.TxClientConfig.CoreGRPCAddr,
				TLSEnabled: cfg.TxClientConfig.CoreGRPCTLSEnabled,
				AuthToken:  cfg.TxClientConfig.CoreGRPCAuthToken,
			},
		},
	}
	ctx := context.Background()
	celestiaClient, err := txClient.New(ctx, config, kr)
	if err != nil {
		return nil, fmt.Errorf("failed to create tx client: %w", err)
	}
	return celestiaClient.Blob, nil
}

// initRPCClient initializes an RPC client for Celestia.
func initRPCClient(cfg RPCClientConfig) (blobAPI.Module, error) {
	celestiaClient, err := client.NewClient(context.Background(), cfg.URL, cfg.AuthToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create rpc client: %w", err)
	}
	return &celestiaClient.Blob, nil
}

func NewDAClient(cfg RPCClientConfig) (*DAClient, error) {
	var blobClient blobAPI.Module
	var err error
	if cfg.TxClientConfig != nil {
		blobClient, err = initTxClient(cfg)
	} else {
		blobClient, err = initRPCClient(cfg)
	}
	if err != nil {
		log.Crit("failed to initialize celestia client", "err", err)
	}
	_, err = libshare.NewNamespaceFromBytes(cfg.Namespace)
	if err != nil {
		log.Crit("failed to parse namespace", "err", err)
	}
	var s3Client *s3.Client
	if cfg.Auth {
		awscfg, err := config.LoadDefaultConfig(context.Background(),
			config.WithRegion(cfg.S3Region),
		)
		if err != nil {
			return nil, err
		}
		s3Client = s3.NewFromConfig(awscfg)
	} else {
		s3Client = s3.New(s3.Options{Region: cfg.S3Region})
	}
	return &DAClient{
		Client:        blobClient,
		GetTimeout:    time.Minute,
		SubmitTimeout: time.Minute,
		Namespace:     cfg.Namespace,
		FallbackMode:  cfg.FallbackMode,
		GasPrice:      cfg.GasPrice,
		S3Client:      s3Client,
		S3Bucket:      cfg.S3Bucket,
	}, nil
}
