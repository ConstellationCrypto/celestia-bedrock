package derive

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"

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
	if s.comm == nil {
		// The L1 source provides the input commitment corresponding to the batch.
		data, err := s.src.Next(ctx)
		if err != nil {
			return nil, err
		}

		if len(data) == 0 {
			return nil, NotEnoughData
		}
		// If the transaction data type isn't Celestia,
		// pass it downstream for further validation
		// and potential parsing as L1 DA inputs.
		if data[0] != celestia.DerivationVersionCelestia {
			return data, nil
		}

		s.comm = data[1:]
	}

	s.log.Info("celestia data source: s.comm", "comm", fmt.Sprintf("%x", s.comm))
	height, commitment := celestia.SplitID(s.comm)

	// hotfix (10-17-2025): Some previous batches were submitted omitting the height on celestia.
	// to resolve, we need to lookup the full id on s3, and download the data there to get the proper id.
	// this works because the proper id was incorrectly uploaded to s3 for these batches.
	// we can detect this issue by checking if the id is 33 bytes wide instead of the correct 41 bytes.
	// as an additional validation, we should ensure that the data retrieved from s3 is 41 bytes long.
	// if this is the case, we can use the default splitID function to get the correct height and commitment,
	// and then fetch the data from _celestia_ using that id information.

	if s.comm == 32 {
		s.log.Info("Found Celestia reference with missing height; attempting to download correct reference from s3", "id", hex.EncodeToString(s.comm))
		ctx2, cancel := context.WithTimeout(context.Background(), d.CelestiaClient.GetTimeout)
		defer cancel()
		blob, err := celestia.DownloadS3Data(ctx2, d.CelestiaClient, append([]byte{celestia.DerivationVersionCelestia}, s.comm...))
		if err != nil {
			return fmt.Errorf("failed to download data from S3: %w", err)
		}
		if len(blob) == 41 {
			id = blob[1:]
		} else {
			return fmt.Errorf("invalid data length from s3 backup: %d", len(blob))
		}
		height, commitment = celestia.SplitID(id)
		s.log.Info("Found updated Celestia reference from S3", "height", height, "commitment", base64.StdEncoding.EncodeToString(commitment))
	}

	namespace, err := libshare.NewNamespaceFromBytes(daClient.Namespace)
	if err != nil {
		return nil, err
	}
	s.log.Info("celestia: fetching blob", "height", height, "commitment", hex.EncodeToString(commitment))
	blob, err := daClient.Client.Get(ctx, height, namespace, commitment)
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
