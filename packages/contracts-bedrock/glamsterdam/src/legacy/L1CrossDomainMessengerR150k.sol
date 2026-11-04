// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Predeploys } from "contracts/libraries/Predeploys.sol";
import { OptimismPortal } from "contracts/L1/OptimismPortal.sol";
import { CrossDomainMessengerR150k } from "./CrossDomainMessengerR150k.sol";
import { Semver } from "contracts/universal/Semver.sol";

/// @custom:proxied
/// @title L1CrossDomainMessenger
/// @notice The L1CrossDomainMessenger is a message passing interface between L1 and L2 responsible
///         for sending and receiving data on the L1 side. Users are encouraged to use this
///         interface instead of interacting with lower-level contracts directly.
contract L1CrossDomainMessengerR150k is CrossDomainMessengerR150k, Semver {
    /// @notice Address of the OptimismPortal.
    OptimismPortal public immutable PORTAL;

    /// @custom:semver 1.4.1
    /// @notice Constructs the L1CrossDomainMessenger contract.
    /// @param _portal Address of the OptimismPortal contract on this network.
    constructor(OptimismPortal _portal)
        Semver(1, 4, 1)
        CrossDomainMessengerR150k(Predeploys.L2_CROSS_DOMAIN_MESSENGER)
    {
        PORTAL = _portal;
        initialize();
    }

    /// @notice Initializes the contract.
    function initialize() public initializer {
        __CrossDomainMessenger_init();
    }

    /// @inheritdoc CrossDomainMessengerR150k
    function _sendMessage(
        address _to,
        uint64 _gasLimit,
        uint256 _value,
        bytes memory _data
    ) internal override {
        PORTAL.depositTransaction{ value: _value }(_to, _value, _gasLimit, false, _data);
    }

    /// @inheritdoc CrossDomainMessengerR150k
    function _isOtherMessenger() internal view override returns (bool) {
        return msg.sender == address(PORTAL) && PORTAL.l2Sender() == OTHER_MESSENGER;
    }

    /// @inheritdoc CrossDomainMessengerR150k
    function _isUnsafeTarget(address _target) internal view override returns (bool) {
        return _target == address(this) || _target == address(PORTAL);
    }
}
