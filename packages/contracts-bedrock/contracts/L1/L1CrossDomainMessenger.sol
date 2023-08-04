// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Predeploys } from "../libraries/Predeploys.sol";
import { OptimismPortal } from "./OptimismPortal.sol";
import { CrossDomainMessenger } from "../universal/CrossDomainMessenger.sol";
import { Semver } from "../universal/Semver.sol";
import { IERC20 } from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import { SafeERC20 } from "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";

/// @custom:proxied
/// @title L1CrossDomainMessenger
/// @notice The L1CrossDomainMessenger is a message passing interface between L1 and L2 responsible
///         for sending and receiving data on the L1 side. Users are encouraged to use this
///         interface instead of interacting with lower-level contracts directly.
contract L1CrossDomainMessenger is CrossDomainMessenger, Semver {
    using SafeERC20 for IERC20;

    /// @notice Address of the OptimismPortal.
    OptimismPortal public immutable PORTAL;
    address public immutable L1_FPE_TOKEN;

    /// @custom:semver 1.4.1
    /// @notice Constructs the L1CrossDomainMessenger contract.
    /// @param _portal Address of the OptimismPortal contract on this network.
    constructor(OptimismPortal _portal, address _l1FpeToken, uint256 _fpeDecimalMultiplier)
        Semver(1, 4, 1)
        CrossDomainMessenger(Predeploys.L2_CROSS_DOMAIN_MESSENGER, _fpeDecimalMultiplier)
    {
        PORTAL = _portal;
        L1_FPE_TOKEN = _l1FpeToken;
    }

    /// @notice Initializes the contract.
    function initialize() public initializer {
        __CrossDomainMessenger_init();
        if (L1_FPE_TOKEN != address(0)) {
          IERC20(L1_FPE_TOKEN).safeApprove(address(PORTAL), ~uint256(0));
        }
    }

    /// @inheritdoc CrossDomainMessenger
    function _sendMessage(
        uint64 _gasLimit,
        uint256 _value,
        bytes memory _data
    ) internal override {
        if (L1_FPE_TOKEN != address(0)) {
          require(msg.value == 0, "CrossDomainMessenger: cannot send ETH from L1 with FPE enabled");
          // only perform the ERC20 transfer if necessary
          if (_value > 0) {
            IERC20(L1_FPE_TOKEN).safeTransferFrom(msg.sender, address(this), _value);
          }
        } else {
          require(msg.value == _value, "CrossDomainMessenger: wrong amount of ETH included");
        }
        PORTAL.depositTransaction{ value: msg.value }(OTHER_MESSENGER, _value, _gasLimit, false, _data);
    }

    /// @inheritdoc CrossDomainMessenger
    function _isOtherMessenger() internal view override returns (bool) {
        return msg.sender == address(PORTAL) && PORTAL.l2Sender() == OTHER_MESSENGER;
    }

    /// @inheritdoc CrossDomainMessenger
    function _isUnsafeTarget(address _target) internal view override returns (bool) {
        return _target == address(this) || _target == address(PORTAL);
    }

    function _handleFpeTransfer(address _target, uint256 _value) internal override returns (bool) {
      if (L1_FPE_TOKEN == address(0)) {
        return false;
      }
      if (_isOtherMessenger()) {
        _value /= LOCAL_DECIMAL_MULTIPLIER;
        // only perform the ERC20 transfer if necessary
        if (_value > 0) {
          IERC20(L1_FPE_TOKEN).safeTransfer(_target, _value);
        }
      }
      return true;
    }

    function _fpeGas() internal pure override returns (uint32) {
      return 0;
    }
}
