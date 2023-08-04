// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { AddressAliasHelper } from "../vendor/AddressAliasHelper.sol";
import { Predeploys } from "../libraries/Predeploys.sol";
import { CrossDomainMessenger } from "../universal/CrossDomainMessenger.sol";
import { Semver } from "../universal/Semver.sol";
import { L2ToL1MessagePasser } from "./L2ToL1MessagePasser.sol";

/// @custom:proxied
/// @custom:predeploy 0x4200000000000000000000000000000000000007
/// @title L2CrossDomainMessenger
/// @notice The L2CrossDomainMessenger is a high-level interface for message passing between L1 and
///         L2 on the L2 side. Users are generally encouraged to use this contract instead of lower
///         level message passing contracts.
contract L2CrossDomainMessenger is CrossDomainMessenger, Semver {
    /// @custom:semver 1.4.1
    /// @notice Constructs the L2CrossDomainMessenger contract.
    /// @param _l1CrossDomainMessenger Address of the L1CrossDomainMessenger contract.
    constructor(address _l1CrossDomainMessenger)
        Semver(1, 4, 1)
        CrossDomainMessenger(_l1CrossDomainMessenger, 1)
    {
        initialize();
    }

    /// @notice Initializer.
    function initialize() public initializer {
        __CrossDomainMessenger_init();
    }

    /// @custom:legacy
    /// @notice Legacy getter for the remote messenger.
    ///         Use otherMessenger going forward.
    /// @return Address of the L1CrossDomainMessenger contract.
    function l1CrossDomainMessenger() public view returns (address) {
        return OTHER_MESSENGER;
    }

    /// @inheritdoc CrossDomainMessenger
    function _sendMessage(
        uint64 _gasLimit,
        uint256 _value,
        bytes memory _data
    ) internal override {
        require(msg.value == _value, "CrossDomainMessenger: wrong amount of ETH included");
        L2ToL1MessagePasser(payable(Predeploys.L2_TO_L1_MESSAGE_PASSER)).initiateWithdrawal{
            value: _value
        }(OTHER_MESSENGER, _gasLimit, _data);
    }

    /// @inheritdoc CrossDomainMessenger
    function _isOtherMessenger() internal view override returns (bool) {
        return AddressAliasHelper.undoL1ToL2Alias(msg.sender) == OTHER_MESSENGER;
    }

    /// @inheritdoc CrossDomainMessenger
    function _isUnsafeTarget(address _target) internal view override returns (bool) {
        return _target == address(this) || _target == address(Predeploys.L2_TO_L1_MESSAGE_PASSER);
    }

    /// FPE transfer not supported on the L2
    function _handleFpeTransfer(address, uint256) internal pure override returns (bool) {
      return false;
    }

    /// TODO: return 0 if FPE_TOKEN is disabled. low priority, excess gas on the l1 is refunded.
    function _fpeGas() internal pure override returns (uint32) {
      return 200_000;
    }
}
