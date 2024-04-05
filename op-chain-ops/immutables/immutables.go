package immutables

import (
	"fmt"
	"math/big"
	"reflect"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/abi/bind/backends"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"

	"github.com/ethereum-optimism/optimism/op-bindings/bindings"
	"github.com/ethereum-optimism/optimism/op-chain-ops/deployer"
)

// PredeploysImmutableConfig represents the set of L2 predeploys. It includes all
// L2 predeploys - not just ones with immutable values. This is to be very explicit
// about the configuration of the predeploys. It is important that the inner struct
// fields are in the same order as the constructor arguments in the solidity code.
type PredeploysImmutableConfig struct {
	L2ToL1MessagePasser    struct{}
	DeployerWhitelist      struct{}
	WETH9                  struct{}
	L2CrossDomainMessenger struct{}
	L2StandardBridge       struct{}
	SequencerFeeVault      struct {
		Recipient           common.Address
		MinWithdrawalAmount *big.Int
		WithdrawalNetwork   uint8
	}
	OptimismMintableERC20Factory  struct{}
	L1BlockNumber                 struct{}
	GasPriceOracle                struct{}
	L1Block                       struct{}
	GovernanceToken               struct{}
	LegacyMessagePasser           struct{}
	L2ERC721Bridge                struct{}
	OptimismMintableERC721Factory struct {
		Bridge        common.Address
		RemoteChainId *big.Int
	}
	ProxyAdmin   struct{}
	BaseFeeVault struct {
		Recipient           common.Address
		MinWithdrawalAmount *big.Int
		WithdrawalNetwork   uint8
	}
	L1FeeVault struct {
		Recipient           common.Address
		MinWithdrawalAmount *big.Int
		WithdrawalNetwork   uint8
	}
	SchemaRegistry struct{}
	EAS            struct {
		Name string
	}
	Create2Deployer              struct{}
	MultiCall3                   struct{}
	Safe_v130                    struct{}
	SafeL2_v130                  struct{}
	MultiSendCallOnly_v130       struct{}
	SafeSingletonFactory         struct{}
	DeterministicDeploymentProxy struct{}
	MultiSend_v130               struct{}
	Permit2                      struct{}
	SenderCreator                struct{}
	EntryPoint                   struct{}
	PT1 struct{}
	PT2 struct{}
	PT3 struct{}
	Ecosystem1 struct{}
	Ecosystem2 struct{}
	Ecosystem3 struct{}
	Ecosystem4 struct{}
	Ecosystem5 struct{}
	Ecosystem6 struct{}
	Ecosystem7 struct{}
	Ecosystem8 struct{}
	Ecosystem9 struct{}
	Ecosystem10 struct{}
	Ecosystem11 struct{}
	Ecosystem12 struct{}
	Ecosystem13 struct{}
	Ecosystem14 struct{}
	Ecosystem15 struct{}
	Ecosystem16 struct{}
	Ecosystem17 struct{}
	Ecosystem18 struct{}
	Ecosystem19 struct{}
	Ecosystem20 struct{}
	Ecosystem21 struct{}
	Ecosystem22 struct{}
	Ecosystem23 struct{}
	Ecosystem24 struct{}
	Ecosystem25 struct{}
	Ecosystem26 struct{}
	Ecosystem27 struct{}
	Ecosystem28 struct{}
	Ecosystem29 struct{}
	Ecosystem30 struct{}
	Ecosystem31 struct{}
	Ecosystem32 struct{}
	Ecosystem33 struct{}
	Ecosystem34 struct{}
	Ecosystem35 struct{}
	Ecosystem36 struct{}
	Ecosystem37 struct{}
	Ecosystem38 struct{}
	Ecosystem39 struct{}
	Ecosystem40 struct{}
	Ecosystem41 struct{}
	Ecosystem42 struct{}
	Ecosystem43 struct{}
	Ecosystem44 struct{}
	Ecosystem45 struct{}
	Ecosystem46 struct{}
	Ecosystem47 struct{}
	Ecosystem48 struct{}
	Ecosystem49 struct{}
	Ecosystem50 struct{}
	Ecosystem51 struct{}
	Ecosystem52 struct{}
	Ecosystem53 struct{}
	Ecosystem54 struct{}
	Ecosystem55 struct{}
	Ecosystem56 struct{}
	Ecosystem57 struct{}
	Ecosystem58 struct{}
	Ecosystem59 struct{}
	Ecosystem60 struct{}
	Ecosystem61 struct{}
	Ecosystem62 struct{}
	Ecosystem63 struct{}
	Ecosystem64 struct{}
	Ecosystem65 struct{}
	Ecosystem66 struct{}
	Ecosystem67 struct{}
	Ecosystem68 struct{}
	Ecosystem69 struct{}
	Ecosystem70 struct{}
	Ecosystem71 struct{}
	Ecosystem72 struct{}
	Ecosystem73 struct{}
	Ecosystem74 struct{}
	Ecosystem75 struct{}
	Ecosystem76 struct{}
	Ecosystem77 struct{}
	Ecosystem78 struct{}
	Ecosystem79 struct{}
	Ecosystem80 struct{}
	Ecosystem81 struct{}
	Ecosystem82 struct{}
	Ecosystem83 struct{}
	Ecosystem84 struct{}
	Ecosystem85 struct{}
	Ecosystem86 struct{}
	Ecosystem87 struct{}
	Ecosystem88 struct{}
	Ecosystem89 struct{}
	Ecosystem90 struct{}
	Ecosystem91 struct{}
	Ecosystem92 struct{}
	Ecosystem93 struct{}
	Ecosystem94 struct{}
	Ecosystem95 struct{}
	Ecosystem96 struct{}
	Ecosystem97 struct{}
	Ecosystem98 struct{}
	Ecosystem99 struct{}
	Ecosystem100 struct{}
	Ecosystem101 struct{}
	Ecosystem102 struct{}
	Ecosystem103 struct{}
	Ecosystem104 struct{}
	Ecosystem105 struct{}
	Ecosystem106 struct{}
	Ecosystem107 struct{}
	Ecosystem108 struct{}
	Ecosystem109 struct{}
	Ecosystem110 struct{}
	Ecosystem111 struct{}
	Ecosystem112 struct{}
	Ecosystem113 struct{}
	Ecosystem114 struct{}
	Ecosystem115 struct{}
	Ecosystem116 struct{}
	Ecosystem117 struct{}
	Ecosystem118 struct{}
	Ecosystem119 struct{}
	Ecosystem120 struct{}
	Ecosystem121 struct{}
	Ecosystem122 struct{}
	Ecosystem123 struct{}
	Ecosystem124 struct{}
	Ecosystem125 struct{}
	Ecosystem126 struct{}
	Ecosystem127 struct{}
	Ecosystem128 struct{}
	Ecosystem129 struct{}
	Ecosystem130 struct{}
	Ecosystem131 struct{}
	Ecosystem132 struct{}
	Ecosystem133 struct{}
	Ecosystem134 struct{}
	Ecosystem135 struct{}
	Ecosystem136 struct{}
	Ecosystem137 struct{}
	Ecosystem138 struct{}
	Ecosystem139 struct{}
	Ecosystem140 struct{}
	Ecosystem141 struct{}
	Ecosystem142 struct{}
	Ecosystem143 struct{}
	Ecosystem144 struct{}
	Ecosystem145 struct{}
	Ecosystem146 struct{}
	Ecosystem147 struct{}
	Ecosystem148 struct{}
	Ecosystem149 struct{}
	Ecosystem150 struct{}
	Ecosystem151 struct{}
	Ecosystem152 struct{}
	Ecosystem153 struct{}
	Ecosystem154 struct{}
	Ecosystem155 struct{}
	Ecosystem156 struct{}
	Ecosystem157 struct{}
	Ecosystem158 struct{}
	Ecosystem159 struct{}
	Ecosystem160 struct{}
	Ecosystem161 struct{}
	Ecosystem162 struct{}
	Ecosystem163 struct{}
	Ecosystem164 struct{}
	Ecosystem165 struct{}
	Ecosystem166 struct{}
	Ecosystem167 struct{}
	Ecosystem168 struct{}
	Ecosystem169 struct{}
	Ecosystem170 struct{}
	Ecosystem171 struct{}
	Ecosystem172 struct{}
	Ecosystem173 struct{}
	Ecosystem174 struct{}
	Ecosystem175 struct{}
	Ecosystem176 struct{}
	Ecosystem177 struct{}
	Ecosystem178 struct{}
	Ecosystem179 struct{}
	Ecosystem180 struct{}
	Ecosystem181 struct{}
	Ecosystem182 struct{}
	Ecosystem183 struct{}
	Ecosystem184 struct{}
	Ecosystem185 struct{}
	Ecosystem186 struct{}
	Ecosystem187 struct{}
	Ecosystem188 struct{}
	Ecosystem189 struct{}
	Ecosystem190 struct{}
	Ecosystem191 struct{}
	Ecosystem192 struct{}
	Ecosystem193 struct{}
	Ecosystem194 struct{}
	Ecosystem195 struct{}
	Ecosystem196 struct{}
	Ecosystem197 struct{}
	Ecosystem198 struct{}
	Ecosystem199 struct{}
	Ecosystem200 struct{}
	Ecosystem201 struct{}
	Ecosystem202 struct{}
	Ecosystem203 struct{}
	Ecosystem204 struct{}
	Ecosystem205 struct{}
	Ecosystem206 struct{}
	Ecosystem207 struct{}
	Ecosystem208 struct{}
	Ecosystem209 struct{}
	Ecosystem210 struct{}
	Ecosystem211 struct{}
	Ecosystem212 struct{}
	Ecosystem213 struct{}
	Ecosystem214 struct{}
	Ecosystem215 struct{}
	Ecosystem216 struct{}
	Ecosystem217 struct{}
	Ecosystem218 struct{}
	Ecosystem219 struct{}
	Ecosystem220 struct{}
	Ecosystem221 struct{}
	Ecosystem222 struct{}
	Ecosystem223 struct{}
	Ecosystem224 struct{}
	Ecosystem225 struct{}
	Ecosystem226 struct{}
	Ecosystem227 struct{}
	Ecosystem228 struct{}
	Ecosystem229 struct{}
	Ecosystem230 struct{}
	Ecosystem231 struct{}
	Ecosystem232 struct{}
	Ecosystem233 struct{}
	Ecosystem234 struct{}
	Ecosystem235 struct{}
	Ecosystem236 struct{}
	Ecosystem237 struct{}
	Ecosystem238 struct{}
	Ecosystem239 struct{}
	Ecosystem240 struct{}
	Ecosystem241 struct{}
	Ecosystem242 struct{}
	Ecosystem243 struct{}
	Ecosystem244 struct{}
	Ecosystem245 struct{}
	Ecosystem246 struct{}
	Ecosystem247 struct{}
	Ecosystem248 struct{}
	Ecosystem249 struct{}
	Ecosystem250 struct{}
	Ecosystem251 struct{}
	Ecosystem252 struct{}
	Ecosystem253 struct{}
	Ecosystem254 struct{}
	Ecosystem255 struct{}
	IP1 struct{}
	IP2 struct{}
	IP3 struct{}
	IP4 struct{}
	IP5 struct{}
	IP6 struct{}
	IP7 struct{}
	IP8 struct{}
	IP9 struct{}
	IP10 struct{}
	IP11 struct{}
	IP12 struct{}
	IP13 struct{}
	IP14 struct{}
	IP15 struct{}
	IP16 struct{}
	IP17 struct{}
	IP18 struct{}
	IP19 struct{}
	IP20 struct{}
	IP21 struct{}
	IP22 struct{}
	IP23 struct{}
	IP24 struct{}
	IP25 struct{}
	IP26 struct{}
	IP27 struct{}
	IP28 struct{}
	IP29 struct{}
	IP30 struct{}
	IP31 struct{}
	IP32 struct{}
	IP33 struct{}
	IP34 struct{}
	IP35 struct{}
	IP36 struct{}
	IP37 struct{}
	IP38 struct{}
	IP39 struct{}
	IP40 struct{}
	IP41 struct{}
	IP42 struct{}
	IP43 struct{}
	IP44 struct{}
	IP45 struct{}
	IP46 struct{}
	IP47 struct{}
	IP48 struct{}
	IP49 struct{}
	IP50 struct{}
	IP51 struct{}
	IP52 struct{}
	IP53 struct{}
	IP54 struct{}
	IP55 struct{}
	IP56 struct{}
	IP57 struct{}
	IP58 struct{}
	IP59 struct{}
	IP60 struct{}
	IP61 struct{}
	IP62 struct{}
	IP63 struct{}
	IP64 struct{}
	IP65 struct{}
	IP66 struct{}
	IP67 struct{}
	IP68 struct{}
	IP69 struct{}
	IP70 struct{}
	IP71 struct{}
	IP72 struct{}
	IP73 struct{}
	IP74 struct{}
	IP75 struct{}
	IP76 struct{}
	IP77 struct{}
	IP78 struct{}
	IP79 struct{}
	IP80 struct{}
	IP81 struct{}
	IP82 struct{}
	IP83 struct{}
	IP84 struct{}
	IP85 struct{}
	IP86 struct{}
	IP87 struct{}
	IP88 struct{}
	IP89 struct{}
	IP90 struct{}
	IP91 struct{}
	IP92 struct{}
	IP93 struct{}
	IP94 struct{}
	IP95 struct{}
	IP96 struct{}
	IP97 struct{}
	IP98 struct{}
	IP99 struct{}
	IP100 struct{}
	IP101 struct{}
	IP102 struct{}
	IP103 struct{}
	IP104 struct{}
	IP105 struct{}
	IP106 struct{}
	IP107 struct{}
	IP108 struct{}
	IP109 struct{}
	IP110 struct{}
	IP111 struct{}
	IP112 struct{}
	IP113 struct{}
	IP114 struct{}
	IP115 struct{}
	IP116 struct{}
	IP117 struct{}
	IP118 struct{}
	IP119 struct{}
	IP120 struct{}
	IP121 struct{}
	IP122 struct{}
	IP123 struct{}
	IP124 struct{}
	IP125 struct{}
	IP126 struct{}
	IP127 struct{}
	IP128 struct{}
	IP129 struct{}
	IP130 struct{}
	IP131 struct{}
	IP132 struct{}
	IP133 struct{}
	IP134 struct{}
	IP135 struct{}
	IP136 struct{}
	IP137 struct{}
	IP138 struct{}
	IP139 struct{}
	IP140 struct{}
	IP141 struct{}
	IP142 struct{}
	IP143 struct{}
	IP144 struct{}
	IP145 struct{}
	IP146 struct{}
	IP147 struct{}
	IP148 struct{}
	IP149 struct{}
	IP150 struct{}
	IP151 struct{}
	IP152 struct{}
	IP153 struct{}
	IP154 struct{}
	IP155 struct{}
	IP156 struct{}
	IP157 struct{}
	IP158 struct{}
	IP159 struct{}
	IP160 struct{}
	IP161 struct{}
	IP162 struct{}
	IP163 struct{}
	IP164 struct{}
	IP165 struct{}
	IP166 struct{}
	IP167 struct{}
	IP168 struct{}
	IP169 struct{}
	IP170 struct{}
	IP171 struct{}
	IP172 struct{}
	IP173 struct{}
	IP174 struct{}
	IP175 struct{}
	IP176 struct{}
	IP177 struct{}
	IP178 struct{}
	IP179 struct{}
	IP180 struct{}
	IP181 struct{}
	IP182 struct{}
	IP183 struct{}
	IP184 struct{}
	IP185 struct{}
	IP186 struct{}
	IP187 struct{}
	IP188 struct{}
	IP189 struct{}
	IP190 struct{}
	IP191 struct{}
	IP192 struct{}
	IP193 struct{}
	IP194 struct{}
	IP195 struct{}
	IP196 struct{}
	IP197 struct{}
	IP198 struct{}
	IP199 struct{}
	IP200 struct{}
	IP201 struct{}
	IP202 struct{}
	IP203 struct{}
	IP204 struct{}
	IP205 struct{}
	IP206 struct{}
	IP207 struct{}
	IP208 struct{}
	IP209 struct{}
	IP210 struct{}
	IP211 struct{}
	IP212 struct{}
	IP213 struct{}
	IP214 struct{}
	IP215 struct{}
	IP216 struct{}
	IP217 struct{}
	IP218 struct{}
	IP219 struct{}
	IP220 struct{}
	IP221 struct{}
	IP222 struct{}
	IP223 struct{}
	IP224 struct{}
	IP225 struct{}
	IP226 struct{}
	IP227 struct{}
	IP228 struct{}
	IP229 struct{}
	IP230 struct{}
	IP231 struct{}
	IP232 struct{}
	IP233 struct{}
	IP234 struct{}
	IP235 struct{}
	IP236 struct{}
	IP237 struct{}
	IP238 struct{}
	IP239 struct{}
	IP240 struct{}
	IP241 struct{}
	IP242 struct{}
	IP243 struct{}
	IP244 struct{}
	IP245 struct{}
	IP246 struct{}
	IP247 struct{}
	IP248 struct{}
	IP249 struct{}
	IP250 struct{}
	IP251 struct{}
	IP252 struct{}
	IP253 struct{}
	IP254 struct{}
	IP255 struct{}
}

// Check will ensure that the required fields are set on the config.
// An error returned by `GetImmutableReferences` means that the solc compiler
// output for the contract has no immutables in it.
func (c *PredeploysImmutableConfig) Check() error {
	return c.ForEach(func(name string, values any) error {
		val := reflect.ValueOf(values)
		if val.NumField() == 0 {
			return nil
		}

		has, err := bindings.HasImmutableReferences(name)
		exists := err == nil && has
		isZero := val.IsZero()

		// There are immutables defined in the solc output and
		// the config is not empty.
		if exists && !isZero {
			return nil
		}
		// There are no immutables defined in the solc output and
		// the config is empty
		if !exists && isZero {
			return nil
		}

		return fmt.Errorf("invalid immutables config: field %s: %w", name, err)
	})
}

// ForEach will iterate over each of the fields in the config and call the callback
// with the value of the field as well as the field's name.
func (c *PredeploysImmutableConfig) ForEach(cb func(string, any) error) error {
	val := reflect.ValueOf(c).Elem()
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		internalVal := reflect.ValueOf(field.Interface())
		if err := cb(typ.Field(i).Name, internalVal.Interface()); err != nil {
			return err
		}
	}
	return nil
}

// DeploymentResults represents the output of deploying each of the
// contracts so that the immutables can be set properly in the bytecode.
type DeploymentResults map[string]hexutil.Bytes

// Deploy will deploy L2 predeploys that include immutables. This is to prevent the need
// for parsing the solc output to find the correct immutable offsets and splicing in the values.
// Skip any predeploys that do not have immutables as their bytecode will be directly inserted
// into the state. This does not currently support recursive structs.
func Deploy(config *PredeploysImmutableConfig) (DeploymentResults, error) {
	if err := config.Check(); err != nil {
		return DeploymentResults{}, err
	}
	deployments := make([]deployer.Constructor, 0)

	val := reflect.ValueOf(config).Elem()
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		if reflect.ValueOf(field.Interface()).IsZero() {
			continue
		}

		deployment := deployer.Constructor{
			Name: typ.Field(i).Name,
			Args: []any{},
		}

		internalVal := reflect.ValueOf(field.Interface())
		for j := 0; j < internalVal.NumField(); j++ {
			internalField := internalVal.Field(j)
			deployment.Args = append(deployment.Args, internalField.Interface())
		}

		deployments = append(deployments, deployment)
	}

	results, err := deployContractsWithImmutables(deployments)
	if err != nil {
		return nil, fmt.Errorf("cannot deploy contracts with immutables: %w", err)
	}
	return results, nil
}

// deployContractsWithImmutables will deploy contracts to a simulated backend so that their immutables
// can be properly set. The bytecode returned in the results is suitable to be
// inserted into the state via state surgery.
func deployContractsWithImmutables(constructors []deployer.Constructor) (DeploymentResults, error) {
	backend, err := deployer.NewL2Backend()
	if err != nil {
		return nil, err
	}
	deployments, err := deployer.Deploy(backend, constructors, l2ImmutableDeployer)
	if err != nil {
		return nil, err
	}
	results := make(DeploymentResults)
	for _, dep := range deployments {
		results[dep.Name] = dep.Bytecode
	}
	return results, nil
}

// l2ImmutableDeployer will deploy L2 predeploys that contain immutables to the simulated backend.
// It only needs to care about the predeploys that have immutables so that the deployed bytecode
// has the dynamic value set at the correct location in the bytecode.
func l2ImmutableDeployer(backend *backends.SimulatedBackend, opts *bind.TransactOpts, deployment deployer.Constructor) (*types.Transaction, error) {
	var tx *types.Transaction
	var recipient common.Address
	var minimumWithdrawalAmount *big.Int
	var withdrawalNetwork uint8
	var err error

	if has, err := bindings.HasImmutableReferences(deployment.Name); err != nil || !has {
		return nil, fmt.Errorf("%s does not have immutables: %w", deployment.Name, err)
	}

	switch deployment.Name {
	case "L2CrossDomainMessenger":
		_, tx, _, err = bindings.DeployL2CrossDomainMessenger(opts, backend)
	case "L2StandardBridge":
		_, tx, _, err = bindings.DeployL2StandardBridge(opts, backend)
	case "SequencerFeeVault":
		recipient, minimumWithdrawalAmount, withdrawalNetwork, err = prepareFeeVaultArguments(deployment)
		if err != nil {
			return nil, err
		}
		_, tx, _, err = bindings.DeploySequencerFeeVault(opts, backend, recipient, minimumWithdrawalAmount, withdrawalNetwork)
	case "BaseFeeVault":
		recipient, minimumWithdrawalAmount, withdrawalNetwork, err = prepareFeeVaultArguments(deployment)
		if err != nil {
			return nil, err
		}
		_, tx, _, err = bindings.DeployBaseFeeVault(opts, backend, recipient, minimumWithdrawalAmount, withdrawalNetwork)
	case "L1FeeVault":
		recipient, minimumWithdrawalAmount, withdrawalNetwork, err = prepareFeeVaultArguments(deployment)
		if err != nil {
			return nil, err
		}
		_, tx, _, err = bindings.DeployL1FeeVault(opts, backend, recipient, minimumWithdrawalAmount, withdrawalNetwork)
	case "OptimismMintableERC20Factory":
		_, tx, _, err = bindings.DeployOptimismMintableERC20Factory(opts, backend)
	case "L2ERC721Bridge":
		_, tx, _, err = bindings.DeployL2ERC721Bridge(opts, backend)
	case "OptimismMintableERC721Factory":
		bridge, ok := deployment.Args[0].(common.Address)
		if !ok {
			return nil, fmt.Errorf("invalid type for bridge")
		}
		remoteChainId, ok := deployment.Args[1].(*big.Int)
		if !ok {
			return nil, fmt.Errorf("invalid type for remoteChainId")
		}
		_, tx, _, err = bindings.DeployOptimismMintableERC721Factory(opts, backend, bridge, remoteChainId)
	case "EAS":
		_, tx, _, err = bindings.DeployEAS(opts, backend)
	default:
		return tx, fmt.Errorf("unknown contract: %s", deployment.Name)
	}

	return tx, err
}

// prepareFeeVaultArguments is a helper function that parses the arguments for the fee vault contracts.
func prepareFeeVaultArguments(deployment deployer.Constructor) (common.Address, *big.Int, uint8, error) {
	recipient, ok := deployment.Args[0].(common.Address)
	if !ok {
		return common.Address{}, nil, 0, fmt.Errorf("invalid type for recipient")
	}
	minimumWithdrawalAmountHex, ok := deployment.Args[1].(*big.Int)
	if !ok {
		return common.Address{}, nil, 0, fmt.Errorf("invalid type for minimumWithdrawalAmount")
	}
	withdrawalNetwork, ok := deployment.Args[2].(uint8)
	if !ok {
		return common.Address{}, nil, 0, fmt.Errorf("invalid type for withdrawalNetwork")
	}
	return recipient, minimumWithdrawalAmountHex, withdrawalNetwork, nil
}
