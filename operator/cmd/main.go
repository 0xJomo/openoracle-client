package main

import (
	"avs-oracle/core/config"
	"avs-oracle/operator/cmd/actions"
	"github.com/urfave/cli"
	"log"
	"os"
)

func main() {
	app := cli.NewApp()
	app.Flags = []cli.Flag{config.ConfigFileFlag}
	app.Name = "openoracle-operator"
	app.Usage = "OpenOracle Operator"
	app.Description = "Service that transforms web2 data onchain."

	app.Commands = []cli.Command{
		{
			Name:    "register-operator-with-eigenlayer",
			Aliases: []string{"rel"},
			Usage:   "registers operator with eigenlayer",
			Action:  actions.RegisterOperatorWithEigenlayer,
		},
		{
			Name:    "register-operator-with-avs",
			Aliases: []string{"r"},
			Usage:   "registers bls keys with pubkey-compendium, opts into slashing by avs service-manager, and registers operators with avs registry",
			Action:  actions.RegisterOperatorWithAvs,
		},
		{
			Name:    "update-operator",
			Aliases: []string{"u"},
			Usage:   "update operator stake info",
			Action:  actions.UpdateOperator,
		},
		{
			Name:    "update-operator-bls-key-and-signer",
			Aliases: []string{"u"},
			Usage:   "update operator bls key and signer",
			Action:  actions.UpdateOperatorBlSKeyAndSigner,
		},
		{
			Name:    "start-operator",
			Aliases: []string{"u"},
			Usage:   "start operator",
			Action:  actions.StartOperator,
		},
		{
			Name:    "start-operator-all",
			Aliases: []string{"u"},
			Usage:   "start operator",
			Action:  actions.StartOperatorAll,
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
