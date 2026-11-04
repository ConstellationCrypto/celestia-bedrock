// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Test } from "forge-std/Test.sol";
import { SafeCall } from "contracts/libraries/SafeCall.sol";
import { SafeCallV172 } from "src/legacy/SafeCallV172.sol";
import { OptimismPortal } from "contracts/L1/OptimismPortal.sol";
import { L2OutputOracle } from "contracts/L1/L2OutputOracle.sol";
import { SystemConfig } from "contracts/L1/SystemConfig.sol";
import { Types } from "contracts/libraries/Types.sol";
import { Hashing } from "contracts/libraries/Hashing.sol";
import { MockOracle } from "test/RelayGuardLocal.t.sol";

contract Reverter {
    fallback() external payable { revert("nope"); }
}

contract SafeCallCaller {
    function callNew(address _t, uint256 _minGas, uint256 _value) external payable returns (bool) {
        return SafeCall.callWithMinGas(_t, _minGas, _value, hex"");
    }
    function callOld(address _t, uint256 _minGas, uint256 _value) external payable returns (bool) {
        return SafeCallV172.callWithMinGas(_t, _minGas, _value, hex"");
    }
}

/// @title RelayGuardSemantics
/// @notice What 1.7.3 must NOT change, and the SafeCall new-account reserve.
contract RelayGuardSemantics is Test {
    bytes32 constant ROOT = keccak256("root");
    uint256 constant PROVEN_WITHDRAWALS_SLOT = 52;

    function _portal() internal returns (OptimismPortal p) {
        vm.warp(10 days);
        p = new OptimismPortal(L2OutputOracle(address(new MockOracle(ROOT))), address(this), false, SystemConfig(address(0)));
        vm.deal(address(p), 10 ether);
    }

    function _prove(OptimismPortal _p, Types.WithdrawalTransaction memory _w) internal returns (bytes32 h) {
        h = Hashing.hashWithdrawal(_w);
        bytes32 s = keccak256(abi.encode(h, PROVEN_WITHDRAWALS_SLOT));
        vm.store(address(_p), s, ROOT);
        vm.store(address(_p), bytes32(uint256(s) + 1), bytes32(uint256(1)));
    }

    /// A withdrawal NOT sent by the L2CrossDomainMessenger whose target reverts is still finalized
    /// as failed, exactly as in 1.7.2.
    function test_directWithdrawal_failure_stillFinalized() external {
        OptimismPortal p = _portal();
        Types.WithdrawalTransaction memory w = Types.WithdrawalTransaction({
            nonce: 1, sender: address(0xA11CE), target: address(new Reverter()), value: 1 ether,
            gasLimit: 100_000, data: hex"1234"
        });
        bytes32 h = _prove(p, w);
        vm.expectEmit(true, false, false, true, address(p));
        emit WithdrawalFinalized(h, false);
        p.finalizeWithdrawalTransaction{ gas: 1_000_000 }(w);
        assertTrue(p.finalizedWithdrawals(h));
    }

    /// The same failing target, but sent by the L2CrossDomainMessenger: 1.7.3 reverts instead.
    function test_messengerWithdrawal_failure_reverts() external {
        OptimismPortal p = _portal();
        Types.WithdrawalTransaction memory w = Types.WithdrawalTransaction({
            nonce: 2, sender: 0x4200000000000000000000000000000000000007, target: address(new Reverter()),
            value: 0, gasLimit: 100_000, data: hex"1234"
        });
        bytes32 h = _prove(p, w);
        vm.expectRevert("OptimismPortal: L1CrossDomainMessenger call failed");
        p.finalizeWithdrawalTransaction{ gas: 1_000_000 }(w);
        assertFalse(p.finalizedWithdrawals(h));
    }

    /// Value to an empty account: 1.7.3's callWithMinGas only proceeds when, after the CALL's
    /// EIP-8037 new-account charge, the callee can still get `minGas` (threshold
    /// minGas * 64/63 + 40,000 + NEW_ACCOUNT_STATE_GAS). 1.7.2 proceeds below that, so under
    /// Amsterdam the callee could get less than `minGas`. To an existing account the threshold
    /// only moves by the EXTCODEHASH lookup.
    function test_callWithMinGas_newAccountReserve() external {
        SafeCallCaller c = new SafeCallCaller();
        vm.deal(address(c), 10 ether);
        uint256 minGas = 50_000;

        uint256 oldLo = _lowestOk(c, true, address(0xE0), minGas);
        uint256 newLo = _lowestOk(c, false, address(0xE1), minGas);
        emit log_named_uint("1.7.2 lowest gas, value to empty account", oldLo);
        emit log_named_uint("1.7.3 lowest gas, value to empty account", newLo);
        uint256 guarantee = (minGas * 64) / 63 + 40_000 + SafeCall.NEW_ACCOUNT_STATE_GAS;
        assertGe(newLo, guarantee, "1.7.3 proceeds below the min-gas guarantee");
        assertLt(oldLo, guarantee, "expected 1.7.2 to proceed below the guarantee");

        vm.deal(address(0xE2), 1); // exists: has balance
        vm.deal(address(0xE3), 1);
        uint256 oldEx = _lowestOk(c, true, address(0xE2), minGas);
        uint256 newEx = _lowestOk(c, false, address(0xE3), minGas);
        assertApproxEqAbs(newEx, oldEx, 5_000, "existing account threshold changed");
    }

    function _lowestOk(SafeCallCaller _c, bool _old, address _t, uint256 _minGas) internal returns (uint256 g) {
        for (g = 50_000; g <= 600_000; g += 250) {
            uint256 snap = vm.snapshotState();
            bool ok;
            if (_old) try _c.callOld{ gas: g }(_t, _minGas, 1 wei) returns (bool r) { ok = r; } catch {}
            else try _c.callNew{ gas: g }(_t, _minGas, 1 wei) returns (bool r) { ok = r; } catch {}
            vm.revertToStateAndDelete(snap);
            if (ok) return g;
        }
        revert("no ok gas");
    }

    event WithdrawalFinalized(bytes32 indexed withdrawalHash, bool success);
}
