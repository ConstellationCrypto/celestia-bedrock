# Deploying the Glamsterdam portal upgrade (Towns testnet)

One contract changes: the `OptimismPortal` implementation (2.7.2 → 2.7.3). The upgrade is a single
`ProxyAdmin.upgrade(portalProxy, newImplementation)`: no re-initialization, storage layout and
constructor unchanged. The FPE token is disabled on this deployment (`L1_FPE_TOKEN = address(0)`,
`FPE_DECIMAL_MULTIPLIER = 1`) and the new implementation keeps those values.

| | Towns testnet |
|---|---|
| Chain | Sepolia (11155111) |
| OptimismPortal proxy | `0x83d872033Cf1e765EE51FF190e34D48A5E5Ec4B5` |
| Current implementation (2.7.2) | `0xdbEf4E03CE7B4509CD877a703673081C0e932d41` |
| New implementation (2.7.3, CREATE2) | `0x4E1061717Ecbc1F134898e8db16D83A3eda9C4F0` |
| ProxyAdmin | `0x71a1498d3Ff0C89d17360F9C0564Be9Cc70A110b` |
| ProxyAdmin owner | EOA `0xc02D845827406B1543c64f8E0D9127ec76850F27` |
| L1CrossDomainMessenger | `0x987d4addf5C9Cc5EfaaAA5c6fC290a70C9fC340C` |

Inputs: `deploy/towns-devnet.json`. Derived: `deploy/out/towns-devnet.plan.json` (new implementation,
CREATE2 salt, init code hash, upgrade calldata). The implementation is deployed through the
deterministic deployment proxy (`0x4e59b44847b379578588920cA78FbF26c0B4956C`) with salt
`keccak256("glamsterdam/portal-relay-guard/v1")`.

## Steps

```sh
cd packages/contracts-bedrock/glamsterdam
npm install
npm run plan                                   # must leave deploy/out unchanged
SEPOLIA_RPC_URL=... npm run test:fork          # preflight + upgrade on a fork

# dry run, then deploy the implementation and send the upgrade from the ProxyAdmin owner
forge script script/UpgradePortal.s.sol --sig "run(string)" towns-devnet --rpc-url $SEPOLIA_RPC_URL --evm-version amsterdam
EXECUTE_UPGRADE=true forge script script/UpgradePortal.s.sol --sig "run(string)" towns-devnet \
  --rpc-url $SEPOLIA_RPC_URL --evm-version amsterdam --broadcast --account <0xc02D…0F27>

cast call 0x83d872033Cf1e765EE51FF190e34D48A5E5Ec4B5 "version()(string)" --rpc-url $SEPOLIA_RPC_URL  # 2.7.3
```

The preflight refuses to run if anything in the config no longer matches the chain (implementation,
version, ProxyAdmin, owner, immutables, messenger wiring).
