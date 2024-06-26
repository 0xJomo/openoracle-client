package chainio

import (
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	gethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/event"

	"github.com/Layr-Labs/eigensdk-go/chainio/clients/eth"
	sdklogging "github.com/Layr-Labs/eigensdk-go/logging"

	cstaskmanager "avs-oracle/contracts/bindings/OpenOracleTaskManager"
)

type AvsSubscriberer interface {
	SubscribeToNewTasks(taskManager ChainTaskManager, newTaskCreatedChan chan *cstaskmanager.ContractOpenOracleTaskManagerNewTaskCreated) event.Subscription
	SubscribeToTaskResponses(taskManager *cstaskmanager.ContractOpenOracleTaskManager, taskResponseChan chan *cstaskmanager.ContractOpenOracleTaskManagerTaskResponded) event.Subscription
	GetAvsContractBindings() *AvsManagersBindings
}

// Subscribers use a ws connection instead of http connection like Readers
// kind of stupid that the geth client doesn't have a unified interface for both...
// it takes a single url, so the bindings, even though they have watcher functions, those can't be used
// with the http connection... seems very very stupid. Am I missing something?
type AvsSubscriber struct {
	AvsContractBindings *AvsManagersBindings
	logger              sdklogging.Logger
}

func BuildAvsSubscriber(registryCoordinatorAddr, blsOperatorStateRetrieverAddr gethcommon.Address, ethclient eth.Client, chainWsClients map[string]eth.Client, logger sdklogging.Logger) (*AvsSubscriber, error) {
	avsContractBindings, err := NewAvsManagersBindings(registryCoordinatorAddr, blsOperatorStateRetrieverAddr, ethclient, chainWsClients, logger)
	if err != nil {
		logger.Errorf("Failed to create contract bindings", "err", err)
		return nil, err
	}
	return NewAvsSubscriber(avsContractBindings, logger), nil
}

func (s *AvsSubscriber) GetAvsContractBindings() *AvsManagersBindings {
	return s.AvsContractBindings
}

func NewAvsSubscriber(avsContractBindings *AvsManagersBindings, logger sdklogging.Logger) *AvsSubscriber {
	return &AvsSubscriber{
		AvsContractBindings: avsContractBindings,
		logger:              logger,
	}
}

func (s *AvsSubscriber) SubscribeToNewTasks(taskManager ChainTaskManager, newTaskCreatedChan chan *cstaskmanager.ContractOpenOracleTaskManagerNewTaskCreated) event.Subscription {
	sub, err := taskManager.TaskManager.WatchNewTaskCreated(
		&bind.WatchOpts{}, newTaskCreatedChan, nil,
	)

	if err != nil {
		s.logger.Error("Failed to subscribe to new TaskManager tasks", taskManager.ChainName, "err", err)
	}
	s.logger.Infof("Subscribed to new TaskManager tasks", taskManager.ChainName)
	return sub
}

func (s *AvsSubscriber) SubscribeToTaskResponses(taskManager *cstaskmanager.ContractOpenOracleTaskManager, taskResponseChan chan *cstaskmanager.ContractOpenOracleTaskManagerTaskResponded) event.Subscription {
	sub, err := taskManager.WatchTaskResponded(
		&bind.WatchOpts{}, taskResponseChan,
	)
	if err != nil {
		s.logger.Error("Failed to subscribe to TaskResponded events", "err", err)
	}
	s.logger.Infof("Subscribed to TaskResponded events")
	return sub
}
