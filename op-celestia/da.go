package celestia

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ethereum/go-ethereum/log"
)

// DerivationVersionCelestia is a byte marker for celestia references submitted
// to the batch inbox address as calldata.
// Mnemonic 0xce = celestia
// version 0xce references are encoded as:
// [8]byte block height ++ [32]byte commitment
// in little-endian encoding.
// see: https://github.com/rollkit/celestia-da/blob/1f2df375fd2fcc59e425a50f7eb950daa5382ef0/celestia.go#L141-L160
const DerivationVersionCelestia = 0xce

// DerivationVersionCelestiaV2 is an alternative byte marker for celestia references
// (format byte 2) used by some batchers. Same encoding as DerivationVersionCelestia.
const DerivationVersionCelestiaV2 = 2

// 00000000000000000000000000000000000000ca1de12a6d29fe535f2d
// namespace input ^^ and have to strip down to 10
func DownloadS3Data(ctx context.Context, daClient *DAClient, frameRefData []byte) ([]byte, error) {
	if len(daClient.Namespace) != 29 {
		return nil, fmt.Errorf("Error: Expected 29 bytes, got %x", len(daClient.Namespace))
	}

	resp, err := daClient.S3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &daClient.S3Bucket,
		Key:    aws.String(fmt.Sprintf("%x/%x", daClient.Namespace, frameRefData)),
	})
	if err != nil {
		log.Error("celestia: failed to download data from S3 cache", "error", err, "path", fmt.Sprintf("%x/%x/%x", daClient.S3Bucket, daClient.Namespace, frameRefData))
		return nil, err
	}
	log.Warn("celestia: downloaded data from S3 cache")
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
