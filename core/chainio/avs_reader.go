package chainio

import (
	"context"
	"github.com/Layr-Labs/eigensdk-go/types"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	gethcommon "github.com/ethereum/go-ethereum/common"

	sdkavsregistry "github.com/Layr-Labs/eigensdk-go/chainio/clients/avsregistry"
	"github.com/Layr-Labs/eigensdk-go/chainio/clients/eth"
	logging "github.com/Layr-Labs/eigensdk-go/logging"

	erc20mock "avs-oracle/contracts/bindings/ERC20Mock"
	cstaskmanager "avs-oracle/contracts/bindings/OpenOracleTaskManager"
	regcoord "avs-oracle/contracts/bindings/RegistryCoordinator"
	"avs-oracle/core/config"
)

type AvsReaderer interface {
	sdkavsregistry.AvsRegistryReader
	GetErc20Mock(ctx context.Context, tokenAddr gethcommon.Address) (*erc20mock.ContractERC20Mock, error)

	GetOperatorBlsKeyAndSignAddr(
		opts *bind.CallOpts, addr types.OperatorAddr,
	) (regcoord.IRegistryCoordinatorOperatorBlsKeyAndSigner, error)
}

type AvsReader struct {
	sdkavsregistry.AvsRegistryReader
	AvsServiceBindings  *AvsManagersBindings
	RegistryCoordinator *regcoord.ContractRegistryCoordinator
	logger              logging.Logger
}

var _ AvsReaderer = (*AvsReader)(nil)

func BuildAvsReaderFromConfig(c *config.Config) (*AvsReader, error) {
	return BuildAvsReader(c.OpenOracleRegistryCoordinatorAddr, c.OperatorStateRetrieverAddr, c.EthHttpClient, c.Logger)
}
func BuildAvsReader(registryCoordinatorAddr, operatorStateRetrieverAddr gethcommon.Address, ethHttpClient eth.Client, logger logging.Logger) (*AvsReader, error) {
	avsManagersBindings, err := NewAvsManagersBindings(registryCoordinatorAddr, operatorStateRetrieverAddr, ethHttpClient, make(map[string]eth.Client), logger)
	if err != nil {
		return nil, err
	}
	avsRegistryReader, err := sdkavsregistry.BuildAvsRegistryChainReader(registryCoordinatorAddr, operatorStateRetrieverAddr, ethHttpClient, logger)
	if err != nil {
		return nil, err
	}
	registryCoordinator, err := regcoord.NewContractRegistryCoordinator(registryCoordinatorAddr, ethHttpClient)
	return NewAvsReader(avsRegistryReader, avsManagersBindings, registryCoordinator, logger)
}
func NewAvsReader(avsRegistryReader sdkavsregistry.AvsRegistryReader, avsServiceBindings *AvsManagersBindings, registryCoordinator *regcoord.ContractRegistryCoordinator, logger logging.Logger) (*AvsReader, error) {
	return &AvsReader{
		AvsRegistryReader:   avsRegistryReader,
		AvsServiceBindings:  avsServiceBindings,
		RegistryCoordinator: registryCoordinator,
		logger:              logger,
	}, nil
}

func (r *AvsReader) GetErc20Mock(ctx context.Context, tokenAddr gethcommon.Address) (*erc20mock.ContractERC20Mock, error) {
	erc20Mock, err := r.AvsServiceBindings.GetErc20Mock(tokenAddr)
	if err != nil {
		r.logger.Error("Failed to fetch ERC20Mock contract", "err", err)
		return nil, err
	}
	return erc20Mock, nil
}

func (r *AvsReader) GetOperatorBlsKeyAndSignAddr(
	opts *bind.CallOpts, addr types.OperatorAddr,
) (regcoord.IRegistryCoordinatorOperatorBlsKeyAndSigner, error) {
	keyAndSignAddr, err := r.RegistryCoordinator.GetOperatorBlsKeyAndSignAddr(
		opts, addr,
	)
	if err != nil {
		return regcoord.IRegistryCoordinatorOperatorBlsKeyAndSigner{}, err
	}
	return keyAndSignAddr, nil
}
