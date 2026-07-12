// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

import {Test} from "forge-std/Test.sol";
import {TesseraBond} from "../src/TesseraBond.sol";
import {MockUSDC} from "./TesseraBond.t.sol";

/// Exposes the REAL _canonicalBlockHash (no override) so we can prove the on-chain
/// EIP-2935 read works against live Base state. Run with:
///   forge test --match-contract Fork --fork-url https://mainnet.base.org
contract ForkHarness is TesseraBond {
    constructor(address usdc_, uint256 window_) TesseraBond(usdc_, window_) {}

    function canonical(uint256 n) external view returns (bytes32) {
        return _canonicalBlockHash(n);
    }
}

contract TesseraBondForkTest is Test {
    ForkHarness bond;
    MockUSDC usdc;
    address oracle;
    address challenger;

    function setUp() public {
        // Only meaningful on a Base fork (EIP-2935 history contract must have code).
        if (0x0000F90827F1C53a10cb7A02335B175320002935.code.length == 0) return;
        oracle = makeAddr("oracle");
        challenger = makeAddr("challenger");
        usdc = new MockUSDC();
        bond = new ForkHarness(address(usdc), 1 hours);
        usdc.mint(oracle, 100e6);
        vm.prank(oracle);
        usdc.approve(address(bond), type(uint256).max);
        vm.prank(oracle);
        bond.deposit(50e6);
    }

    function testRealEip2935SlashAndNoFraud() public {
        if (address(bond) == address(0)) {
            vm.skip(true);
            return;
        }
        // A block comfortably inside the ~8191-block EIP-2935 window.
        uint256 blk = block.number - 1000;
        bytes32 real = bond.canonical(blk);
        assertTrue(real != bytes32(0), "EIP-2935 read returned 0 on fork");

        // Fraud: oracle anchored a wrong hash for a real block -> challenge slashes.
        bytes32 pidBad = keccak256("bad");
        vm.prank(oracle);
        bond.anchor(pidBad, blk, keccak256("a-lie"), 5e6);
        vm.prank(challenger);
        bond.challenge(pidBad);
        assertEq(usdc.balanceOf(challenger), 5e6, "challenger not paid the slash");

        // Honest: oracle anchored the REAL hash -> challenge reverts.
        bytes32 pidGood = keccak256("good");
        vm.prank(oracle);
        bond.anchor(pidGood, blk, real, 5e6);
        vm.prank(challenger);
        vm.expectRevert("no fraud");
        bond.challenge(pidGood);
    }
}
