package actions

import (
	"avs-oracle/core/config"
	"avs-oracle/operator"
	"avs-oracle/types"
	"context"
	"encoding/json"
	"log"
	"os"

	sdkutils "github.com/Layr-Labs/eigensdk-go/utils"
	"github.com/urfave/cli"
)

func UpdateOperatorBlSKeyAndSigner(ctx *cli.Context) error {

	configPath := ctx.GlobalString(config.ConfigFileFlag.Name)
	nodeConfig := types.NodeConfig{}
	err := sdkutils.ReadYamlConfig(configPath, &nodeConfig)
	if err != nil {
		return err
	}
	nodeConfig.OperatorAddress, _ = os.LookupEnv("OPERATOR_ADDRESS")
	nodeConfig.OperatorSignatureAddress, _ = os.LookupEnv("OPERATOR_SIGNATURE_ADDRESS")
	nodeConfig.BlsPrivateKeyStorePath, _ = os.LookupEnv("BLS_PRIVATE_KEY_PATH")
	nodeConfig.EcdsaPrivateKeyStorePath, _ = os.LookupEnv("ECDSA_PRIVATE_KEY_PATH")
	nodeConfig.EcdsaPrivateSignKeyStorePath, _ = os.LookupEnv("ECDSA_SIGNER_PRIVATE_KEY_PATH")
	nodeConfig.EthRpcUrl, _ = os.LookupEnv("HTTP_RPC_URL")
	nodeConfig.EthWsUrl, _ = os.LookupEnv("WS_RPC_URL")

	// need to make sure we don't register the operator on startup
	// when using the cli commands to register the operator.
	nodeConfig.RegisterOperatorOnStartup = false

	configJson, err := json.MarshalIndent(nodeConfig, "", "  ")
	if err != nil {
		log.Fatalf(err.Error())
	}
	log.Println("Config:", string(configJson))

	operator, err := operator.NewOperatorFromConfig(nodeConfig)
	if err != nil {
		return err
	}

	err = operator.UpdateOperatorBlsKeyAndSigner(context.Background())
	if err != nil {
		return err
	}

	return nil
}
