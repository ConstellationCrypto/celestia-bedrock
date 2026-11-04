// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Test, console } from "forge-std/Test.sol";
import { OptimismPortal } from "contracts/L1/OptimismPortal.sol";
import { OptimismPortalV272 } from "src/legacy/OptimismPortalV272.sol";
import { L1CrossDomainMessenger } from "contracts/L1/L1CrossDomainMessenger.sol";
import { L2OutputOracle } from "contracts/L1/L2OutputOracle.sol";
import { SystemConfig } from "contracts/L1/SystemConfig.sol";
import { Proxy } from "contracts/universal/Proxy.sol";
import { SafeCall } from "contracts/libraries/SafeCall.sol";
import { SafeCallV172 } from "src/legacy/SafeCallV172.sol";
import { Types } from "contracts/libraries/Types.sol";
import { Hashing } from "contracts/libraries/Hashing.sol";
import { Encoding } from "contracts/libraries/Encoding.sol";
import { Predeploys } from "contracts/libraries/Predeploys.sol";

/// @notice Stand-in for L2OutputOracle 1.3.1 (upstream interface: FINALIZATION_PERIOD_SECONDS).
contract MockOracle {
    bytes32 public immutable ROOT;
    constructor(bytes32 _root) { ROOT = _root; }
    function startingTimestamp() external pure returns (uint256) { return 0; }
    function FINALIZATION_PERIOD_SECONDS() external pure returns (uint256) { return 12; }
    function getL2Output(uint256) external view returns (Types.OutputProposal memory p) {
        p.outputRoot = ROOT;
        p.timestamp = 1;
        p.l2BlockNumber = 1;
    }
}

/// @notice Relay target that behaves like a token transfer to a new holder: writes a fresh slot,
///         optionally burns extra gas (heavier tokens) or reverts (blocked recipient).
contract Sink {
    mapping(address => uint256) public bal;
    uint256 public burn;
    bool public blocked;
    function setBurn(uint256 _b) external { burn = _b; }
    function setBlocked(bool _b) external { blocked = _b; }
    function credit(address _holder) external payable {
        require(!blocked, "Sink: blocked");
        uint256 b = burn;
        if (b > 0) {
            uint256 stopAt = gasleft() - b;
            while (gasleft() > stopAt) {}
        }
        bal[_holder] = 1;
    }
}

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

/// @title RelayGuardLocal (Towns testnet tree: OptimismPortal 2.7.2 -> 2.7.3, FPE messenger 1.4.1)
/// @notice Local gas-limit sweep of portal -> L1CrossDomainMessenger -> target, where the target
///         writes a fresh storage slot like a token transfer to a new holder. The L1StandardBridge
///         in this tree does not compile against the FPE messenger, so a Sink stands in for it.
///           A: OptimismPortal 2.7.2 (deployed today) + SafeCall as deployed
///           B: OptimismPortal 2.7.3 (this branch)
///         Run with --evm-version amsterdam (Glamsterdam) and osaka (baseline).
contract RelayGuardLocal is Test {
    bytes32 constant ROOT = keccak256("root");
    uint256 constant PROVEN_WITHDRAWALS_SLOT = 52;
    address constant L2_SENDER = address(0x2222);
    uint8 constant REVERTED = 0;
    uint8 constant DELIVERED = 1;
    uint8 constant FAILED_REPLAYABLE = 2;
    uint8 constant LOST = 3;

    struct Env { address portal; address messenger; Sink sink; }
    struct W { Types.WithdrawalTransaction wtx; bytes32 wHash; bytes32 mHash; uint256 msgNonce; address target; uint256 value; bytes message; address holder; }

    function _deploy(bool _patched) internal returns (Env memory e) {
        vm.warp(10 days);
        MockOracle oracle = new MockOracle(ROOT);
        Proxy mProxy = new Proxy(address(this));
        e.portal = _patched
            ? address(new OptimismPortal(L2OutputOracle(address(oracle)), address(this), false, SystemConfig(address(0)), address(0), 1))
            : address(new OptimismPortalV272(L2OutputOracle(address(oracle)), address(this), false, SystemConfig(address(0)), address(0), 1));
        mProxy.upgradeToAndCall(
            address(new L1CrossDomainMessenger(OptimismPortal(payable(e.portal)), address(0), 1)),
            abi.encodeWithSignature("initialize()")
        );
        e.messenger = address(mProxy);
        e.sink = new Sink();
        vm.deal(e.portal, 10 ether);
    }

    function _withdrawal(Env memory e, address _target, uint256 _value, bytes memory _message, uint256 _salt)
        internal
        returns (W memory w)
    {
        w.target = _target;
        w.value = _value;
        w.message = _message;
        w.msgNonce = Encoding.encodeVersionedNonce(uint240(uint256(keccak256(abi.encode("m", _salt))) >> 16), 1);
        bytes memory data = abi.encodeWithSignature(
            "relayMessage(uint256,address,address,uint256,uint256,bytes)", w.msgNonce, L2_SENDER, _target, _value, uint256(0), _message
        );
        w.wtx = Types.WithdrawalTransaction({
            nonce: uint256(keccak256(abi.encode("w", _salt))) >> 16,
            sender: Predeploys.L2_CROSS_DOMAIN_MESSENGER,
            target: e.messenger,
            value: _value,
            gasLimit: L1CrossDomainMessenger(e.messenger).baseGas(_message, 0),
            data: data
        });
        w.wHash = Hashing.hashWithdrawal(w.wtx);
        w.mHash = keccak256(data);
        bytes32 s = keccak256(abi.encode(w.wHash, PROVEN_WITHDRAWALS_SLOT));
        vm.store(e.portal, s, ROOT);
        vm.store(e.portal, bytes32(uint256(s) + 1), bytes32(uint256(1)));
    }

    function _sinkWithdrawal(Env memory e, uint256 _salt) internal returns (W memory w) {
        w = _withdrawal(e, address(e.sink), 0, abi.encodeCall(Sink.credit, (address(uint160(0xC0FFEE + _salt)))), _salt);
        w.holder = address(uint160(0xC0FFEE + _salt));
    }

    function _ethWithdrawal(Env memory e, uint256 _salt) internal returns (W memory w) {
        w = _withdrawal(e, address(uint160(0xE7E0000 + _salt)), 0.01 ether, "", _salt);
    }

    function _delivered(Env memory e, W memory w) internal view returns (bool) {
        if (w.holder != address(0)) return e.sink.bal(w.holder) == 1;
        return w.target.balance == w.value;
    }

    function _attempt(Env memory e, W memory w, uint256 _g) internal returns (uint8 r) {
        uint256 snap = vm.snapshotState();
        (bool ok,) = e.portal.call{ gas: _g }(abi.encodeWithSignature(
            "finalizeWithdrawalTransaction((uint256,address,address,uint256,uint256,bytes))", w.wtx));
        if (!ok) r = REVERTED;
        else if (_delivered(e, w)) r = DELIVERED;
        else if (L1CrossDomainMessenger(e.messenger).failedMessages(w.mHash)) r = FAILED_REPLAYABLE;
        else if (OptimismPortal(payable(e.portal)).finalizedWithdrawals(w.wHash)) r = LOST;
        else r = 9;
        vm.revertToStateAndDelete(snap);
        require(r != 9, "unclassified");
    }

    function _sweep(Env memory e, W memory w) internal returns (uint256 lost, uint8 lowestOk) {
        bool found;
        for (uint256 g = 250_000; g <= 900_000; g += 500) {
            uint8 r = _attempt(e, w, g);
            if (r != REVERTED && !found) { found = true; lowestOk = r; }
            if (r == LOST) lost++;
        }
    }

    function _matrix(bool _patched) internal returns (uint256 levelsWithLoss, uint256 lowestLost, uint256 lowestNotDelivered) {
        for (uint256 burn = 0; burn <= 150_000; burn += 10_000) {
            Env memory e = _deploy(_patched);
            e.sink.setBurn(burn);
            (uint256 lost, uint8 lo) = _sweep(e, _sinkWithdrawal(e, burn));
            if (lost > 0) levelsWithLoss++;
            if (lo == LOST) lowestLost++;
            if (lo != DELIVERED) lowestNotDelivered++;
        }
    }

    function test_matrix_A_current() external {
        (uint256 lv, uint256 ll,) = _matrix(false);
        console.log("A 2.7.2: target-cost levels (of 16) with a LOST window:", lv, " LOST at lowest gas:", ll);
    }

    function test_matrix_B_patched() external {
        (uint256 lv, uint256 ll, uint256 nd) = _matrix(true);
        console.log("B 2.7.3: target-cost levels (of 16) with a LOST window:", lv, " not delivered at lowest gas:", nd);
        assertEq(lv + ll, 0, "patched portal lost a message");
        assertEq(nd, 0, "lowest non-reverting gas did not deliver");
    }

    function test_eth_newAccount() external {
        Env memory a = _deploy(false);
        (uint256 lostA,) = _sweep(a, _ethWithdrawal(a, 1));
        Env memory b = _deploy(true);
        (uint256 lostB, uint8 loB) = _sweep(b, _ethWithdrawal(b, 1));
        console.log("ETH to new account: gas limits LOST  A:", lostA, " B:", lostB);
        assertEq(lostB, 0);
        assertEq(loB, DELIVERED);
    }

    function test_B_nonGasFailureStillReplayable() external {
        Env memory e = _deploy(true);
        W memory w = _sinkWithdrawal(e, 7);
        e.sink.setBlocked(true);
        OptimismPortal(payable(e.portal)).finalizeWithdrawalTransaction{ gas: 1_000_000 }(w.wtx);
        assertTrue(OptimismPortal(payable(e.portal)).finalizedWithdrawals(w.wHash));
        assertTrue(L1CrossDomainMessenger(e.messenger).failedMessages(w.mHash));
        e.sink.setBlocked(false);
        L1CrossDomainMessenger(e.messenger).relayMessage{ gas: 1_000_000 }(w.msgNonce, L2_SENDER, w.target, 0, 0, w.message);
        assertEq(e.sink.bal(w.holder), 1);
    }

    /// Where 2.7.2 loses the message, 2.7.3 reverts; the same withdrawal then finalizes with more gas.
    function test_B_revertsWhereCurrentLoses() external {
        uint256 n;
        for (uint256 burn = 0; burn <= 150_000 && n == 0; burn += 10_000) {
            Env memory a = _deploy(false);
            a.sink.setBurn(burn);
            W memory wa = _sinkWithdrawal(a, 1);
            Env memory b = _deploy(true);
            b.sink.setBurn(burn);
            W memory wb = _sinkWithdrawal(b, 1);
            for (uint256 g = 250_000; g <= 900_000; g += 500) {
                if (_attempt(a, wa, g) != LOST) continue;
                n++;
                assertEq(_attempt(b, wb, g), REVERTED, "patched portal did not revert");
            }
            if (n > 0) {
                console.log("target burn", burn, "- gas limits where 2.7.2 loses and 2.7.3 reverts:", n);
                OptimismPortal(payable(b.portal)).finalizeWithdrawalTransaction{ gas: 1_000_000 }(wb.wtx);
                assertEq(b.sink.bal(wb.holder), 1);
            }
        }
        if (n == 0) console.log("no loss window on this EVM (expected before Glamsterdam)");
    }

    function test_directWithdrawal_failure_stillFinalized() external {
        Env memory e = _deploy(true);
        Types.WithdrawalTransaction memory w = Types.WithdrawalTransaction({
            nonce: 1, sender: address(0xA11CE), target: address(new Reverter()), value: 1 ether, gasLimit: 100_000, data: hex"1234"
        });
        bytes32 h = Hashing.hashWithdrawal(w);
        bytes32 s = keccak256(abi.encode(h, PROVEN_WITHDRAWALS_SLOT));
        vm.store(e.portal, s, ROOT);
        vm.store(e.portal, bytes32(uint256(s) + 1), bytes32(uint256(1)));
        OptimismPortal(payable(e.portal)).finalizeWithdrawalTransaction{ gas: 1_000_000 }(w);
        assertTrue(OptimismPortal(payable(e.portal)).finalizedWithdrawals(h));
    }

    function test_messengerWithdrawal_failure_reverts() external {
        Env memory e = _deploy(true);
        Types.WithdrawalTransaction memory w = Types.WithdrawalTransaction({
            nonce: 2, sender: Predeploys.L2_CROSS_DOMAIN_MESSENGER, target: address(new Reverter()), value: 0,
            gasLimit: 100_000, data: hex"1234"
        });
        bytes32 h = Hashing.hashWithdrawal(w);
        bytes32 s = keccak256(abi.encode(h, PROVEN_WITHDRAWALS_SLOT));
        vm.store(e.portal, s, ROOT);
        vm.store(e.portal, bytes32(uint256(s) + 1), bytes32(uint256(1)));
        vm.expectRevert("OptimismPortal: L1CrossDomainMessenger call failed");
        OptimismPortal(payable(e.portal)).finalizeWithdrawalTransaction{ gas: 1_000_000 }(w);
        assertFalse(OptimismPortal(payable(e.portal)).finalizedWithdrawals(h));
    }

    function test_callWithMinGas_newAccountReserve() external {
        SafeCallCaller c = new SafeCallCaller();
        vm.deal(address(c), 10 ether);
        uint256 minGas = 50_000;
        uint256 oldLo = _lowestOk(c, true, address(0xE0), minGas);
        uint256 newLo = _lowestOk(c, false, address(0xE1), minGas);
        uint256 guarantee = (minGas * 64) / 63 + 40_000 + SafeCall.NEW_ACCOUNT_STATE_GAS;
        assertGe(newLo, guarantee);
        assertLt(oldLo, guarantee);
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
}
