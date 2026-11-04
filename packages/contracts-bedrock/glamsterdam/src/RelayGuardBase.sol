// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Test, console } from "forge-std/Test.sol";
import { ERC20 } from "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import { Types } from "contracts/libraries/Types.sol";
import { Hashing } from "contracts/libraries/Hashing.sol";
import { Encoding } from "contracts/libraries/Encoding.sol";
import { Predeploys } from "contracts/libraries/Predeploys.sol";

/// @notice ERC20 whose transfer burns `burn` extra gas (to model heavier tokens such as USDT or a
///         proxied USDC) and that can be switched to revert (to model a blacklisted recipient).
contract TestToken is ERC20 {
    uint256 public burn;
    bool public blocked;

    constructor() ERC20("Relay Guard Test", "RGT") {}

    function mint(address _to, uint256 _a) external { _mint(_to, _a); }
    function setBurn(uint256 _b) external { burn = _b; }
    function setBlocked(bool _b) external { blocked = _b; }

    function _beforeTokenTransfer(address, address, uint256) internal view override {
        require(!blocked, "TestToken: blocked");
        uint256 b = burn;
        if (b == 0) return;
        uint256 stopAt = gasleft() - b;
        while (gasleft() > stopAt) {}
    }
}

interface IPortalView {
    function finalizeWithdrawalTransaction(Types.WithdrawalTransaction memory) external;
    function finalizedWithdrawals(bytes32) external view returns (bool);
}

interface IMessengerView {
    function successfulMessages(bytes32) external view returns (bool);
    function failedMessages(bytes32) external view returns (bool);
    function baseGas(bytes calldata, uint32) external pure returns (uint64);
    function relayMessage(uint256, address, address, uint256, uint256, bytes calldata) external payable;
}

/// @notice Shared machinery: build a standard-bridge withdrawal (minGasLimit 0, new recipient),
///         finalize it at a given gas limit and classify what happened.
abstract contract RelayGuardBase is Test {
    uint8 internal constant REVERTED = 0;
    uint8 internal constant DELIVERED = 1;
    uint8 internal constant FAILED_REPLAYABLE = 2;
    uint8 internal constant LOST = 3;

    /// @dev provenWithdrawals mapping slot in OptimismPortal 1.7.x (checked with `forge inspect`).
    uint256 internal constant PROVEN_WITHDRAWALS_SLOT = 52;
    /// @dev deposits mapping slot in L1StandardBridge 1.1.x.
    uint256 internal constant BRIDGE_DEPOSITS_SLOT = 2;

    address internal constant L2_TOKEN = address(0xBEEF);

    struct W {
        Types.WithdrawalTransaction wtx;
        bytes32 wHash; // portal withdrawal hash
        bytes32 mHash; // messenger versioned hash (= keccak256(wtx.data))
        address recipient;
        address token; // address(0) for ETH
        uint256 amount;
        uint256 msgNonce;
        bytes message;
    }

    struct Sweep {
        uint256 lost;
        uint256 lostLo;
        uint256 lostHi;
        uint256 failed;
        uint256 delivered;
        uint256 lowestOk;
        uint8 lowestOkOutcome;
    }

    function _outcomeName(uint8 _o) internal pure returns (string memory) {
        if (_o == REVERTED) return "reverted";
        if (_o == DELIVERED) return "delivered";
        if (_o == FAILED_REPLAYABLE) return "failed (replayable)";
        if (_o == LOST) return "LOST";
        return "?";
    }

    /// @notice A standard-bridge withdrawal as L2StandardBridge.withdrawTo(..., minGasLimit = 0, "") emits it.
    function _buildWithdrawal(
        address _messenger,
        address _bridge,
        address _token,
        uint256 _amount,
        address _recipient,
        uint256 _salt
    ) internal view returns (W memory w) {
        w.recipient = _recipient;
        w.token = _token;
        w.amount = _amount;
        // `>> 16` keeps the value below 2**240: Encoding.encodeVersionedNonce ORs the version in
        // assembly and would otherwise see dirty high bits.
        w.msgNonce = Encoding.encodeVersionedNonce(uint240(uint256(keccak256(abi.encode("relay-guard", _salt))) >> 16), 1);
        if (_token == address(0)) {
            w.message = abi.encodeWithSignature(
                "finalizeBridgeETH(address,address,uint256,bytes)", _recipient, _recipient, _amount, bytes("")
            );
        } else {
            w.message = abi.encodeWithSignature(
                "finalizeBridgeERC20(address,address,address,address,uint256,bytes)",
                _token, L2_TOKEN, _recipient, _recipient, _amount, bytes("")
            );
        }
        uint256 value = _token == address(0) ? _amount : 0;
        bytes memory data = abi.encodeWithSignature(
            "relayMessage(uint256,address,address,uint256,uint256,bytes)",
            w.msgNonce, Predeploys.L2_STANDARD_BRIDGE, _bridge, value, uint256(0), w.message
        );
        w.wtx = Types.WithdrawalTransaction({
            nonce: Encoding.encodeVersionedNonce(uint240(uint256(keccak256(abi.encode("relay-guard-w", _salt))) >> 16), 1),
            sender: Predeploys.L2_CROSS_DOMAIN_MESSENGER,
            target: _messenger,
            value: value,
            // what the (unchanged) L2CrossDomainMessenger.baseGas(message, 0) puts in the withdrawal
            gasLimit: IMessengerView(_messenger).baseGas(w.message, 0),
            data: data
        });
        w.wHash = Hashing.hashWithdrawal(w.wtx);
        w.mHash = keccak256(data);
    }

    /// @notice Write a proof for `_wHash` straight into portal storage (local test only).
    function _markProven(address _portal, bytes32 _wHash, bytes32 _outputRoot, uint256 _timestamp, uint256 _index)
        internal
    {
        bytes32 p = keccak256(abi.encode(_wHash, PROVEN_WITHDRAWALS_SLOT));
        vm.store(_portal, p, _outputRoot);
        vm.store(_portal, bytes32(uint256(p) + 1), bytes32((uint256(_index) << 128) | uint128(_timestamp)));
    }

    /// @notice Give the bridge `_amount` of `_token` and a matching deposits[_token][L2_TOKEN] entry.
    function _fundBridge(address _bridge, TestToken _token, uint256 _amount) internal {
        _token.mint(_bridge, _amount);
        bytes32 slot = keccak256(abi.encode(L2_TOKEN, keccak256(abi.encode(address(_token), BRIDGE_DEPOSITS_SLOT))));
        vm.store(_bridge, slot, bytes32(uint256(vm.load(_bridge, slot)) + _amount));
    }

    function _delivered(W memory _w) internal view returns (bool) {
        if (_w.token == address(0)) return _w.recipient.balance == _w.amount;
        return ERC20(_w.token).balanceOf(_w.recipient) == _w.amount;
    }

    /// @notice Finalize at gas `_g` (as the gas the portal frame receives) and classify the result.
    ///         State is rolled back afterwards.
    function _attempt(address _portal, address _messenger, W memory _w, uint256 _g) internal returns (uint8 r) {
        uint256 snap = vm.snapshotState();
        try IPortalView(_portal).finalizeWithdrawalTransaction{ gas: _g }(_w.wtx) {
            if (_delivered(_w)) r = DELIVERED;
            else if (IMessengerView(_messenger).failedMessages(_w.mHash)) r = FAILED_REPLAYABLE;
            else if (
                IPortalView(_portal).finalizedWithdrawals(_w.wHash) && !IMessengerView(_messenger).successfulMessages(_w.mHash)
            ) r = LOST;
            else r = 9;
        } catch {
            r = REVERTED;
        }
        vm.revertToStateAndDelete(snap);
        require(r != 9, "unclassified outcome");
    }

    function _sweep(address _portal, address _messenger, W memory _w, uint256 _lo, uint256 _hi, uint256 _step)
        internal
        returns (Sweep memory s)
    {
        for (uint256 g = _lo; g <= _hi; g += _step) {
            uint8 r = _attempt(_portal, _messenger, _w, g);
            if (r != REVERTED && s.lowestOk == 0) {
                s.lowestOk = g;
                s.lowestOkOutcome = r;
            }
            if (r == LOST) {
                if (s.lost == 0) s.lostLo = g;
                s.lostHi = g;
                s.lost++;
            }
            if (r == FAILED_REPLAYABLE) s.failed++;
            if (r == DELIVERED) s.delivered++;
        }
    }

    function _log(string memory _name, Sweep memory _s, uint256 _step) internal view {
        console.log("----", _name);
        console.log("  lowest non-reverting gas:", _s.lowestOk, "->", _outcomeName(_s.lowestOkOutcome));
        console.log("  gas limits -> LOST:      ", _s.lost);
        if (_s.lost > 0) console.log("    LOST window:", _s.lostLo, "-", _s.lostHi + _step - 1);
        console.log("  gas limits -> failed (replayable):", _s.failed);
        console.log("  gas limits -> delivered:          ", _s.delivered);
    }
}
