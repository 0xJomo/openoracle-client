package actions

import (
	"encoding/json"
	"log"
	"os"

	"avs-oracle/core/config"
	"avs-oracle/operator"
	"avs-oracle/types"

	sdkecdsa "github.com/Layr-Labs/eigensdk-go/crypto/ecdsa"
	sdkutils "github.com/Layr-Labs/eigensdk-go/utils"
	"github.com/urfave/cli"
)

func RegisterOperatorWithAvs(ctx *cli.Context) error {

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

	ecdsaKeyPassword, ok := os.LookupEnv("OPERATOR_ECDSA_KEY_PASSWORD")
	if !ok {
		log.Printf("OPERATOR_ECDSA_KEY_PASSWORD env var not set. using empty string")
	}
	operatorEcdsaPrivKey, err := sdkecdsa.ReadKey(
		nodeConfig.EcdsaPrivateKeyStorePath,
		ecdsaKeyPassword,
	)
	if err != nil {
		return err
	}

	err = operator.RegisterOperatorWithAvs(operatorEcdsaPrivKey)
	if err != nil {
		return err
	}

	return nil
}
