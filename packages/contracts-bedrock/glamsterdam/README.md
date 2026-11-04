# Glamsterdam relay guard

- Deployment (configs, script, Safe batch, fork test): see [`deploy/README.md`](deploy/README.md).
- Local tests: below.

## Local tests

Local (no RPC) gas-limit sweep of
`OptimismPortal.finalizeWithdrawalTransaction` -> `L1CrossDomainMessenger.relayMessage`
-> `L1StandardBridge.finalizeBridge{ERC20,ETH}`, compiled from `../contracts` on this branch.

| variant | portal | messenger |
|---|---|---|
| A | 1.7.2 (deployed today, `src/legacy/OptimismPortalV172.sol` + `SafeCallV172.sol`) | 1.4.1 |
| B | 1.7.3 (this branch: reverts when a withdrawal from the L2CrossDomainMessenger fails; `callWithMinGas` reserves EIP-8037 new-account gas) | 1.4.1 |
| C | 1.7.2 | `RELAY_RESERVED_GAS = 150_000` (alternative, not recommended) |

The oracle is a stand-in for Manta's L2OutputOracle 1.3.1 (finality-service interface); proofs are
written to storage. `TestToken` burns extra gas per transfer to model heavier tokens, and can revert to
model a blocked recipient.

```sh
npm install
npm run test:local          # --evm-version amsterdam (EIP-8037 + EIP-8038)
npm run test:local:osaka    # baseline
```

Needs Foundry 1.8.5 (`foundryup -i 1.8.5`); solc 0.8.15 is fetched by forge.

`RelayGuardSemantics` checks what 1.7.3 must not change (other withdrawals still finalize as failed)
and the `SafeCall` new-account reserve. Both changes mirror upstream op-contracts v9.0.0
(OptimismPortal2 6.0.0, commits 676ffde and ee51245/8b22f40).

Expected under amsterdam: A loses the message at the lowest non-reverting gas for most token costs and
for ETH to a new account; B never loses a message and delivers at its lowest non-reverting gas; a
non-gas target failure under B is still recorded and replayable. Under osaka nothing is lost.

Note: a plain local `anvil --hardfork amsterdam` (1.8.5) did not charge EIP-8037 state gas in our runs;
forge's in-process EVM does, so these are forge tests.
