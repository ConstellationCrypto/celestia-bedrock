// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Script } from "forge-std/Script.sol";
import { console2 as console } from "forge-std/console2.sol";
import { PortalUpgrade, ISafe } from "helpers/PortalUpgrade.sol";

/// @title UpgradePortal
/// @notice Deploys the Glamsterdam OptimismPortal implementation deterministically and prepares
///         (or, for an EOA-owned ProxyAdmin, executes) `ProxyAdmin.upgrade`.
///
///   Offline (no RPC): recompute the plan and the Safe batch from deploy/<network>.json
///     forge script script/UpgradePortal.s.sol --sig "plan(string)" <network>
///
///   Dry run against the chain (preflight, deploy and upgrade simulated, nothing sent)
///     forge script script/UpgradePortal.s.sol --sig "run(string)" <network> --rpc-url $RPC_URL
///
///   Deploy the implementation (any funded account), then write the upgrade transaction
///     forge script script/UpgradePortal.s.sol --sig "run(string)" <network> --rpc-url $RPC_URL \
///       --broadcast --account <deployer>
///
///   EOA-owned ProxyAdmin (testnets): also send the upgrade from the owner
///     EXECUTE_UPGRADE=true forge script ... --broadcast --account <proxy-admin-owner>
contract UpgradePortal is Script, PortalUpgrade {
    /// @notice Offline: deterministic address, calldata and Safe batch from the config alone.
    function plan(string memory _network) external {
        Cfg memory c = _loadCfg(_network);
        bytes memory initCode = _initCode(c);
        address impl = _create2Address(initCode);
        _writeOutputs(c, impl, initCode, "");
    }

    function run(string memory _network) external {
        Cfg memory c = _loadCfg(_network);
        _preflight(c);
        console.log("preflight ok:", c.network);

        bytes memory initCode = _initCode(c);
        address impl = _create2Address(initCode);

        if (impl.code.length == 0) {
            require(CREATE2_DEPLOYER.code.length > 0, "no CREATE2 deployer on this chain");
            vm.broadcast();
            (bool ok,) = CREATE2_DEPLOYER.call(abi.encodePacked(SALT, initCode));
            require(ok && impl.code.length > 0, "CREATE2 deployment failed");
            console.log("deployed implementation:", impl);
        } else {
            console.log("implementation already deployed:", impl);
        }
        _checkImpl(c, impl);

        // Simulate the exact upgrade as the ProxyAdmin owner and check the result, then roll back.
        uint256 snap = vm.snapshotState();
        _upgradeAsOwnerAndCheck(c, impl);
        vm.revertToStateAndDelete(snap);
        console.log("upgrade simulation ok");

        string memory extra = "";
        if (c.ownerIsSafe) {
            uint256 nonce = ISafe(c.owner).nonce();
            bytes32 h = _safeTxHash(c, _upgradeCalldata(c, impl), nonce);
            extra = string.concat(
                ',\n  "safeNonce": ', vm.toString(nonce), ',\n  "safeTxHash": "', vm.toString(h), '",\n  "atBlock": ',
                vm.toString(block.number)
            );
            console.log("Safe nonce:", nonce);
            console.log("safeTxHash:");
            console.logBytes32(h);
        }
        _writeOutputs(c, impl, initCode, extra);

        if (vm.envOr("EXECUTE_UPGRADE", false)) {
            require(!c.ownerIsSafe, "owner is a Safe: submit deploy/out/<network>.safe-batch.json instead");
            vm.broadcast(c.owner);
            (bool ok, bytes memory ret) = c.proxyAdmin.call(_upgradeCalldata(c, impl));
            require(ok, string(ret));
            console.log("upgrade sent from", c.owner);
        }
    }
}
