package kmssigner

import (
	"errors"

	opservice "github.com/ethereum-optimism/optimism/op-service"
	"github.com/urfave/cli/v2"
)

const (
	IdFlagName     = "kms.id"
	RegionFlagName = "kms.region"
)

func NewCLIConfig() CLIConfig {
	return CLIConfig{}
}

func CLIFlags(envPrefix string) []cli.Flag {
	envPrefix += "_KMS"
	flags := []cli.Flag{
		&cli.StringFlag{
			Name:    IdFlagName,
			Usage:   "KMS ID the client will reference",
			EnvVars: opservice.PrefixEnvVar(envPrefix, "ID"),
		},
		&cli.StringFlag{
			Name:    RegionFlagName,
			Usage:   "AWS region the client will connect to",
			EnvVars: opservice.PrefixEnvVar(envPrefix, "REGION"),
		},
	}
	return flags
}

type CLIConfig struct {
	Id     string
	Region string
}

func (c CLIConfig) Check() error {
	if !((c.Id == "" && c.Region == "") || (c.Id != "" && c.Region != "")) {
		return errors.New("signer endpoint and address must both be set or not set")
	}
	return nil
}

func (c CLIConfig) Enabled() bool {
	if c.Id != "" && c.Region != "" {
		return true
	}
	return false
}

func ReadCLIConfig(ctx *cli.Context) CLIConfig {
	cfg := CLIConfig{
		Id:     ctx.String(IdFlagName),
		Region: ctx.String(RegionFlagName),
	}
	return cfg
}
