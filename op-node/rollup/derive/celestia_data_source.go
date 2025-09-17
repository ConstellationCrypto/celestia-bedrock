package derive

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	libshare "github.com/celestiaorg/go-square/v2/share"
	celestia "github.com/ethereum-optimism/optimism/op-celestia"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/log"
)

var daClient *celestia.DAClient
var celestiaLegacyMode = os.Getenv("CELESTIA_LEGACY_MODE") == "true"

func CelestiaDAEnabled() bool {
	return daClient != nil
}

func SetCelestiaDA(c *celestia.DAClient) error {
	daClient = c
	return nil
}

type CelestiaDataSource struct {
	log log.Logger
	src DataIter
	// keep track of a pending commitment so we can keep trying to fetch the input.
	comm eth.Data
}

func NewCelestiaDataSource(log log.Logger, src DataIter) *CelestiaDataSource {
	return &CelestiaDataSource{
		log: log,
		src: src,
	}
}

func (s *CelestiaDataSource) Next(ctx context.Context) (eth.Data, error) {
	data, err := s.src.Next(ctx)
	if s.comm == nil {
		// The L1 source provides the input commitment corresponding to the batch.
		if err != nil {
			return nil, err
		}

		if len(data) == 0 {
			return nil, NotEnoughData
		}
		//caldera
		version := data[0]
		if celestiaLegacyMode {
			if data[0] == 1 && len(data) > 1 { // legacy eth data
				data = data[1:]
				version = data[0]
			}
			if data[0] == 2 { // legacy celestia data
				version = celestia.DerivationVersionCelestia
			}
		}
		// If the transaction data type isn't Celestia,
		// pass it downstream for further validation
		// and potential parsing as L1 DA inputs.
		// if data[0] != celestia.DerivationVersionCelestia {
		// 	return data, nil
		// }

		switch version {
		case celestia.DerivationVersionCelestia:
			s.comm = data[1:]
		default:
			return data, nil
		}
	}

	log.Info("celestia: blob request", "id", hex.EncodeToString(s.comm))
	ctx2, cancel := context.WithTimeout(context.Background(), daClient.GetTimeout)
	awsBlob, err := downloadS3Data(ctx2, data)
	cancel()
	if err != nil {
		log.Error("aws request failed", "err", err)
		height, commitment := celestia.SplitID(s.comm)
		namespace, err := libshare.NewNamespaceFromBytes(daClient.Namespace)
		if err != nil {
			return nil, err
		}

		blob, err := daClient.Client.Blob.Get(ctx, height, namespace, commitment)
		if err != nil {
			// return temporary error so we can keep retrying.
			return nil, NewTemporaryError(fmt.Errorf("celestia: failed to resolve frame: %w", err))
		}
		if blob == nil {
			s.log.Warn("celestia: skipping empty blobs")
			s.comm = nil
			// skip the input
			return s.Next(ctx)
		}

		// reset the commitment so we can fetch the next one from the source at the next iteration.
		s.comm = nil
		return blob.Data(), nil
	}
	log.Info("celestia: creating commitment from s3 retrieved data")
	commit, err := celestia.CreateCommitment(awsBlob, daClient.Namespace)
	cancel()
	if err != nil || !bytes.Equal(commit, data[9:]) {
		return nil, NewTemporaryError(fmt.Errorf("celestia: invalid commitment: calldata=%x commit=%x err=%w", data, commit, err))
	}
	return awsBlob, nil
}

// 00000000000000000000000000000000000000ca1de12a6d29fe535f2d
// namespace input ^^ and have to strip down to 10
func downloadS3Data(ctx context.Context, frameRefData []byte) ([]byte, error) {
	if len(daClient.Namespace) != 29 {
		return nil, fmt.Errorf("Error: Expected 29 bytes, got %x", len(daClient.Namespace))
	}

	resp, err := daClient.S3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &daClient.S3Bucket,
		Key:    aws.String(fmt.Sprintf("%x/%x", daClient.Namespace, frameRefData)),
	})
	if err != nil {
		return nil, err
	}
	log.Warn("celestia: downloaded data from S3 cache")
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
