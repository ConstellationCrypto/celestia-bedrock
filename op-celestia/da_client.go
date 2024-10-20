package celestia

import (
	"context"
	"encoding/hex"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/celestiaorg/go-square/blob"
	"github.com/celestiaorg/go-square/inclusion"
	"github.com/celestiaorg/go-square/namespace"
	"github.com/rollkit/go-da"
	"github.com/rollkit/go-da/proxy"
	"github.com/tendermint/tendermint/crypto/merkle"
)

type DAClient struct {
	Client     da.DA
	Namespace  da.Namespace
	S3Client   *s3.Client
	S3Bucket   string
	GetTimeout time.Duration
}

func NewDAClient(cfg CLIConfig, auth bool) (*DAClient, error) {
	nsBytes, err := hex.DecodeString(cfg.Namespace)
	if err != nil {
		return nil, err
	}
	if len(nsBytes) != 10 {
		return nil, errors.New("celestia: wrong namespace length")
	}
	var client da.DA
	if cfg.DaRpc != "" {
		client, err = proxy.NewClient(cfg.DaRpc, cfg.AuthToken)
		if err != nil {
			return nil, err
		}
	}
	var s3Client *s3.Client
	if auth {
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
		Client:     client,
		Namespace:  append(make([]byte, 19), nsBytes...),
		S3Client:   s3Client,
		S3Bucket:   cfg.S3Bucket,
		GetTimeout: cfg.Timeout,
	}, nil
}

func CreateCommitment(data da.Blob, ns da.Namespace) ([]byte, error) {
	ins, err := namespace.From(ns)
	if err != nil {
		return nil, err
	}
	return inclusion.CreateCommitment(blob.New(ins, data, 0), merkle.HashFromByteSlices, 64)
}
