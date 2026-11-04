# Glamsterdam relay guard (Towns testnet)

This branch is upstream `eb58b74bd5` with the Towns testnet L1 contracts as deployed on Sepolia
(OptimismPortal 2.7.2 and L1CrossDomainMessenger 1.4.1 with FPE token support, first commit), then
the Glamsterdam patch (OptimismPortal 2.7.3, second commit).

- Deployment (config, script, fork test): [`deploy/README.md`](deploy/README.md).
- Local tests: below.

## Local tests

Gas-limit sweep of `OptimismPortal.finalizeWithdrawalTransaction` -> `L1CrossDomainMessenger.relayMessage`
-> target. The L1StandardBridge in this tree does not compile against the FPE messenger, so the target
is a `Sink` that writes a fresh storage slot like a token transfer to a new holder (and can burn extra
gas or revert). A second case sends ETH to a new account.

| variant | portal |
|---|---|
| A | 2.7.2 as deployed (`src/legacy/OptimismPortalV272.sol` + `SafeCallV172.sol`) |
| B | 2.7.3 (this branch) |

```sh
npm install
npm run test:local          # --evm-version amsterdam (EIP-8037 + EIP-8038)
npm run test:local:osaka    # baseline
```

Expected under amsterdam: A loses the message at the lowest non-reverting gas for most target costs and
for ETH to a new account; B never loses a message and delivers at its lowest non-reverting gas. Under
osaka nothing is lost. Also covered: other withdrawals still finalize as failed, a failed messenger
withdrawal reverts, a non-gas target failure stays replayable, and the `SafeCall` new-account reserve.

Needs Foundry 1.8.5 (`foundryup -i 1.8.5`); solc 0.8.15 is fetched by forge.
