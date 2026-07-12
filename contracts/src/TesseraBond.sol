// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

/// @notice Minimal ERC-20 surface Tessera needs (USDC).
interface IERC20 {
    function transfer(address to, uint256 amount) external returns (bool);
    function transferFrom(address from, address to, uint256 amount) external returns (bool);
}

/// @title TesseraBond
/// @notice Honesty stake for the Tessera on-chain verification oracle.
///
/// An oracle keeps a *standing* USDC bond (rolling deposit). Every AVP proof it
/// issues advertises that stake. A specific proof can additionally be *anchored*
/// on-chain, earmarking part of the bond as slashable for a challenge window.
///
/// Slashing is **trustless**: `challenge` reads the block's canonical hash on-chain
/// — from the EIP-2935 history contract (last ~8191 blocks), with the `blockhash`
/// opcode as a fallback for very recent blocks — and slashes the earmark to the
/// challenger iff the anchored `claimedBlockHash` differs from canonical. No oracle,
/// no arbiter, no governance.
///
/// The withdraw/release paths are intentionally hook-free (mirroring ERC-8183's rule
/// that refunds must never be gate-able), CEI-ordered, and reentrancy-guarded.
contract TesseraBond {
    /// @dev Canonical EIP-2935 history-storage contract (serves last 8191 block hashes).
    address internal constant HISTORY = 0x0000F90827F1C53a10cb7A02335B175320002935;

    IERC20 public immutable usdc;
    uint256 public immutable challengeWindow; // seconds a proof stays challengeable after anchor

    /// @notice Free (withdrawable / earmark-able) standing bond per oracle.
    mapping(address => uint256) public free;

    struct Anchor {
        address oracle;
        uint256 blockNumber;
        bytes32 claimedBlockHash;
        uint256 slashAmount;
        uint64 unlockAt;
        bool resolved; // slashed or released
    }

    /// @notice proofId (keccak256 of the canonical AVP) => anchor.
    mapping(bytes32 => Anchor) public anchors;

    event Deposited(address indexed oracle, uint256 amount, uint256 free);
    event Withdrawn(address indexed oracle, uint256 amount, uint256 free);
    event Anchored(bytes32 indexed proofId, address indexed oracle, uint256 blockNumber, bytes32 claimedBlockHash, uint256 slashAmount, uint64 unlockAt);
    event Challenged(bytes32 indexed proofId, address indexed challenger, address indexed oracle, uint256 slashAmount, bytes32 canonicalBlockHash);
    event Released(bytes32 indexed proofId, address indexed oracle, uint256 slashAmount);

    uint256 private _lock = 1;
    modifier nonReentrant() {
        require(_lock == 1, "reentrant");
        _lock = 2;
        _;
        _lock = 1;
    }

    constructor(address usdc_, uint256 challengeWindow_) {
        require(usdc_ != address(0), "usdc=0");
        require(challengeWindow_ > 0, "window=0");
        usdc = IERC20(usdc_);
        challengeWindow = challengeWindow_;
    }

    // --- standing bond ---------------------------------------------------------

    /// @notice Top up the caller's standing bond. Requires prior USDC approval.
    function deposit(uint256 amount) external nonReentrant {
        require(amount > 0, "amount=0");
        _pull(msg.sender, amount);
        free[msg.sender] += amount;
        emit Deposited(msg.sender, amount, free[msg.sender]);
    }

    /// @notice Withdraw free (unearmarked) bond. Hook-free & non-blockable.
    function withdraw(uint256 amount) external nonReentrant {
        require(amount > 0, "amount=0");
        uint256 bal = free[msg.sender];
        require(bal >= amount, "insufficient free");
        free[msg.sender] = bal - amount; // effects before interaction (CEI)
        _push(msg.sender, amount);
        emit Withdrawn(msg.sender, amount, free[msg.sender]);
    }

    // --- per-proof anchor ------------------------------------------------------

    /// @notice Earmark `slashAmount` of the caller's bond against a specific proof
    /// for the challenge window. The proof becomes trustlessly slashable.
    function anchor(bytes32 proofId, uint256 blockNumber, bytes32 claimedBlockHash, uint256 slashAmount) external {
        require(slashAmount > 0, "slash=0");
        require(claimedBlockHash != bytes32(0), "hash=0");
        require(anchors[proofId].oracle == address(0), "anchored");
        uint256 bal = free[msg.sender];
        require(bal >= slashAmount, "insufficient free");
        free[msg.sender] = bal - slashAmount;

        uint64 unlockAt = uint64(block.timestamp + challengeWindow);
        anchors[proofId] = Anchor({
            oracle: msg.sender,
            blockNumber: blockNumber,
            claimedBlockHash: claimedBlockHash,
            slashAmount: slashAmount,
            unlockAt: unlockAt,
            resolved: false
        });
        emit Anchored(proofId, msg.sender, blockNumber, claimedBlockHash, slashAmount, unlockAt);
    }

    /// @notice Permissionlessly slash a fraudulent anchored proof. Trustless: the
    /// canonical block hash is read on-chain and compared to the anchored claim.
    function challenge(bytes32 proofId) external nonReentrant {
        Anchor storage a = anchors[proofId];
        require(a.oracle != address(0), "unknown proof");
        require(!a.resolved, "resolved");
        require(block.timestamp <= a.unlockAt, "window closed");

        bytes32 canonical = _canonicalBlockHash(a.blockNumber);
        require(canonical != bytes32(0), "block out of verifiable window");
        require(canonical != a.claimedBlockHash, "no fraud");

        a.resolved = true; // effects
        uint256 amt = a.slashAmount;
        _push(msg.sender, amt); // interaction
        emit Challenged(proofId, msg.sender, a.oracle, amt, canonical);
    }

    /// @notice After the window closes with no successful challenge, return the
    /// earmark to the oracle's free bond. Hook-free & non-blockable (pure accounting).
    function release(bytes32 proofId) external {
        Anchor storage a = anchors[proofId];
        require(a.oracle != address(0), "unknown proof");
        require(!a.resolved, "resolved");
        require(block.timestamp > a.unlockAt, "window open");

        a.resolved = true;
        free[a.oracle] += a.slashAmount;
        emit Released(proofId, a.oracle, a.slashAmount);
    }

    // --- canonical hash source (overridable in tests) --------------------------

    /// @dev Canonical hash of `blockNumber`, or 0 if not on-chain verifiable.
    /// Recent (<256) via the blockhash opcode; older via EIP-2935 history (~8191).
    function _canonicalBlockHash(uint256 blockNumber) internal view virtual returns (bytes32) {
        if (block.number > blockNumber && block.number - blockNumber < 256) {
            bytes32 bh = blockhash(blockNumber);
            if (bh != bytes32(0)) return bh;
        }
        (bool ok, bytes memory out) = HISTORY.staticcall(abi.encode(blockNumber));
        if (ok && out.length == 32) return abi.decode(out, (bytes32));
        return bytes32(0);
    }

    // --- safe ERC20 (no external deps) -----------------------------------------

    function _pull(address from, uint256 amount) private {
        (bool ok, bytes memory data) =
            address(usdc).call(abi.encodeWithSelector(IERC20.transferFrom.selector, from, address(this), amount));
        require(ok && (data.length == 0 || abi.decode(data, (bool))), "usdc transferFrom failed");
    }

    function _push(address to, uint256 amount) private {
        (bool ok, bytes memory data) =
            address(usdc).call(abi.encodeWithSelector(IERC20.transfer.selector, to, amount));
        require(ok && (data.length == 0 || abi.decode(data, (bool))), "usdc transfer failed");
    }
}
