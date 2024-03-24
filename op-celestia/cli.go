package celestia

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/urfave/cli/v2"

	opservice "github.com/ethereum-optimism/optimism/op-service"
)

var (
	ErrInvalidPort = errors.New("invalid port")
)

func Check(address string) error {
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}

	if port == "" {
		return ErrInvalidPort
	}

	_, err = net.LookupPort("tcp", port)
	if err != nil {
		return err
	}

	return nil
}

func CLIFlags(envPrefix string) []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "da-rpc",
			Usage:   "dial address of data availability grpc client",
			EnvVars: opservice.PrefixEnvVar(envPrefix, "DA_RPC"),
		},
		&cli.StringFlag{
			Name:    "namespace-id",
			Usage:   "Namespace ID for DA node",
			Value:   "00000000000000000000",
			EnvVars: opservice.PrefixEnvVar(envPrefix, "NAMESPACE_ID"),
		},
		&cli.StringFlag{
			Name:    "auth-token",
			Usage:   "Authentication Token for DA node",
			EnvVars: opservice.PrefixEnvVar(envPrefix, "AUTH_TOKEN"),
		},
		&cli.StringFlag{
			Name:    "s3-bucket",
			Usage:   "S3 Bucket for DA layer",
			EnvVars: opservice.PrefixEnvVar(envPrefix, "S3_BUCKET"),
		},
		&cli.StringFlag{
			Name:    "s3-region",
			Usage:   "S3 Region for DA layer",
			Value:   "us-west-2",
			EnvVars: opservice.PrefixEnvVar(envPrefix, "S3_REGION"),
		},
		&cli.DurationFlag{
			Name:    "celestia-timeout",
			Usage:   "timeout for celestia requests",
			Value:   time.Minute,
			EnvVars: opservice.PrefixEnvVar(envPrefix, "CELESTIA_TIMEOUT"),
		},
	}
}

type CLIConfig struct {
	DaRpc     string
	Namespace string
	AuthToken string
	S3Bucket  string
	S3Region  string
	Timeout   time.Duration
}

func (c CLIConfig) Check() error {
	if c.DaRpc != "" {
		if err := Check(c.DaRpc); err != nil {
			return fmt.Errorf("invalid da rpc: %w", err)
		}
	}

	return nil
}

func ReadCLIConfig(ctx *cli.Context) CLIConfig {
	return CLIConfig{
		DaRpc:     ctx.String("da-rpc"),
		Namespace: ctx.String("namespace-id"),
		AuthToken: ctx.String("auth-token"),
		S3Bucket:  ctx.String("s3-bucket"),
		S3Region:  ctx.String("s3-region"),
		Timeout:   ctx.Duration("celestia-timeout"),
	}
}
