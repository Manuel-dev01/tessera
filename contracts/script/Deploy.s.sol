// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

import {Script, console2} from "forge-std/Script.sol";
import {TesseraBond} from "../src/TesseraBond.sol";

/// Deploy TesseraBond to Base.
///   USDC          - defaults to Base mainnet USDC
///   CHALLENGE_WINDOW_SEC - defaults to 3600
/// Run: forge script script/Deploy.s.sol --rpc-url base --broadcast --private-key $ORACLE_PRIVATE_KEY
contract Deploy is Script {
    function run() external returns (TesseraBond bond) {
        address usdc = vm.envOr("USDC", address(0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913));
        uint256 window = vm.envOr("CHALLENGE_WINDOW_SEC", uint256(3600));

        vm.startBroadcast();
        bond = new TesseraBond(usdc, window);
        vm.stopBroadcast();

        console2.log("TesseraBond deployed:", address(bond));
        console2.log("USDC:", usdc);
        console2.log("challengeWindow:", window);
    }
}
