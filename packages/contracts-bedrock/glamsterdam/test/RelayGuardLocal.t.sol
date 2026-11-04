// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { console } from "forge-std/Test.sol";
import { RelayGuardBase, TestToken } from "src/RelayGuardBase.sol";
import { OptimismPortalV172 } from "src/legacy/OptimismPortalV172.sol";
import { L1CrossDomainMessengerR150k } from "src/legacy/L1CrossDomainMessengerR150k.sol";
import { OptimismPortal } from "contracts/L1/OptimismPortal.sol";
import { L1CrossDomainMessenger } from "contracts/L1/L1CrossDomainMessenger.sol";
import { L1StandardBridge } from "contracts/L1/L1StandardBridge.sol";
import { L2OutputOracle } from "contracts/L1/L2OutputOracle.sol";
import { SystemConfig } from "contracts/L1/SystemConfig.sol";
import { Proxy } from "contracts/universal/Proxy.sol";
import { Types } from "contracts/libraries/Types.sol";
import { Predeploys } from "contracts/libraries/Predeploys.sol";

/// @notice Stand-in for Manta's L2OutputOracle 1.3.1 (finality-service interface).
contract MockOracle {
    bytes32 public immutable ROOT;
    constructor(bytes32 _root) { ROOT = _root; }
    function startingTimestamp() external pure returns (uint256) { return 0; }
    function stateRootPeriodSeconds(bytes32) external pure returns (uint256) { return 3 days; }
    function getL2Output(uint256) external view returns (Types.OutputProposal memory p) {
        p.outputRoot = ROOT;
        p.timestamp = 1;
        p.l2BlockNumber = 1;
    }
}

/// @title RelayGuardLocal
/// @notice Local (no RPC) gas-limit sweep over portal -> messenger -> bridge -> token, using the
///         sources of this branch. Run with the default evm_version (amsterdam) and with osaka.
///           A: OptimismPortal 1.7.2 (deployed today)       + messenger 1.4.1
///           B: OptimismPortal 1.7.3 (this branch)          + messenger 1.4.1
///           C: OptimismPortal 1.7.2 + messenger with RELAY_RESERVED_GAS = 150k (rejected alternative)
contract RelayGuardLocal is RelayGuardBase {
    bytes32 constant ROOT = keccak256("root");
    uint256 constant AMOUNT = 148e6;

    struct Env { address portal; address messenger; address bridge; TestToken token; }

    function _deploy(bool _patchedPortal, bool _reserve150k) internal returns (Env memory e) {
        vm.warp(10 days);
        MockOracle oracle = new MockOracle(ROOT);
        Proxy mProxy = new Proxy(address(this));
        Proxy bProxy = new Proxy(address(this));
        e.portal = _patchedPortal
            ? address(new OptimismPortal(L2OutputOracle(address(oracle)), address(this), false, SystemConfig(address(0))))
            : address(new OptimismPortalV172(L2OutputOracle(address(oracle)), address(this), false, SystemConfig(address(0))));
        address mImpl = _reserve150k
            ? address(new L1CrossDomainMessengerR150k(OptimismPortal(payable(e.portal))))
            : address(new L1CrossDomainMessenger(OptimismPortal(payable(e.portal))));
        mProxy.upgradeToAndCall(mImpl, abi.encodeWithSignature("initialize()"));
        bProxy.upgradeTo(address(new L1StandardBridge(payable(address(mProxy)))));
        e.messenger = address(mProxy);
        e.bridge = address(bProxy);
        e.token = new TestToken();
        _fundBridge(e.bridge, e.token, 1_000_000e6);
        e.token.mint(address(0xD00D), 1);
    }

    function _erc20(Env memory e, uint256 _burn, uint256 _salt) internal returns (W memory w) {
        e.token.setBurn(_burn);
        w = _buildWithdrawal(e.messenger, e.bridge, address(e.token), AMOUNT, address(uint160(0xC0FFEE + _salt)), _salt);
        _markProven(e.portal, w.wHash, ROOT, 1, 0);
    }

    function _eth(Env memory e, uint256 _salt) internal returns (W memory w) {
        w = _buildWithdrawal(e.messenger, e.bridge, address(0), 0.01 ether, address(uint160(0xE7E0000 + _salt)), _salt);
        vm.deal(e.portal, 1 ether);
        _markProven(e.portal, w.wHash, ROOT, 1, 0);
    }

    /// Sweep token-transfer cost x gas limit for each variant.
    function _matrix(bool _pp, bool _rr) internal returns (uint256 lost, uint256 levelsWithLoss, uint256 lowestOkFailed, uint256 lowestOkLost) {
        for (uint256 burn = 0; burn <= 150_000; burn += 5_000) {
            Env memory e = _deploy(_pp, _rr);
            W memory w = _erc20(e, burn, burn);
            Sweep memory s = _sweep(e.portal, e.messenger, w, 300_000, 900_000, 500);
            lost += s.lost;
            if (s.lost > 0) levelsWithLoss++;
            if (s.lowestOkOutcome == FAILED_REPLAYABLE) lowestOkFailed++;
            if (s.lowestOkOutcome == LOST) lowestOkLost++;
        }
    }

    function _report(string memory _n, bool _pp, bool _rr) internal {
        (uint256 lost, uint256 lv, uint256 lf, uint256 ll) = _matrix(_pp, _rr);
        console.log("----", _n);
        console.log("  (cost, gas) points -> LOST:", lost);
        console.log("  token-cost levels with a LOST window (of 31):", lv);
        console.log("  lowest non-reverting gas -> LOST (levels):", ll);
        console.log("  lowest non-reverting gas -> failed/replayable (levels):", lf);
    }

    function test_matrix_A_current() external { _report("A portal 1.7.2 (current)", false, false); }

    function test_matrix_B_patched() external {
        _report("B portal 1.7.3 (this branch)", true, false);
        (uint256 lost, , uint256 lf, uint256 ll) = _matrix(true, false);
        assertEq(lost, 0, "patched portal lost a message");
        assertEq(ll + lf, 0, "lowest non-reverting gas did not deliver");
    }

    function test_matrix_C_reserve150kOnly() external { _report("C reserve 150k only (rejected)", false, true); }

    function test_eth_newAccount_A_vs_B() external {
        Env memory a = _deploy(false, false);
        _log("ETH to new account, A current", _sweep(a.portal, a.messenger, _eth(a, 1), 300_000, 1_000_000, 250), 250);
        Env memory b = _deploy(true, false);
        Sweep memory s = _sweep(b.portal, b.messenger, _eth(b, 1), 300_000, 1_000_000, 250);
        _log("ETH to new account, B patched", s, 250);
        assertEq(s.lost, 0);
    }

    /// A target that reverts for a non-gas reason is still recorded and replayable with the patch.
    function test_B_nonGasFailureStillReplayable() external {
        Env memory e = _deploy(true, false);
        W memory w = _erc20(e, 0, 7);
        e.token.setBlocked(true);
        OptimismPortal(payable(e.portal)).finalizeWithdrawalTransaction{ gas: 1_000_000 }(w.wtx);
        assertTrue(OptimismPortal(payable(e.portal)).finalizedWithdrawals(w.wHash));
        assertTrue(L1CrossDomainMessenger(e.messenger).failedMessages(w.mHash));
        e.token.setBlocked(false);
        L1CrossDomainMessenger(e.messenger).relayMessage{ gas: 1_000_000 }(
            w.msgNonce, Predeploys.L2_STANDARD_BRIDGE, e.bridge, 0, 0, w.message
        );
        assertEq(e.token.balanceOf(w.recipient), AMOUNT);
    }

    /// Where 1.7.2 loses the message, 1.7.3 reverts; the same withdrawal then finalizes with more gas.
    function test_B_revertsWhereCurrentLoses() external {
        Env memory a = _deploy(false, false);
        W memory wa = _erc20(a, 25_000, 1);
        Env memory b = _deploy(true, false);
        W memory wb = _erc20(b, 25_000, 1);
        uint256 n;
        for (uint256 g = 450_000; g <= 700_000; g += 500) {
            if (_attempt(a.portal, a.messenger, wa, g) != LOST) continue;
            n++;
            assertEq(_attempt(b.portal, b.messenger, wb, g), REVERTED, "patched portal did not revert");
        }
        console.log("gas limits where 1.7.2 loses and 1.7.3 reverts:", n);
        if (block.chainid == 31337 && n == 0) console.log("(no loss window: run with --evm-version amsterdam)");
        OptimismPortal(payable(b.portal)).finalizeWithdrawalTransaction{ gas: 800_000 }(wb.wtx);
        assertEq(b.token.balanceOf(wb.recipient), AMOUNT);
    }

    /// Loss on 1.7.2 is permanent: finalizing again and replaying both revert.
    function test_A_lossIsPermanent() external {
        Env memory e = _deploy(false, false);
        W memory w = _erc20(e, 25_000, 3);
        uint256 g;
        for (g = 400_000; g <= 800_000; g += 500) if (_attempt(e.portal, e.messenger, w, g) == LOST) break;
        if (g > 800_000) { console.log("no loss window on this EVM (expected under osaka)"); return; }
        OptimismPortalV172(payable(e.portal)).finalizeWithdrawalTransaction{ gas: g }(w.wtx);
        vm.expectRevert("OptimismPortal: withdrawal has already been finalized");
        OptimismPortalV172(payable(e.portal)).finalizeWithdrawalTransaction{ gas: 2_000_000 }(w.wtx);
        vm.expectRevert("CrossDomainMessenger: message cannot be replayed");
        L1CrossDomainMessenger(e.messenger).relayMessage{ gas: 2_000_000 }(
            w.msgNonce, Predeploys.L2_STANDARD_BRIDGE, e.bridge, 0, 0, w.message
        );
        assertEq(e.token.balanceOf(w.recipient), 0);
        console.log("lost at gas", g);
    }
}
