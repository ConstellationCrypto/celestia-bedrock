# Deploying the Glamsterdam portal upgrade (Manta Pacific)

One contract changes: the `OptimismPortal` implementation (1.7.2 → 1.7.3). Manta Pacific and
Manta Pacific Sepolia run byte-identical 1.7.2 code (only immutables differ), so both deploy from this
branch. The upgrade is a single `ProxyAdmin.upgrade(portalProxy, newImplementation)`: no
re-initialization, storage layout unchanged, constructor unchanged.

| | Manta Pacific | Manta Pacific Sepolia |
|---|---|---|
| Chain | Ethereum mainnet (1) | Sepolia (11155111) |
| OptimismPortal proxy | `0x9168765EE952de7C6f8fC6FaD5Ec209B960b7622` | `0x80f86c5d3AE8cF84596FF22DB2829F1b7a9Fe83d` |
| Current implementation (1.7.2) | `0x4fEee20712abF5724C2BC0476BD87CBf1F1eE388` | `0xc2692875541dB770A258d5025Aa4AfAA4F8716cd` |
| New implementation (1.7.3, CREATE2) | `0x8d5b2063EAC1bE69CE10AaA66F76AB5AC9601888` | `0x5a95E7A8c50FfbF25cAea0e0175B581EB786AfC5` |
| ProxyAdmin | `0xa2DCa85BB892De55D8B262d1806114733106e8D1` | `0x9F3e00A9eB428D026691aA1a5C8D47B53EFbC46b` |
| ProxyAdmin owner | Safe 1.3.0, 5 of 7, `0x3c46C05c2eba8eDd0B0f0C8B1D2fBf9fc53ea01E` | EOA `0x6A42a60E14EeEE588AD622159971963560D7F570` |
| L1CrossDomainMessenger | `0x635ba609680c55C3bDd0B3627b4c5dB21b13c310` | `0xFe7cF31c4579bb1C578716e04E1Ae16Ac5549fF0` |

Inputs live in `deploy/<network>.json` (read from chain at the block noted there). `deploy/out/` holds
what the script derives from them: `<network>.plan.json` (new implementation address, CREATE2 salt,
init code hash, upgrade calldata) and, for the Safe, `<network>.safe-batch.json` for the Safe
Transaction Builder.

The new implementation is deployed through the deterministic deployment proxy
(`0x4e59b44847b379578588920cA78FbF26c0B4956C`) with salt `keccak256("glamsterdam/portal-relay-guard/v1")`,
so its address is fixed by the source, the compiler settings (solc 0.8.15, optimizer 999999,
bytecodeHash none; solc 0.8.15 always targets london) and the constructor arguments. Anyone can deploy
it; only the ProxyAdmin owner can switch the proxy to it.

## Steps

Needs Foundry 1.8.5 (`foundryup -i 1.8.5`) and Node for the npm dependencies.

```sh
cd packages/contracts-bedrock/glamsterdam
npm install

# 1. Reproduce the plan from the configs: must leave deploy/out unchanged.
npm run plan

# 2. Fork test: preflight against the chain, deploy, upgrade as the owner, check storage/getters.
MAINNET_RPC_URL=... npm run test:fork:mainnet
SEPOLIA_RPC_URL=... npm run test:fork:sepolia

# 3. Dry run (nothing is sent): preflight, deploy and upgrade simulated, Safe nonce and safeTxHash.
forge script script/UpgradePortal.s.sol --sig "run(string)" manta-mainnet --rpc-url $MAINNET_RPC_URL --evm-version osaka

# 4. Deploy the implementation (any funded account).
forge script script/UpgradePortal.s.sol --sig "run(string)" manta-mainnet --rpc-url $MAINNET_RPC_URL \
  --evm-version osaka --broadcast --account <deployer>
```

5. **Manta Pacific (Safe):** load `deploy/out/manta-mainnet.safe-batch.json` in the Safe Transaction
   Builder (or create a transaction to the ProxyAdmin with the `upgradeCalldata` from the plan, value 0,
   operation CALL). Signers check that the Safe UI shows the `safeTxHash` printed by step 3 for the same
   nonce, and that the call is `upgrade(0x9168…7622, 0x8d5b…1888)` on `0xa2DC…8D1`. At Safe nonce 3
   (block 26,341,724) the safeTxHash is
   `0x8979ca0eba420bbf82338b509b6e0a41a13b15790ff1aa2aaf643dabe08cd43a` (computed offline and confirmed
   with the Safe's `getTransactionHash`); it changes if another transaction uses that nonce first.

   **Manta Pacific Sepolia (EOA):** send the upgrade from the owner:

   ```sh
   EXECUTE_UPGRADE=true forge script script/UpgradePortal.s.sol --sig "run(string)" manta-sepolia \
     --rpc-url $SEPOLIA_RPC_URL --evm-version amsterdam --broadcast --account <0x6A42…F570>
   ```

6. Verify:

   ```sh
   cast call 0x9168765EE952de7C6f8fC6FaD5Ec209B960b7622 "version()(string)" --rpc-url $MAINNET_RPC_URL   # 1.7.3
   cast storage 0x9168765EE952de7C6f8fC6FaD5Ec209B960b7622 \
     0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc --rpc-url $MAINNET_RPC_URL     # new impl
   ```

The preflight refuses to run if anything in the config no longer matches the chain (implementation,
version, ProxyAdmin, owner, Safe threshold, immutables, messenger wiring). After an intended change,
update the config, re-run `plan`, and review the diff.

Do the testnet first. The upgrade must be live before the Glamsterdam mainnet fork.
