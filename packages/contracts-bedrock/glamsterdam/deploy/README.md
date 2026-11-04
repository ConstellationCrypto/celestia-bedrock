# Deploying the Glamsterdam portal upgrade (Towns mainnet)

One contract changes: the `OptimismPortal` implementation (2.6.0 → 2.6.1). The upgrade is a single
`ProxyAdmin.upgrade(portalProxy, newImplementation)`: no re-initialization (the proxy keeps its
initialized `l2Oracle`, `systemConfig` and `superchainConfig`), storage layout unchanged, no constructor
arguments.

| | Towns mainnet |
|---|---|
| Chain | Ethereum mainnet (1) |
| OptimismPortal proxy | `0x9fDEEa19836A413C04e9672d3d09f482278e863c` |
| Current implementation (2.6.0) | `0xCEa36be2e9724d88cB107C552c602a8025DB88bA` |
| New implementation (2.6.1, CREATE2) | `0x0e267C5Ead9232B361c69De550a6dd110fa949eA` |
| ProxyAdmin | `0x566c7DB023111D897F16b602B2B57f0F12f7bF44` |
| ProxyAdmin owner | Safe 1.3.0, 4 of 7, `0x2876c43B17A5750CBea5E2A3C42718374E21D5a2` (also the SuperchainConfig guardian) |
| L1CrossDomainMessenger proxy (2.3.0) | `0x22B0cd077c937b9912772B38519b6d2d91541c1A` |
| SuperchainConfig | `0xb6aFBB2A7299e968c9f98f8b518bD89e670a420A` |

Note: `0xCEa36be2…88bA` and `0x71fA82Ea…C99f` are the implementations; the proxies are the portal
and messenger addresses above.

Inputs: `deploy/towns-mainnet.json`. Derived: `deploy/out/towns-mainnet.plan.json` (new implementation,
CREATE2 salt, init code hash, upgrade calldata) and `deploy/out/towns-mainnet.safe-batch.json` (Safe
Transaction Builder). The implementation is deployed through the deterministic deployment proxy
(`0x4e59b44847b379578588920cA78FbF26c0B4956C`) with salt `keccak256("glamsterdam/portal-relay-guard/v1")`;
its address depends only on the source and compiler settings (solc 0.8.15, optimizer 999999,
bytecodeHash none).

## Steps

```sh
cd packages/contracts-bedrock/glamsterdam
npm install && npm run submodules
npm run plan                                   # must leave deploy/out unchanged
MAINNET_RPC_URL=... npm run test:fork          # preflight + upgrade on a fork

# dry run (nothing sent): preflight, deploy and upgrade simulated, Safe nonce and safeTxHash
forge script script/UpgradePortal.s.sol --sig "run(string)" towns-mainnet --rpc-url $MAINNET_RPC_URL --evm-version osaka
# deploy the implementation (any funded account)
forge script script/UpgradePortal.s.sol --sig "run(string)" towns-mainnet --rpc-url $MAINNET_RPC_URL \
  --evm-version osaka --broadcast --account <deployer>
```

Then load `deploy/out/towns-mainnet.safe-batch.json` in the Safe Transaction Builder of
`0x2876…D5a2` (or create a transaction to the ProxyAdmin with the plan's `upgradeCalldata`, value 0,
operation CALL). Signers check the Safe UI shows the `safeTxHash` the dry run printed for the same nonce
and the call `upgrade(0x9fDE…863c, 0x0e26…49eA)` on `0x566c…bF44`. At Safe nonce 6 (block 26,341,724)
the safeTxHash is `0x2719a93d2abf720115dd1dc1db8775883209ec4aab9dc179c5bde346713b2078` (computed
offline and confirmed with the Safe's `getTransactionHash`); it changes if another transaction uses
that nonce first.

Verify:

```sh
cast call 0x9fDEEa19836A413C04e9672d3d09f482278e863c "version()(string)" --rpc-url $MAINNET_RPC_URL  # 2.6.1
```

The preflight refuses to run if anything in the config no longer matches the chain (implementation,
version, ProxyAdmin, owner, Safe threshold, l2Oracle/systemConfig/superchainConfig, messenger wiring).
