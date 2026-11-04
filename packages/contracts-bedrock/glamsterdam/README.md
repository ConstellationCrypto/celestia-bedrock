# Glamsterdam relay guard (Towns mainnet)

This branch is `op-contracts/v1.4.0-rc.2`, the release Towns mainnet's OptimismPortal 2.6.0 was built
from (every source file of the deployed implementation is identical to this tag, and the bytecode
compiled here matches the chain), plus the Glamsterdam patch (OptimismPortal 2.6.1).

- Deployment (config, script, Safe batch, fork test): [`deploy/README.md`](deploy/README.md).
- Local tests: below.

## Local tests

Gas-limit sweep of `OptimismPortal.finalizeWithdrawalTransaction` -> `L1CrossDomainMessenger.relayMessage`
(2.3.0) -> target, where the target is a `Sink` that writes a fresh storage slot like a token transfer
to a new holder (and can burn extra gas or revert); a second case sends ETH to a new account. The
portal runs behind a proxy, initialized with a stand-in oracle and SuperchainConfig.

| variant | portal |
|---|---|
| A | 2.6.0 as deployed (`helpers/legacy/OptimismPortalV260.sol` + `SafeCallV260.sol`) |
| B | 2.6.1 (this branch) |

```sh
npm install
npm run submodules          # openzeppelin-contracts, openzeppelin-contracts-upgradeable, solmate
npm run test:local          # --evm-version amsterdam (EIP-8037 + EIP-8038)
npm run test:local:osaka    # baseline
```

Expected under amsterdam: A loses the message at the lowest non-reverting gas for most target costs and
for ETH to a new account; B never loses a message and delivers at its lowest non-reverting gas. Under
osaka nothing is lost. Also covered: other withdrawals still finalize as failed, a failed messenger
withdrawal reverts with `L1CDMCallFailed()`, a non-gas target failure stays replayable, and the
`SafeCall` new-account reserve.

The repo's own `test/L1/{OptimismPortal,L1CrossDomainMessenger,L1StandardBridge}.t.sol` give the same
result before and after the patch (67 pass; the suite that needs the `go-ffi` helper needs it built).

Needs Foundry 1.8.5 (`foundryup -i 1.8.5`); solc 0.8.15 is fetched by forge.
