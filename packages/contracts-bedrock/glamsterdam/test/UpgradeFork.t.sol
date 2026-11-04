// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Test } from "forge-std/Test.sol";
import { PortalUpgrade } from "src/PortalUpgrade.sol";

/// @title UpgradeFork
/// @notice Fork test of the upgrade itself: preflight against the live chain, deploy the new
///         implementation at its CREATE2 address, run `ProxyAdmin.upgrade` as the owner, check
///         storage, getters and wiring are unchanged. Skipped unless FORK_RPC_URL is set.
///
///   NETWORK=manta-mainnet FORK_RPC_URL=$MAINNET_RPC_URL forge test --match-contract UpgradeFork -vv
///   NETWORK=manta-sepolia FORK_RPC_URL=$SEPOLIA_RPC_URL forge test --match-contract UpgradeFork -vv
contract UpgradeFork is Test, PortalUpgrade {
    Cfg internal c;
    bool internal enabled;

    function setUp() public {
        string memory rpc = vm.envOr("FORK_RPC_URL", string(""));
        if (bytes(rpc).length == 0) return;
        vm.createSelectFork(rpc);
        c = _loadCfg(vm.envString("NETWORK"));
        enabled = true;
    }

    function _deployImpl() internal returns (address impl) {
        bytes memory initCode = _initCode(c);
        impl = _create2Address(initCode);
        if (impl.code.length == 0) {
            (bool ok,) = CREATE2_DEPLOYER.call(abi.encodePacked(SALT, initCode));
            require(ok, "CREATE2 deployment failed");
        }
    }

    function test_preflight() external view {
        if (!enabled) return;
        _preflight(c);
    }

    function test_upgrade() external {
        if (!enabled) return;
        _preflight(c);
        address impl = _deployImpl();
        _checkImpl(c, impl);
        _upgradeAsOwnerAndCheck(c, impl);
    }

    /// The committed plans must match what the configs and this tree's compiler output produce
    /// (no RPC needed).
    function test_committedPlansMatchConfigs() external view {
        string[2] memory networks = ["manta-mainnet", "manta-sepolia"];
        for (uint256 i = 0; i < networks.length; i++) {
            Cfg memory n = _loadCfg(networks[i]);
            string memory j = vm.readFile(string.concat(vm.projectRoot(), "/deploy/out/", n.network, ".plan.json"));
            bytes memory initCode = _initCode(n);
            require(
                vm.parseJsonAddress(j, ".newImplementation") == _create2Address(initCode),
                string.concat("deploy/out plan is stale, re-run plan(string): ", n.network)
            );
            require(vm.parseJsonBytes32(j, ".initCodeHash") == keccak256(initCode), "initCodeHash mismatch");
            require(
                keccak256(vm.parseJsonBytes(j, ".upgradeCalldata")) == keccak256(_upgradeCalldata(n, _create2Address(initCode))),
                "upgrade calldata mismatch"
            );
        }
    }
}
