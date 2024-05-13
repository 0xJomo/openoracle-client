package chainio

import (
	"log"
	"math/big"

	"github.com/Layr-Labs/eigensdk-go/chainio/clients/eth"
	"github.com/Layr-Labs/eigensdk-go/logging"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	gethcommon "github.com/ethereum/go-ethereum/common"

	erc20mock "avs-oracle/contracts/bindings/ERC20Mock"
	csservicemanager "avs-oracle/contracts/bindings/OpenOracleServiceManager"
	cstaskmanager "avs-oracle/contracts/bindings/OpenOracleTaskManager"

	regcoord "github.com/Layr-Labs/eigensdk-go/contracts/bindings/RegistryCoordinator"
)

type AvsManagersBindings struct {
	TaskManagers   []ChainTaskManager
	ServiceManager *csservicemanager.ContractOpenOracleServiceManager
	ethClient      eth.Client
	logger         logging.Logger
}

type ChainTaskManager struct {
	TaskManager *cstaskmanager.ContractOpenOracleTaskManager
	ChainName   string
}

func NewAvsManagersBindings(registryCoordinatorAddr, operatorStateRetrieverAddr gethcommon.Address, ethclient eth.Client, chainWsClients map[string]eth.Client, logger logging.Logger) (*AvsManagersBindings, error) {
	contractRegistryCoordinator, err := regcoord.NewContractRegistryCoordinator(registryCoordinatorAddr, ethclient)
	if err != nil {
		return nil, err
	}
	serviceManagerAddr, err := contractRegistryCoordinator.ServiceManager(&bind.CallOpts{})
	if err != nil {
		return nil, err
	}
	contractServiceManager, err := csservicemanager.NewContractOpenOracleServiceManager(serviceManagerAddr, ethclient)
	if err != nil {
		logger.Error("Failed to fetch IServiceManager contract", "err", err)
		return nil, err
	}

	// Retrieve the total count of task managers
	count, err := contractServiceManager.TaskManagerCount(nil)
	if err != nil {
		logger.Error("Failed to retrieve task manager count: %v", err)
		return nil, err
	}

	// Slice to store active task managers
	var activeTaskManagers []ChainTaskManager

	// Iterate through all task managers and check if they are active
	for i := big.NewInt(0); i.Cmp(count) < 0; i.Add(i, big.NewInt(1)) {
		taskManager, err := contractServiceManager.TaskManagers(nil, i)
		if err != nil {
			log.Printf("Failed to retrieve task manager at index %v: %v", i, err)
			continue
		}
		chainWsClient, ok := chainWsClients[taskManager.ChainName]
		if !ok {
			log.Printf("Chain url not found", taskManager.ChainName)
			continue
		}
		if taskManager.IsActive {
			// TODO: fetch from config based on task manager chain
			contractTaskManager, err := cstaskmanager.NewContractOpenOracleTaskManager(taskManager.TaskManagerAddress, chainWsClient)
			if err != nil {
				logger.Error("Failed to fetch IOpenOracleTaskManager contract", "err", err)
				return nil, err
			}
			activeTaskManagers = append(activeTaskManagers, ChainTaskManager{
				contractTaskManager,
				taskManager.ChainName,
			})
		}
	}

	return &AvsManagersBindings{
		ServiceManager: contractServiceManager,
		TaskManagers:   activeTaskManagers,
		ethClient:      ethclient,
		logger:         logger,
	}, nil
}

func (b *AvsManagersBindings) GetErc20Mock(tokenAddr common.Address) (*erc20mock.ContractERC20Mock, error) {
	contractErc20Mock, err := erc20mock.NewContractERC20Mock(tokenAddr, b.ethClient)
	if err != nil {
		b.logger.Error("Failed to fetch ERC20Mock contract", "err", err)
		return nil, err
	}
	return contractErc20Mock, nil
}
