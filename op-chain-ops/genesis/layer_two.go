package genesis

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-bindings/bindings"
	"github.com/ethereum-optimism/optimism/op-bindings/predeploys"
	"github.com/ethereum-optimism/optimism/op-chain-ops/deployer"
	"github.com/ethereum-optimism/optimism/op-chain-ops/immutables"
	"github.com/ethereum-optimism/optimism/op-chain-ops/squash"
	"github.com/ethereum-optimism/optimism/op-chain-ops/state"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

// BuildL2Genesis will build the L2 genesis block.
func BuildL2Genesis(config *DeployConfig, l1StartBlock *types.Block) (*core.Genesis, error) {
	log.Info("here we are")
	genspec, err := NewL2Genesis(config, l1StartBlock)
	if err != nil {
		return nil, err
	}
	log.Info("here we are genspec", "genspec", genspec, "alloc", genspec.Alloc)
	db := state.NewMemoryStateDB(genspec)
	log.Info("inspecting alloc agian", "alloc", genspec.Alloc)
	if config.FundDevAccounts {
		log.Info("Funding developer accounts in L2 genesis")
		FundDevAccounts(db)
	}
	log.Info("SetPrecompileBalances")
	SetPrecompileBalances(db)
	log.Info("inspecting alloc agian2", "alloc", genspec.Alloc)
	log.Info("NewL2StorageConfig")
	storage, err := NewL2StorageConfig(config, l1StartBlock)
	if err != nil {
		return nil, err
	}
	immutableConfig, err := NewL2ImmutableConfig(config, l1StartBlock)
	if err != nil {
		return nil, err
	}

	// Set up the proxies for Optimism Predeploys
	err = setProxies(db, predeploys.ProxyAdminAddr, BigL2PredeployNamespace, 2048)
	if err != nil {
		return nil, err
	}

	// Set up the proxies for Story Predeploys PT and Ecosystem
	err = setProxies(db, predeploys.ProxyAdminAddr, BigL2PredeployNamespacePT, 2048)
	if err != nil {
		return nil, err
	}
	err = setProxies(db, predeploys.ProxyAdminAddr, BigL2PredeployNamespaceEcosystem, 2048)
	if err != nil {
		return nil, err
	}
	// Set up the implementations that contain immutables
	deployResults, err := immutables.Deploy(immutableConfig)
	if err != nil {
		return nil, err
	}
	for name, predeploy := range predeploys.Predeploys {

		if predeploy.Enabled != nil && !predeploy.Enabled(config) {
			log.Warn("Skipping disabled predeploy.", "name", name, "address", predeploy.Address)
			continue
		}

		codeAddr := predeploy.Address
		switch name {
		case "Permit2":
			deployerAddressBytes, err := bindings.GetDeployerAddress(name)
			if err != nil {
				return nil, err
			}
			deployerAddress := common.BytesToAddress(deployerAddressBytes)
			predeploys := map[string]*common.Address{
				"DeterministicDeploymentProxy": &deployerAddress,
			}
			backend, err := deployer.NewL2BackendWithChainIDAndPredeploys(
				new(big.Int).SetUint64(config.L2ChainID),
				predeploys,
			)
			if err != nil {
				return nil, err
			}
			deployedBin, err := deployer.DeployWithDeterministicDeployer(backend, name)
			if err != nil {
				return nil, err
			}
			deployResults[name] = deployedBin
			fallthrough
		case "MultiCall3", "Create2Deployer", "Safe_v130",
			"SafeL2_v130", "MultiSendCallOnly_v130", "SafeSingletonFactory",
			"DeterministicDeploymentProxy", "MultiSend_v130", "SenderCreator", "EntryPoint":
			db.CreateAccount(codeAddr)
		default:
			if !predeploy.ProxyDisabled {
				log.Info("!predeploy.ProxyDisabled", "name", name, "predeploy", predeploy.Address)
				codeAddr, err = AddressToCodeNamespace(predeploy.Address)
				log.Info("!predeploy.ProxyDisabled", "codeAddr", codeAddr)
				if err != nil {
					return nil, fmt.Errorf("error converting to code namespace: %w", err)
				}
				db.CreateAccount(codeAddr)
				log.Info("eth.AddressAsLeftPaddedHash(codeAddr)", "hash", eth.AddressAsLeftPaddedHash(codeAddr))
				db.SetState(predeploy.Address, ImplementationSlot, eth.AddressAsLeftPaddedHash(codeAddr))
				log.Info("Set proxy", "name", name, "address", predeploy.Address, "implementation", codeAddr)
			}
		}
		log.Info("are we here tho? ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~`")
		if predeploy.ProxyDisabled && db.Exist(predeploy.Address) {
			db.DeleteState(predeploy.Address, AdminSlot)
		}

		if err := setupPredeploy(db, deployResults, storage, name, predeploy.Address, codeAddr); err != nil {
			log.Info("are we here tho????? ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~`")
			return nil, err
		}
		code := db.GetCode(codeAddr)
		if len(code) == 0 {
			return nil, fmt.Errorf("code not set for %s", name)
		}
	}

	if err := PerformUpgradeTxs(db); err != nil {
		return nil, fmt.Errorf("failed to perform upgrade txs: %w", err)
	}

	return db.Genesis(), nil
}

func PerformUpgradeTxs(db *state.MemoryStateDB) error {
	// Only the Ecotone upgrade is performed with upgrade-txs.
	if !db.Genesis().Config.IsEcotone(db.Genesis().Timestamp) {
		return nil
	}
	sim := squash.NewSimulator(db)
	ecotone, err := derive.EcotoneNetworkUpgradeTransactions()
	if err != nil {
		return fmt.Errorf("failed to build ecotone upgrade txs: %w", err)
	}
	if err := sim.AddUpgradeTxs(ecotone); err != nil {
		return fmt.Errorf("failed to apply ecotone upgrade txs: %w", err)
	}
	return nil
}
