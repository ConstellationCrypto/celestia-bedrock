package rollup

import (
	"context"
	"encoding/hex"
	"errors"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	openrpc "github.com/rollkit/celestia-openrpc"
	"github.com/rollkit/celestia-openrpc/types/share"
)

type DAConfig struct {
	Namespace share.Namespace
	Client    *openrpc.Client
	S3Client  *s3.Client
	S3Bucket  string
}

func NewDAConfig(rpc, token, ns, bucket, region string) (*DAConfig, error) {
	nsBytes, err := hex.DecodeString(ns)
	if err != nil {
		return nil, err
	}

	namespace, err := share.NewBlobNamespaceV0(nsBytes)
	if err != nil {
		return nil, err
	}

	var client *openrpc.Client
	if len(rpc) > 0 {
		client, err = openrpc.NewClient(context.Background(), rpc, token)
		if err != nil {
			return nil, err
		}
	}

	if len(bucket) == 0 {
		return nil, errors.New("s3 bucket is empty")
	}

	return &DAConfig{
		Namespace: namespace,
		Client:    client,
		S3Client:  s3.New(s3.Options{Region: region}),
		S3Bucket:  bucket,
	}, nil
}
