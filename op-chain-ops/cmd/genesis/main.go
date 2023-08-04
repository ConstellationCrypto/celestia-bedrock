package main

import (
	"os"

	"github.com/urfave/cli/v2"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-chain-ops/genesis"
	"github.com/ethereum-optimism/optimism/op-node/flags"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
)

func main() {
	// Set up logger with a default INFO level in case we fail to parse flags,
	// otherwise the final critical log won't show what the parsing error was.
	oplog.SetupDefaults()

	app := cli.NewApp()
	app.Flags = flags.Flags
	app.Name = "op-node"
	app.Usage = "Generate rollup genesis file"
	app.Description = "Subcommand of op-node to generate the genesis file"
	app.Commands = []*cli.Command{
		{
			Name:        "genesis",
			Subcommands: genesis.Subcommands,
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Crit("Application failed", "message", err)
	}
}
