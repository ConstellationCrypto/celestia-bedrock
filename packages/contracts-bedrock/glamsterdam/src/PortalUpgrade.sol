// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { CommonBase } from "forge-std/Base.sol";
import { stdJson } from "forge-std/StdJson.sol";
import { console2 as console } from "forge-std/console2.sol";
import { OptimismPortal } from "contracts/L1/OptimismPortal.sol";

interface IProxyAdmin {
    function owner() external view returns (address);
    function proxyType(address) external view returns (uint8);
    function upgrade(address payable _proxy, address _implementation) external;
}

interface ISafe {
    function getThreshold() external view returns (uint256);
    function nonce() external view returns (uint256);
    function getTransactionHash(
        address to,
        uint256 value,
        bytes calldata data,
        uint8 operation,
        uint256 safeTxGas,
        uint256 baseGas,
        uint256 gasPrice,
        address gasToken,
        address refundReceiver,
        uint256 _nonce
    ) external view returns (bytes32);
}

interface IVersioned {
    function version() external view returns (string memory);
}

interface IL1MessengerV1 {
    function PORTAL() external view returns (address);
}

/// @title PortalUpgrade
/// @notice Shared logic for the Glamsterdam OptimismPortal upgrade: reads a network config from
///         `deploy/<network>.json`, checks it against the chain, derives the new implementation's
///         deterministic (CREATE2) address, and simulates `ProxyAdmin.upgrade` with post-checks.
///         Used by `script/UpgradePortal.s.sol` and `test/UpgradeFork.t.sol`.
abstract contract PortalUpgrade is CommonBase {
    using stdJson for string;

    /// @notice Arachnid deterministic deployment proxy (present on mainnet and Sepolia).
    address internal constant CREATE2_DEPLOYER = 0x4e59b44847b379578588920cA78FbF26c0B4956C;

    /// @notice CREATE2 salt for the new implementation.
    bytes32 internal constant SALT = keccak256("glamsterdam/portal-relay-guard/v1");

    /// @notice EIP-1967 slots.
    bytes32 internal constant IMPL_SLOT = 0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc;
    bytes32 internal constant ADMIN_SLOT = 0xb53127684a568b3173ae13b9f8a6016e243e63b6e8ee1178d6a717850b5d6103;

    /// @notice Portal storage slots compared before and after the upgrade (layout ends at slot 53).
    uint256 internal constant CHECKED_SLOTS = 64;

    struct Cfg {
        string network;
        uint256 chainId;
        address portalProxy;
        address proxyAdmin;
        address owner;
        bool ownerIsSafe;
        uint256 safeThreshold;
        address currentImpl;
        string currentVersion;
        string newVersion;
        address l1Messenger;
        address l2Oracle;
        address guardian;
        address systemConfig;
        string[] getters;
    }

    function _cfgPath(string memory _network) internal view returns (string memory) {
        return string.concat(vm.projectRoot(), "/deploy/", _network, ".json");
    }

    function _loadCfg(string memory _network) internal view returns (Cfg memory c) {
        string memory j = vm.readFile(_cfgPath(_network));
        c.network = j.readString(".network");
        c.chainId = j.readUint(".chainId");
        c.portalProxy = j.readAddress(".portalProxy");
        c.proxyAdmin = j.readAddress(".proxyAdmin");
        c.owner = j.readAddress(".proxyAdminOwner");
        c.ownerIsSafe = j.readBool(".ownerIsSafe");
        c.safeThreshold = j.readUint(".safeThreshold");
        c.currentImpl = j.readAddress(".currentImplementation");
        c.currentVersion = j.readString(".currentVersion");
        c.newVersion = j.readString(".newVersion");
        c.l1Messenger = j.readAddress(".l1CrossDomainMessenger");
        c.l2Oracle = j.readAddress(".constructor.l2Oracle");
        c.guardian = j.readAddress(".constructor.guardian");
        c.systemConfig = j.readAddress(".constructor.systemConfig");
        c.getters = j.readStringArray(".getters");
    }

    /// @notice Creation code + constructor args of the new implementation. The implementation is
    ///         deployed paused (it is never used directly; the proxy keeps its own `paused`).
    function _initCode(Cfg memory _c) internal pure virtual returns (bytes memory) {
        return abi.encodePacked(
            type(OptimismPortal).creationCode, abi.encode(_c.l2Oracle, _c.guardian, true, _c.systemConfig)
        );
    }

    function _create2Address(bytes memory _initCode) internal pure returns (address) {
        return address(
            uint160(uint256(keccak256(abi.encodePacked(bytes1(0xff), CREATE2_DEPLOYER, SALT, keccak256(_initCode)))))
        );
    }

    function _upgradeCalldata(Cfg memory _c, address _impl) internal pure returns (bytes memory) {
        return abi.encodeCall(IProxyAdmin.upgrade, (payable(_c.portalProxy), _impl));
    }

    function _eq(string memory _a, string memory _b) internal pure returns (bool) {
        return keccak256(bytes(_a)) == keccak256(bytes(_b));
    }

    function _slotAddr(address _a, bytes32 _slot) internal view returns (address) {
        return address(uint160(uint256(vm.load(_a, _slot))));
    }

    function _get(address _a, string memory _sig) internal view returns (bytes memory r) {
        bool ok;
        (ok, r) = _a.staticcall(abi.encodeWithSignature(_sig));
        require(ok, string.concat("getter reverted: ", _sig));
    }

    /// @notice Everything the config claims about the live system must hold before we touch it.
    function _preflight(Cfg memory _c) internal view virtual {
        require(block.chainid == _c.chainId, "preflight: wrong chain");
        require(_c.portalProxy.code.length > 0, "preflight: no portal proxy code");
        require(_slotAddr(_c.portalProxy, IMPL_SLOT) == _c.currentImpl, "preflight: portal implementation changed");
        require(_slotAddr(_c.portalProxy, ADMIN_SLOT) == _c.proxyAdmin, "preflight: portal admin is not ProxyAdmin");
        require(IProxyAdmin(_c.proxyAdmin).owner() == _c.owner, "preflight: ProxyAdmin owner changed");
        require(IProxyAdmin(_c.proxyAdmin).proxyType(_c.portalProxy) == 0, "preflight: portal is not an ERC1967 proxy");
        require(_eq(IVersioned(_c.portalProxy).version(), _c.currentVersion), "preflight: unexpected portal version");
        require(IL1MessengerV1(_c.l1Messenger).PORTAL() == _c.portalProxy, "preflight: messenger/portal mismatch");
        _preflightImmutables(_c, _c.portalProxy);
        if (_c.ownerIsSafe) {
            require(_c.owner.code.length > 0, "preflight: owner Safe has no code");
            require(ISafe(_c.owner).getThreshold() == _c.safeThreshold, "preflight: Safe threshold changed");
        }
    }

    /// @notice The new implementation's immutables must equal the live ones.
    function _preflightImmutables(Cfg memory _c, address _portal) internal view virtual {
        OptimismPortal p = OptimismPortal(payable(_portal));
        require(address(p.L2_ORACLE()) == _c.l2Oracle, "immutables: L2_ORACLE");
        require(p.GUARDIAN() == _c.guardian, "immutables: GUARDIAN");
        require(address(p.SYSTEM_CONFIG()) == _c.systemConfig, "immutables: SYSTEM_CONFIG");
    }

    function _checkImpl(Cfg memory _c, address _impl) internal view {
        require(_impl.code.length > 0, "impl: not deployed");
        require(_eq(IVersioned(_impl).version(), _c.newVersion), "impl: wrong version");
        _preflightImmutables(_c, _impl);
    }

    struct Snapshot {
        bytes32[] slots;
        bytes[] getters;
    }

    function _snapshot(Cfg memory _c) internal view returns (Snapshot memory s) {
        s.slots = new bytes32[](CHECKED_SLOTS);
        for (uint256 i = 0; i < CHECKED_SLOTS; i++) s.slots[i] = vm.load(_c.portalProxy, bytes32(i));
        s.getters = new bytes[](_c.getters.length);
        for (uint256 i = 0; i < _c.getters.length; i++) s.getters[i] = _get(_c.portalProxy, _c.getters[i]);
    }

    /// @notice Post-upgrade invariants: new implementation in place, admin unchanged, storage and
    ///         every listed getter unchanged, messenger still wired to this portal.
    function _postChecks(Cfg memory _c, address _impl, Snapshot memory _before) internal view {
        require(_slotAddr(_c.portalProxy, IMPL_SLOT) == _impl, "post: implementation not set");
        require(_slotAddr(_c.portalProxy, ADMIN_SLOT) == _c.proxyAdmin, "post: admin changed");
        require(_eq(IVersioned(_c.portalProxy).version(), _c.newVersion), "post: wrong version");
        Snapshot memory a = _snapshot(_c);
        for (uint256 i = 0; i < CHECKED_SLOTS; i++) {
            require(a.slots[i] == _before.slots[i], string.concat("post: storage slot changed: ", vm.toString(i)));
        }
        for (uint256 i = 0; i < _c.getters.length; i++) {
            require(keccak256(a.getters[i]) == keccak256(_before.getters[i]), string.concat("post: getter changed: ", _c.getters[i]));
        }
        require(IL1MessengerV1(_c.l1Messenger).PORTAL() == _c.portalProxy, "post: messenger/portal mismatch");
    }

    /// @notice Run the exact upgrade call as the ProxyAdmin owner and check the result. Callers
    ///         wrap this in a state snapshot when it is only a simulation.
    function _upgradeAsOwnerAndCheck(Cfg memory _c, address _impl) internal {
        Snapshot memory before = _snapshot(_c);
        vm.prank(_c.owner);
        (bool ok, bytes memory ret) = _c.proxyAdmin.call(_upgradeCalldata(_c, _impl));
        require(ok, string(ret));
        _postChecks(_c, _impl, before);
    }

    function _safeTxHash(Cfg memory _c, bytes memory _data, uint256 _nonce) internal view returns (bytes32) {
        return ISafe(_c.owner).getTransactionHash(_c.proxyAdmin, 0, _data, 0, 0, 0, 0, address(0), address(0), _nonce);
    }

    /// @notice Writes the upgrade transaction: Safe Transaction Builder batch when the owner is a
    ///         Safe, plus a plan file with everything needed to review it.
    function _writeOutputs(Cfg memory _c, address _impl, bytes memory _initCode, string memory _extra) internal {
        bytes memory data = _upgradeCalldata(_c, _impl);
        string memory base = string.concat(vm.projectRoot(), "/deploy/out/", _c.network);
        vm.writeFile(string.concat(base, ".plan.json"), _planJson(_c, _impl, _initCode, data, _extra));
        if (_c.ownerIsSafe) vm.writeFile(string.concat(base, ".safe-batch.json"), _safeBatchJson(_c, _impl, data));
        console.log("new implementation:", _impl);
        console.log("upgrade: call", _c.proxyAdmin);
        console.log("calldata:");
        console.logBytes(data);
    }

    function _kv(string memory _k, string memory _v) internal pure returns (string memory) {
        return string.concat('  "', _k, '": "', _v, '",\n');
    }

    function _planJson(Cfg memory _c, address _impl, bytes memory _initCode, bytes memory _data, string memory _extra)
        internal
        view
        returns (string memory s)
    {
        s = string.concat("{\n", _kv("network", _c.network), '  "chainId": ', vm.toString(_c.chainId), ",\n");
        s = string.concat(s, _kv("portalProxy", vm.toString(_c.portalProxy)), _kv("proxyAdmin", vm.toString(_c.proxyAdmin)));
        s = string.concat(s, _kv("proxyAdminOwner", vm.toString(_c.owner)), _kv("currentImplementation", vm.toString(_c.currentImpl)));
        s = string.concat(s, _kv("currentVersion", _c.currentVersion), _kv("newVersion", _c.newVersion));
        s = string.concat(s, _kv("newImplementation", vm.toString(_impl)), _kv("create2Deployer", vm.toString(CREATE2_DEPLOYER)));
        s = string.concat(s, _kv("create2Salt", vm.toString(SALT)), _kv("initCodeHash", vm.toString(keccak256(_initCode))));
        s = string.concat(s, _kv("upgradeTo", vm.toString(_c.proxyAdmin)));
        s = string.concat(s, '  "upgradeCalldata": "', vm.toString(_data), '"', _extra, "\n}\n");
    }

    function _safeBatchJson(Cfg memory _c, address _impl, bytes memory _data) internal view returns (string memory s) {
        s = string.concat('{\n  "version": "1.0",\n  "chainId": "', vm.toString(_c.chainId), '",\n  "createdAt": 0,\n');
        s = string.concat(s, '  "meta": {\n    "name": "Glamsterdam: upgrade OptimismPortal to ', _c.newVersion, " (", _c.network, ')",\n');
        s = string.concat(
            s,
            '    "description": "ProxyAdmin.upgrade(',
            vm.toString(_c.portalProxy),
            ", ",
            vm.toString(_impl),
            "). Finalization now reverts when an L2CrossDomainMessenger withdrawal fails, so it can be retried with more gas.\",\n"
        );
        s = string.concat(s, '    "txBuilderVersion": "1.18.0",\n    "createdFromSafeAddress": "', vm.toString(_c.owner), '",\n');
        s = string.concat(s, '    "createdFromOwnerAddress": ""\n  },\n  "transactions": [\n    {\n');
        s = string.concat(s, '      "to": "', vm.toString(_c.proxyAdmin), '",\n      "value": "0",\n      "data": "', vm.toString(_data), '",\n');
        s = string.concat(
            s,
            '      "contractMethod": {\n        "inputs": [\n',
            '          { "internalType": "address payable", "name": "_proxy", "type": "address" },\n',
            '          { "internalType": "address", "name": "_implementation", "type": "address" }\n',
            '        ],\n        "name": "upgrade",\n        "payable": false\n      },\n'
        );
        s = string.concat(
            s,
            '      "contractInputsValues": {\n        "_proxy": "',
            vm.toString(_c.portalProxy),
            '",\n        "_implementation": "',
            vm.toString(_impl),
            '"\n      }\n    }\n  ]\n}\n'
        );
    }
}
