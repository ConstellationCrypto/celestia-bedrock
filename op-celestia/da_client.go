package celestia

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/celestiaorg/go-square/blob"
	"github.com/celestiaorg/go-square/inclusion"
	"github.com/celestiaorg/go-square/namespace"
	"github.com/rollkit/go-da"
	"github.com/tendermint/tendermint/crypto/merkle"

	"github.com/celestiaorg/celestia-node/api/rpc/client"
)

type DAClient struct {
	Client       *client.Client
	GetTimeout   time.Duration
	Namespace    da.Namespace
	FallbackMode string
	GasPrice     float64
	S3Client     *s3.Client
	S3Bucket     string
}

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

func NewDAClient(rpc, token, namespace, fallbackMode string, gasPrice float64, s3region string, s3bucket string, auth bool) (*DAClient, error) {
	client, err := client.NewClient(context.Background(), rpc, token)
	if err != nil {
		return nil, err
	}
	ns, err := hex.DecodeString(namespace)
	if err != nil {
		return nil, err
	}
	if fallbackMode != "disabled" && fallbackMode != "blobdata" && fallbackMode != "calldata" {
		return nil, fmt.Errorf("celestia: unknown fallback mode: %s", fallbackMode)
	}
	var s3Client *s3.Client
	if auth {
		awscfg, err := config.LoadDefaultConfig(context.Background(),
			config.WithRegion(s3region),
		)
		if err != nil {
			return nil, err
		}
		s3Client = s3.NewFromConfig(awscfg)
	} else {
		s3Client = s3.New(s3.Options{Region: s3region})
	}
	return &DAClient{
		Client:       client,
		GetTimeout:   time.Minute,
		Namespace:    ns,
		FallbackMode: fallbackMode,
		GasPrice:     gasPrice,
		S3Client:     s3Client,
		S3Bucket:     s3bucket,
	}, nil
}

func CreateCommitment(data da.Blob, ns da.Namespace) ([]byte, error) {
	ins, err := namespace.From(ns)
	if err != nil {
		return nil, err
	}
	return inclusion.CreateCommitment(blob.New(ins, data, 0), merkle.HashFromByteSlices, 64)
}
