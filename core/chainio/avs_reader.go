package chainio

import (
	"context"
	"errors"
	"github.com/Layr-Labs/eigensdk-go/types"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	gethcommon "github.com/ethereum/go-ethereum/common"

	sdkavsregistry "github.com/Layr-Labs/eigensdk-go/chainio/clients/avsregistry"
	"github.com/Layr-Labs/eigensdk-go/chainio/clients/eth"
	logging "github.com/Layr-Labs/eigensdk-go/logging"

	blsapkregistry "avs-oracle/contracts/bindings/BLSApkRegistry"
	erc20mock "avs-oracle/contracts/bindings/ERC20Mock"
	regcoord "avs-oracle/contracts/bindings/RegistryCoordinator"
	stakeregistry "avs-oracle/contracts/bindings/StakeRegistry"
	"avs-oracle/core/config"
	egtypes "github.com/Layr-Labs/eigensdk-go/types"
)

type AvsReaderer interface {
	sdkavsregistry.AvsRegistryReader
	GetErc20Mock(ctx context.Context, tokenAddr gethcommon.Address) (*erc20mock.ContractERC20Mock, error)

	GetOperatorBlsKeyAndSignAddr(
		opts *bind.CallOpts, addr types.OperatorAddr,
	) (blsapkregistry.BN254G1Point, types.Bytes32, gethcommon.Address, error)
}

type AvsReader struct {
	sdkavsregistry.AvsRegistryReader
	AvsServiceBindings  *AvsManagersBindings
	RegistryCoordinator *regcoord.ContractRegistryCoordinator
	BLSApkRegistry      *blsapkregistry.ContractBLSApkRegistry
	StakeRegistry       *stakeregistry.ContractStakeRegistry
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
	if err != nil {
		return nil, err
	}
	blsApkRegistryAddr, err := registryCoordinator.BlsApkRegistry(&bind.CallOpts{})
	if err != nil {
		return nil, egtypes.WrapError(errors.New("failed to get BLSApkRegistry address"), err)
	}
	blsApkRegistry, err := blsapkregistry.NewContractBLSApkRegistry(blsApkRegistryAddr, ethHttpClient)
	if err != nil {
		return nil, err
	}
	stakeRegistryAddr, err := registryCoordinator.StakeRegistry(&bind.CallOpts{})
	if err != nil {
		return nil, egtypes.WrapError(errors.New("failed to get StakeRegistry address"), err)
	}
	stakeRegistry, err := stakeregistry.NewContractStakeRegistry(stakeRegistryAddr, ethHttpClient)
	if err != nil {
		return nil, err
	}
	return NewAvsReader(avsRegistryReader, avsManagersBindings, registryCoordinator, blsApkRegistry, stakeRegistry, logger)
}
func NewAvsReader(avsRegistryReader sdkavsregistry.AvsRegistryReader, avsServiceBindings *AvsManagersBindings, registryCoordinator *regcoord.ContractRegistryCoordinator, blsApkRegistry *blsapkregistry.ContractBLSApkRegistry, stakeRegistry *stakeregistry.ContractStakeRegistry, logger logging.Logger) (*AvsReader, error) {
	return &AvsReader{
		AvsRegistryReader:   avsRegistryReader,
		AvsServiceBindings:  avsServiceBindings,
		RegistryCoordinator: registryCoordinator,
		BLSApkRegistry:      blsApkRegistry,
		StakeRegistry:       stakeRegistry,
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
) (blsapkregistry.BN254G1Point, types.Bytes32, gethcommon.Address, error) {
	g1Point, pubkeyHash, err := r.BLSApkRegistry.GetRegisteredPubkey(
		opts, addr,
	)
	if err != nil {
		return blsapkregistry.BN254G1Point{}, types.Bytes32{}, gethcommon.Address{}, err
	}
	singerAddr, err := r.StakeRegistry.GetOperatorSignAddress(opts, addr)
	if err != nil {
		return blsapkregistry.BN254G1Point{}, types.Bytes32{}, gethcommon.Address{}, err
	}
	return g1Point, pubkeyHash, singerAddr, nil
}
