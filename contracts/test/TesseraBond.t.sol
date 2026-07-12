// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

import {Test} from "forge-std/Test.sol";
import {TesseraBond, IERC20} from "../src/TesseraBond.sol";

/// Minimal 6-decimal mock USDC.
contract MockUSDC {
    string public name = "Mock USDC";
    uint8 public decimals = 6;
    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;

    function mint(address to, uint256 amt) external {
        balanceOf[to] += amt;
    }

    function approve(address s, uint256 a) external returns (bool) {
        allowance[msg.sender][s] = a;
        return true;
    }

    function transfer(address to, uint256 a) public virtual returns (bool) {
        balanceOf[msg.sender] -= a;
        balanceOf[to] += a;
        return true;
    }

    function transferFrom(address f, address t, uint256 a) external returns (bool) {
        uint256 al = allowance[f][msg.sender];
        if (al != type(uint256).max) allowance[f][msg.sender] = al - a;
        balanceOf[f] -= a;
        balanceOf[t] += a;
        return true;
    }
}

/// Harness: canonical hash is injected so tests don't depend on EIP-2935 state.
contract BondHarness is TesseraBond {
    mapping(uint256 => bytes32) public canon;

    constructor(address usdc_, uint256 window_) TesseraBond(usdc_, window_) {}

    function setCanonical(uint256 blockNumber, bytes32 h) external {
        canon[blockNumber] = h;
    }

    function _canonicalBlockHash(uint256 blockNumber) internal view override returns (bytes32) {
        return canon[blockNumber];
    }
}

/// Token that tries to re-enter challenge() on transfer.
contract ReentrantUSDC is MockUSDC {
    TesseraBond public bond;
    bytes32 public target;
    bool public attack;

    function arm(TesseraBond b, bytes32 proofId) external {
        bond = b;
        target = proofId;
        attack = true;
    }

    function transfer(address to, uint256 a) public override returns (bool) {
        if (attack) {
            attack = false;
            // re-entrant slash attempt; must revert inside and not double-pay
            try bond.challenge(target) {} catch {}
        }
        return super.transfer(to, a);
    }
}

contract TesseraBondTest is Test {
    BondHarness bond;
    MockUSDC usdc;

    address oracle;
    address challenger;

    uint256 constant WINDOW = 1 hours;
    bytes32 constant PID = keccak256("proof-1");
    uint256 constant BLK = 48_000_000;
    bytes32 constant REAL = keccak256("real-block-hash");
    bytes32 constant LIE = keccak256("fake-block-hash");

    function setUp() public {
        oracle = makeAddr("oracle");
        challenger = makeAddr("challenger");
        usdc = new MockUSDC();
        bond = new BondHarness(address(usdc), WINDOW);
        usdc.mint(oracle, 100e6);
        vm.prank(oracle);
        usdc.approve(address(bond), type(uint256).max);
    }

    function _deposit(uint256 amt) internal {
        vm.prank(oracle);
        bond.deposit(amt);
    }

    function _anchor(bytes32 pid, bytes32 claimed, uint256 slash) internal {
        vm.prank(oracle);
        bond.anchor(pid, BLK, claimed, slash);
    }

    function testHappyPath() public {
        _deposit(10e6);
        assertEq(bond.free(oracle), 10e6);
        _anchor(PID, REAL, 5e6);
        assertEq(bond.free(oracle), 5e6); // earmarked
        bond.setCanonical(BLK, REAL); // honest proof

        vm.warp(block.timestamp + WINDOW + 1);
        bond.release(PID);
        assertEq(bond.free(oracle), 10e6); // earmark returned

        vm.prank(oracle);
        bond.withdraw(10e6);
        assertEq(usdc.balanceOf(oracle), 100e6);
    }

    function testFalseProofSlashed() public {
        _deposit(10e6);
        _anchor(PID, LIE, 5e6); // oracle anchored a WRONG block hash
        bond.setCanonical(BLK, REAL); // canonical differs -> fraud

        uint256 before = usdc.balanceOf(challenger);
        vm.prank(challenger);
        bond.challenge(PID);

        assertEq(usdc.balanceOf(challenger), before + 5e6); // slashed to challenger
        (,,,,, bool resolved) = bond.anchors(PID);
        assertTrue(resolved);
        assertEq(bond.free(oracle), 5e6); // earmark gone, rest of bond intact
    }

    function testHonestProofCannotBeSlashed() public {
        _deposit(10e6);
        _anchor(PID, REAL, 5e6);
        bond.setCanonical(BLK, REAL);
        vm.prank(challenger);
        vm.expectRevert("no fraud");
        bond.challenge(PID);
    }

    function testOutOfWindowUnverifiable() public {
        _deposit(10e6);
        _anchor(PID, LIE, 5e6);
        // canonical unknown (0) -> cannot prove fraud
        vm.prank(challenger);
        vm.expectRevert("block out of verifiable window");
        bond.challenge(PID);
    }

    function testChallengeAfterWindowReverts() public {
        _deposit(10e6);
        _anchor(PID, LIE, 5e6);
        bond.setCanonical(BLK, REAL);
        vm.warp(block.timestamp + WINDOW + 1);
        vm.prank(challenger);
        vm.expectRevert("window closed");
        bond.challenge(PID);
    }

    function testReleaseBeforeWindowReverts() public {
        _deposit(10e6);
        _anchor(PID, REAL, 5e6);
        vm.expectRevert("window open");
        bond.release(PID);
    }

    function testDoubleSlashPrevented() public {
        _deposit(10e6);
        _anchor(PID, LIE, 5e6);
        bond.setCanonical(BLK, REAL);
        vm.prank(challenger);
        bond.challenge(PID);
        vm.prank(challenger);
        vm.expectRevert("resolved");
        bond.challenge(PID);
    }

    function testReleaseAfterSlashPrevented() public {
        _deposit(10e6);
        _anchor(PID, LIE, 5e6);
        bond.setCanonical(BLK, REAL);
        vm.prank(challenger);
        bond.challenge(PID);
        vm.warp(block.timestamp + WINDOW + 1);
        vm.expectRevert("resolved");
        bond.release(PID);
    }

    function testCannotReanchorSameProofId() public {
        _deposit(10e6);
        _anchor(PID, LIE, 5e6);
        vm.prank(oracle);
        vm.expectRevert("anchored");
        bond.anchor(PID, BLK, REAL, 1e6);
    }

    function testCannotWithdrawEarmarked() public {
        _deposit(10e6);
        _anchor(PID, REAL, 8e6);
        vm.prank(oracle);
        vm.expectRevert("insufficient free");
        bond.withdraw(5e6); // only 2e6 free
    }

    function testReentrancyGuarded() public {
        ReentrantUSDC rtoken = new ReentrantUSDC();
        BondHarness rbond = new BondHarness(address(rtoken), WINDOW);
        rtoken.mint(oracle, 100e6);
        vm.prank(oracle);
        rtoken.approve(address(rbond), type(uint256).max);
        vm.prank(oracle);
        rbond.deposit(10e6);
        vm.prank(oracle);
        rbond.anchor(PID, BLK, LIE, 5e6);
        rbond.setCanonical(BLK, REAL);
        rtoken.arm(rbond, PID);

        // The re-entrant challenge() inside transfer must be blocked by nonReentrant;
        // the outer challenge still succeeds and pays exactly once.
        vm.prank(challenger);
        rbond.challenge(PID);
        assertEq(rtoken.balanceOf(challenger), 5e6); // paid exactly once
        (,,,,, bool resolved) = rbond.anchors(PID);
        assertTrue(resolved);
    }
}
