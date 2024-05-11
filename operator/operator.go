package operator

import (
	"context"
	cryptoecdsa "crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/prometheus/client_golang/prometheus"

	cstaskmanager "avs-oracle/contracts/bindings/OpenOracleTaskManager"
	"avs-oracle/core"
	"avs-oracle/core/chainio"
	"avs-oracle/metrics"
	"avs-oracle/types"

	"github.com/Layr-Labs/eigensdk-go/chainio/clients"
	sdkelcontracts "github.com/Layr-Labs/eigensdk-go/chainio/clients/elcontracts"
	"github.com/Layr-Labs/eigensdk-go/chainio/clients/eth"
	"github.com/Layr-Labs/eigensdk-go/chainio/clients/wallet"
	"github.com/Layr-Labs/eigensdk-go/chainio/txmgr"
	"github.com/Layr-Labs/eigensdk-go/crypto/bls"
	"github.com/Layr-Labs/eigensdk-go/crypto/ecdsa"
	sdkecdsa "github.com/Layr-Labs/eigensdk-go/crypto/ecdsa"
	"github.com/Layr-Labs/eigensdk-go/logging"
	sdklogging "github.com/Layr-Labs/eigensdk-go/logging"
	sdkmetrics "github.com/Layr-Labs/eigensdk-go/metrics"
	"github.com/Layr-Labs/eigensdk-go/metrics/collectors/economic"
	rpccalls "github.com/Layr-Labs/eigensdk-go/metrics/collectors/rpc_calls"
	"github.com/Layr-Labs/eigensdk-go/nodeapi"
	"github.com/Layr-Labs/eigensdk-go/signerv2"
	sdktypes "github.com/Layr-Labs/eigensdk-go/types"
)

const AVS_NAME = "open-oracle"
const SEM_VER = "0.0.1"

type Operator struct {
	config    types.NodeConfig
	logger    logging.Logger
	ethClient eth.Client
	// TODO(samlaf): remove both avsWriter and eigenlayerWrite from operator
	// they are only used for registration, so we should make a special registration package
	// this way, auditing this operator code makes it obvious that operators don't need to
	// write to the chain during the course of their normal operations
	// writing to the chain should be done via the cli only
	metricsReg            *prometheus.Registry
	metrics               metrics.Metrics
	nodeApi               *nodeapi.NodeApi
	chainName             string
	avsWriter             *chainio.AvsWriter
	avsReader             chainio.AvsReaderer
	avsSubscriber         chainio.AvsSubscriberer
	eigenlayerReader      sdkelcontracts.ELReader
	eigenlayerWriter      sdkelcontracts.ELWriter
	blsKeypair            *bls.KeyPair
	ecdsaKey              *cryptoecdsa.PrivateKey
	ecdsaSignKey          *cryptoecdsa.PrivateKey
	operatorId            sdktypes.OperatorId
	operatorAddr          common.Address
	operatorSignatureAddr common.Address
	// receive new tasks in this chan (typically from listening to onchain event)
	newTaskCreatedChan chan *cstaskmanager.ContractOpenOracleTaskManagerNewTaskCreated
	// ip address of aggregator
	aggregatorServerIpPortAddr string
	// rpc client to send signed task responses to aggregator
	aggregatorRpcClient AggregatorRpcClienter
	// needed when opting in to avs (allow this service manager contract to slash operator)
	credibleSquaringServiceManagerAddr common.Address
}

// TODO(samlaf): config is a mess right now, since the chainio client constructors
//
//	take the config in core (which is shared with aggregator and challenger)
func NewOperatorFromConfig(c types.NodeConfig) (*Operator, error) {
	var logLevel logging.LogLevel
	if c.Production {
		logLevel = sdklogging.Production
	} else {
		logLevel = sdklogging.Development
	}
	logger, err := sdklogging.NewZapLogger(logLevel)
	if err != nil {
		return nil, err
	}
	reg := prometheus.NewRegistry()
	eigenMetrics := sdkmetrics.NewEigenMetrics(AVS_NAME, c.EigenMetricsIpPortAddress, reg, logger)
	avsAndEigenMetrics := metrics.NewAvsAndEigenMetrics(AVS_NAME, eigenMetrics, reg)

	// Setup Node Api
	nodeApi := nodeapi.NewNodeApi(AVS_NAME, SEM_VER, c.NodeApiIpPortAddress, logger)

	var ethRpcClient, ethWsClient eth.Client
	if c.EnableMetrics {
		rpcCallsCollector := rpccalls.NewCollector(AVS_NAME, reg)
		ethRpcClient, err = eth.NewInstrumentedClient(c.EthRpcUrl, rpcCallsCollector)
		if err != nil {
			logger.Errorf("Cannot create http ethclient", "err", err)
			return nil, err
		}
		ethWsClient, err = eth.NewInstrumentedClient(c.EthWsUrl, rpcCallsCollector)
		if err != nil {
			logger.Errorf("Cannot create ws ethclient", "err", err)
			return nil, err
		}
	} else {
		ethRpcClient, err = eth.NewClient(c.EthRpcUrl)
		if err != nil {
			logger.Errorf("Cannot create http ethclient", "err", err)
			return nil, err
		}
		ethWsClient, err = eth.NewClient(c.EthWsUrl)
		if err != nil {
			logger.Errorf("Cannot create ws ethclient", "err", err)
			return nil, err
		}
	}

	blsKeyPassword, ok := os.LookupEnv("OPERATOR_BLS_KEY_PASSWORD")
	if !ok {
		logger.Warnf("OPERATOR_BLS_KEY_PASSWORD env var not set. using empty string")
	}
	blsKeyPair, err := bls.ReadPrivateKeyFromFile(c.BlsPrivateKeyStorePath, blsKeyPassword)
	if err != nil {
		logger.Errorf("Cannot parse bls private key", "err", err)
		return nil, err
	}
	// TODO(samlaf): should we add the chainId to the config instead?
	// this way we can prevent creating a signer that signs on mainnet by mistake
	// if the config says chainId=5, then we can only create a goerli signer
	chainId, err := ethRpcClient.ChainID(context.Background())
	if err != nil {
		logger.Error("Cannot get chainId", "err", err)
		return nil, err
	}

	// Get ECDSA key and signer
	ecdsaKeyPassword, ok := os.LookupEnv("OPERATOR_ECDSA_KEY_PASSWORD")
	if !ok {
		logger.Warnf("OPERATOR_ECDSA_KEY_PASSWORD env var not set. using empty string")
	}
	ecdsaKey, err := ecdsa.ReadKey(c.EcdsaPrivateKeyStorePath, ecdsaKeyPassword)
	signerV2, _, err := signerv2.SignerFromConfig(signerv2.Config{
		KeystorePath: c.EcdsaPrivateKeyStorePath,
		Password:     ecdsaKeyPassword,
	}, chainId)

	ecdsaSignerKeyPassword, ok := os.LookupEnv("OPERATOR_SIGNER_ECDSA_KEY_PASSWORD")
	if !ok {
		logger.Warnf("OPERATOR_SIGNER_ECDSA_KEY_PASSWORD env var not set. using empty string")
	}
	// TODO: stop gap solution here to separate start operator vs register operator
	// When starting operator, the operator private key won't be here and won't be used
	// So we use the signer private key here just to fill the space
	if err != nil {
		ecdsaKey, err = ecdsa.ReadKey(c.EcdsaPrivateSignKeyStorePath, ecdsaSignerKeyPassword)
		signerV2, _, err = signerv2.SignerFromConfig(signerv2.Config{
			KeystorePath: c.EcdsaPrivateSignKeyStorePath,
			Password:     ecdsaSignerKeyPassword,
		}, chainId)
	}

	// If unable to get any signer, panic
	if err != nil {
		panic(err)
	}

	// Get the ECDSA key for signing task responses
	ecdsaSignKey, err := ecdsa.ReadKey(c.EcdsaPrivateSignKeyStorePath, ecdsaSignerKeyPassword)
	if err != nil {
		panic(err)
	}
	chainioConfig := clients.BuildAllConfig{
		EthHttpUrl:                 c.EthRpcUrl,
		EthWsUrl:                   c.EthWsUrl,
		RegistryCoordinatorAddr:    c.AVSRegistryCoordinatorAddress,
		OperatorStateRetrieverAddr: c.OperatorStateRetrieverAddress,
		AvsName:                    AVS_NAME,
		PromMetricsIpPortAddress:   c.EigenMetricsIpPortAddress,
	}
	sdkClients, err := clients.BuildAll(chainioConfig, ecdsaKey, logger)
	if err != nil {
		panic(err)
	}
	wallet, err := wallet.NewPrivateKeyWallet(ethRpcClient, signerV2, common.HexToAddress(c.OperatorAddress), logger)
	txMgr := txmgr.NewSimpleTxManager(wallet, ethRpcClient, logger, common.HexToAddress(c.OperatorAddress))

	avsWriter, err := chainio.BuildAvsWriter(
		txMgr, common.HexToAddress(c.AVSRegistryCoordinatorAddress),
		common.HexToAddress(c.OperatorStateRetrieverAddress), ethRpcClient, logger,
	)
	if err != nil {
		logger.Error("Cannot create AvsWriter", "err", err)
		return nil, err
	}

	avsReader, err := chainio.BuildAvsReader(
		common.HexToAddress(c.AVSRegistryCoordinatorAddress),
		common.HexToAddress(c.OperatorStateRetrieverAddress),
		ethRpcClient, logger)
	if err != nil {
		logger.Error("Cannot create AvsReader", "err", err)
		return nil, err
	}
	avsSubscriber, err := chainio.BuildAvsSubscriber(common.HexToAddress(c.AVSRegistryCoordinatorAddress),
		common.HexToAddress(c.OperatorStateRetrieverAddress), ethWsClient, logger,
	)
	if err != nil {
		logger.Error("Cannot create AvsSubscriber", "err", err)
		return nil, err
	}

	// We must register the economic metrics separately because they are exported metrics (from jsonrpc or subgraph calls)
	// and not instrumented metrics: see https://prometheus.io/docs/instrumenting/writing_clientlibs/#overall-structure
	quorumNames := map[sdktypes.QuorumNum]string{
		0: "quorum0",
	}
	economicMetricsCollector := economic.NewCollector(
		sdkClients.ElChainReader, sdkClients.AvsRegistryChainReader,
		AVS_NAME, logger, common.HexToAddress(c.OperatorAddress), quorumNames)
	reg.MustRegister(economicMetricsCollector)

	aggregatorRpcClient, err := NewAggregatorRpcClient(c.AggregatorServerIpPortAddress, logger, avsAndEigenMetrics)
	if err != nil {
		logger.Error("Cannot create AggregatorRpcClient. Is aggregator running?", "err", err)
		return nil, err
	}

	operator := &Operator{
		config:                             c,
		logger:                             logger,
		metricsReg:                         reg,
		metrics:                            avsAndEigenMetrics,
		nodeApi:                            nodeApi,
		ethClient:                          ethRpcClient,
		chainName:                          c.ChainName,
		avsWriter:                          avsWriter,
		avsReader:                          avsReader,
		avsSubscriber:                      avsSubscriber,
		eigenlayerReader:                   sdkClients.ElChainReader,
		eigenlayerWriter:                   sdkClients.ElChainWriter,
		blsKeypair:                         blsKeyPair,
		ecdsaKey:                           ecdsaKey,
		ecdsaSignKey:                       ecdsaSignKey,
		operatorAddr:                       common.HexToAddress(c.OperatorAddress),
		operatorSignatureAddr:              common.HexToAddress(c.OperatorSignatureAddress),
		aggregatorServerIpPortAddr:         c.AggregatorServerIpPortAddress,
		aggregatorRpcClient:                aggregatorRpcClient,
		newTaskCreatedChan:                 make(chan *cstaskmanager.ContractOpenOracleTaskManagerNewTaskCreated),
		credibleSquaringServiceManagerAddr: common.HexToAddress(c.AVSRegistryCoordinatorAddress),
		operatorId:                         [32]byte{0}, // this is set below

	}

	if c.RegisterOperatorOnStartup {
		operatorEcdsaPrivateKey, err := sdkecdsa.ReadKey(
			c.EcdsaPrivateKeyStorePath,
			ecdsaKeyPassword,
		)
		if err != nil {
			return nil, err
		}
		operator.registerOperatorOnStartup(operatorEcdsaPrivateKey)
	}

	// OperatorId is set in contract during registration so we get it after registering operator.
	operatorId, err := sdkClients.AvsRegistryChainReader.GetOperatorId(&bind.CallOpts{}, operator.operatorAddr)
	if err != nil {
		logger.Error("Cannot get operator id", "err", err)
		return nil, err
	}
	operator.operatorId = operatorId
	logger.Info("Operator info",
		"operatorId", operatorId,
		"operatorAddr", c.OperatorAddress,
		"operatorG1Pubkey", operator.blsKeypair.GetPubKeyG1(),
		"operatorG2Pubkey", operator.blsKeypair.GetPubKeyG2(),
	)

	return operator, nil

}

func (o *Operator) Start(ctx context.Context) error {
	operatorIsRegistered, err := o.avsReader.IsOperatorRegistered(&bind.CallOpts{}, o.operatorAddr)
	if err != nil {
		o.logger.Error("Error checking if operator is registered", "err", err)
		return err
	}
	if !operatorIsRegistered {
		// We bubble the error all the way up instead of using logger.Fatal because logger.Fatal prints a huge stack trace
		// that hides the actual error message. This error msg is more explicit and doesn't require showing a stack trace to the user.
		return fmt.Errorf("operator is not registered. Registering operator using the operator-cli before starting operator")
	}

	o.logger.Infof("Starting operator.")

	if o.config.EnableNodeApi {
		o.nodeApi.Start()
	}
	var metricsErrChan <-chan error
	if o.config.EnableMetrics {
		metricsErrChan = o.metrics.Start(ctx, o.metricsReg)
	} else {
		metricsErrChan = make(chan error, 1)
	}

	// TODO(samlaf): wrap this call with increase in avs-node-spec metric
	sub := o.avsSubscriber.SubscribeToNewTasks(o.newTaskCreatedChan)
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-metricsErrChan:
			// TODO(samlaf); we should also register the service as unhealthy in the node api
			// https://eigen.nethermind.io/docs/spec/api/
			o.logger.Fatal("Error in metrics server", "err", err)
		case err := <-sub.Err():
			o.logger.Error("Error in websocket subscription", "err", err)
			// TODO(samlaf): write unit tests to check if this fixed the issues we were seeing
			sub.Unsubscribe()
			// TODO(samlaf): wrap this call with increase in avs-node-spec metric
			sub = o.avsSubscriber.SubscribeToNewTasks(o.newTaskCreatedChan)
		case newTaskCreatedLog := <-o.newTaskCreatedChan:
			o.metrics.IncNumTasksReceived()
			// TODO: call aggregator API to check if task has received enough response
			taskResponse, err := o.ProcessNewTaskCreatedLog(newTaskCreatedLog)
			if err != nil {
				continue
			}
			signedTaskResponse, err := o.SignTaskResponse(taskResponse)
			if err != nil {
				continue
			}
			// TODO: send response to aggregator
			o.logger.Info("Processed task", "success", signedTaskResponse)
			SendECDSASignedRequest(signedTaskResponse, o.config.AggregatorServerIpPortAddress)
			o.metrics.IncNumTasksAcceptedByAggregator()
		}
	}
}

// Takes a NewTaskCreatedLog struct as input and returns a TaskResponseHeader struct.
// The TaskResponseHeader struct is the struct that is signed and sent to the contract as a task response.
func (o *Operator) ProcessNewTaskCreatedLog(newTaskCreatedLog *cstaskmanager.ContractOpenOracleTaskManagerNewTaskCreated) (*cstaskmanager.IOpenOracleTaskManagerTaskResponse, error) {
	o.logger.Debug("Received new task", "task", newTaskCreatedLog)
	o.logger.Info("Received new task",
		"taskType", newTaskCreatedLog.Task.TaskType,
		"taskIndex", newTaskCreatedLog.TaskIndex,
		"taskCreatedBlock", newTaskCreatedLog.Task.TaskCreatedBlock,
		"responderThreshold", newTaskCreatedLog.Task.ResponderThreshold,
		"stakeThreshold", newTaskCreatedLog.Task.StakeThreshold,
		"creator", newTaskCreatedLog.Task.Creator,
		"creationFee", newTaskCreatedLog.Task.CreationFee,
	)

	// goldPrice, error := FetchGoldPrice()
	// if error != nil {
	// 	o.logger.Error("Fetching gold price", "error", error)
	// 	return nil, error
	// }

	price, error := o.FetchPrice(newTaskCreatedLog.Task.TaskType)
	if error != nil {
		o.logger.Error("Fetching price", "error", error)
		return nil, error
	}

	taskResponse := cstaskmanager.IOpenOracleTaskManagerTaskResponse{
		ReferenceTaskIndex: newTaskCreatedLog.TaskIndex,
		Result:             big.NewInt(price),
		Timestamp:          big.NewInt(time.Now().Unix()),
	}
	return &taskResponse, nil
}

func (o *Operator) SignTaskResponse(taskResponse *cstaskmanager.IOpenOracleTaskManagerTaskResponse) (SignedTaskResponse, error) {
	taskResponseHash, err := core.GetTaskResponseDigest(taskResponse)
	if err != nil {
		log.Fatalf("Error serializing TaskResponse to JSON: %v", err)
		return SignedTaskResponse{}, err
	}
	prefixedHash := crypto.Keccak256Hash([]byte(fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(taskResponseHash), taskResponseHash)))
	// Sign the hash using the operator's ECDSA private key
	signature, err := crypto.Sign(prefixedHash[:], o.ecdsaSignKey)
	// Adjust 'v' value; Ethereum expects 'v' to be 27 or 28
	// Go's crypto.Sign returns 'v' as 0 or 1, so we adjust it by adding 27
	if signature[64] < 27 {
		signature[64] += 27
	}
	o.logger.Info("Signed task response", "signature64", signature[64])
	if err != nil {
		log.Fatalf("Error signing task response: %v", err)
		return SignedTaskResponse{}, err
	}
	if err != nil {
		log.Fatalf("Error signing task response: %v", err)
		return SignedTaskResponse{}, err
	}
	signedTaskResponse := SignedTaskResponse{
		TaskResponse:    taskResponse,
		Signature:       "0x" + hex.EncodeToString(signature),
		ChainName:       o.chainName,
		OperatorAddress: o.config.OperatorAddress,
	}
	o.logger.Debug("Signed task response", "signature", hex.EncodeToString(signature))
	o.logger.Debug("Signed task response", "taskResponse", taskResponse)
	o.logger.Debug("Signed task response", "taskResponseHash", hex.EncodeToString(taskResponseHash[:]))
	o.logger.Debug("Signed task response", "signedTaskResponse", signedTaskResponse)
	return signedTaskResponse, nil
}
