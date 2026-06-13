// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";

/// @title NeuronEscrow
/// @notice ERC-20 token escrow for the Neuron payment protocol (spec 008, evm-escrow binding).
/// @dev Implements the 6 EscrowAdapter operations: createEscrow, deposit, getBalance,
///      requestRelease, approveRelease/withdraw, claimRefund.
///      Uses pull pattern for fund transfers (FR-P21: "pull via withdraw() on EVM").
contract NeuronEscrow is ReentrancyGuard {
    using SafeERC20 for IERC20;

    // ===================== Types =====================

    enum EscrowState {
        Created,
        Funded,
        Released,
        Refunded
    }

    enum ReleaseState {
        Pending,
        Approved,
        Withdrawn
    }

    struct ReleaseRequest {
        uint256 amount;
        address recipient;
        bytes32 evidenceHash;
        ReleaseState state;
    }

    struct Escrow {
        address buyer;
        address seller;
        address arbiter;
        IERC20 token;
        uint64 threshold;
        bytes32 agreementHash;
        uint64 timeout;
        uint256 balance;
        uint256 pendingReleaseTotal;
        EscrowState state;
        uint256 nextReleaseId;
        mapping(uint256 => ReleaseRequest) releases;
    }

    // ===================== Errors =====================

    error InvalidParticipant();
    error EscrowNotFound(uint256 escrowId);
    error InsufficientBalance(uint256 requested, uint256 available);
    error TimeoutNotElapsed(uint64 current, uint64 timeout);
    error TimeoutElapsed();
    error ReleaseNotFound(uint256 escrowId, uint256 releaseId);
    error ReleaseNotPending(uint256 escrowId, uint256 releaseId);
    error ReleaseNotApproved(uint256 escrowId, uint256 releaseId);
    error NotBuyer(uint256 escrowId, address caller);
    error NotParticipant(uint256 escrowId, address caller);
    error InvalidAmount();

    // ===================== Events =====================

    event EscrowCreated(
        uint256 indexed escrowId,
        address indexed buyer,
        address indexed seller,
        address token,
        bytes32 agreementHash,
        uint64 timeout
    );

    event Deposited(uint256 indexed escrowId, address indexed depositor, uint256 amount, uint256 newBalance);

    event ReleaseRequested(
        uint256 indexed escrowId, uint256 indexed releaseId, uint256 amount, address recipient, bytes32 evidenceHash
    );

    event ReleaseApproved(uint256 indexed escrowId, uint256 indexed releaseId);

    event Withdrawn(uint256 indexed escrowId, uint256 indexed releaseId, address indexed recipient, uint256 amount);

    event RefundClaimed(uint256 indexed escrowId, address indexed buyer, uint256 amount);

    // ===================== State =====================

    uint256 private _nextEscrowId = 1;
    mapping(uint256 => Escrow) private _escrows;

    // ===================== Modifiers =====================

    modifier escrowExists(uint256 escrowId) {
        if (_escrows[escrowId].buyer == address(0)) revert EscrowNotFound(escrowId);
        _;
    }

    // ===================== createEscrow (FR-P17) =====================

    /// @notice Create a new escrow instance.
    /// @param buyer The buyer's address.
    /// @param seller The seller's address.
    /// @param arbiter Optional arbiter (address(0) for none).
    /// @param token The ERC-20 token used for payment.
    /// @param threshold Approval threshold (unused in v1, reserved for multi-sig).
    /// @param agreementHash keccak256 of the accepted service response.
    /// @param timeout Unix timestamp (seconds) after which buyer can claim refund.
    /// @return escrowId The newly created escrow ID.
    function createEscrow(
        address buyer,
        address seller,
        address arbiter,
        address token,
        uint64 threshold,
        bytes32 agreementHash,
        uint64 timeout
    ) external nonReentrant returns (uint256 escrowId) {
        if (buyer == address(0) || seller == address(0)) revert InvalidParticipant();
        if (token == address(0)) revert InvalidParticipant();

        escrowId = _nextEscrowId++;
        Escrow storage e = _escrows[escrowId];
        e.buyer = buyer;
        e.seller = seller;
        e.arbiter = arbiter;
        e.token = IERC20(token);
        e.threshold = threshold;
        e.agreementHash = agreementHash;
        e.timeout = timeout;
        e.state = EscrowState.Created;
        e.nextReleaseId = 1;

        emit EscrowCreated(escrowId, buyer, seller, token, agreementHash, timeout);
    }

    // ===================== deposit (FR-P18) =====================

    /// @notice Deposit tokens into an escrow. Caller must have approved this contract.
    /// @param escrowId The escrow to deposit into.
    /// @param amount The amount to deposit.
    function deposit(uint256 escrowId, uint256 amount) external nonReentrant escrowExists(escrowId) {
        if (amount == 0) revert InvalidAmount();

        Escrow storage e = _escrows[escrowId];
        e.token.safeTransferFrom(msg.sender, address(this), amount);
        e.balance += amount;

        if (e.state == EscrowState.Created) {
            e.state = EscrowState.Funded;
        }

        emit Deposited(escrowId, msg.sender, amount, e.balance);
    }

    // ===================== getBalance (FR-P19) =====================

    /// @notice Get the available balance (total - pending releases).
    /// @param escrowId The escrow to query.
    /// @return available The available balance.
    function getBalance(uint256 escrowId) external view escrowExists(escrowId) returns (uint256 available) {
        Escrow storage e = _escrows[escrowId];
        available = e.balance - e.pendingReleaseTotal;
    }

    // ===================== requestRelease (FR-P20, FR-P25a) =====================

    /// @notice Request release of funds. Only seller or arbiter.
    /// @param escrowId The escrow ID.
    /// @param amount The amount to release.
    /// @param recipient The recipient of funds.
    /// @param evidenceHash keccak256 of delivery proof.
    /// @return releaseId The release request ID.
    function requestRelease(uint256 escrowId, uint256 amount, address recipient, bytes32 evidenceHash)
        external
        nonReentrant
        escrowExists(escrowId)
        returns (uint256 releaseId)
    {
        if (amount == 0) revert InvalidAmount();

        Escrow storage e = _escrows[escrowId];
        if (msg.sender != e.seller && msg.sender != e.arbiter) {
            revert NotParticipant(escrowId, msg.sender);
        }

        uint256 available = e.balance - e.pendingReleaseTotal;
        if (amount > available) revert InsufficientBalance(amount, available);

        releaseId = e.nextReleaseId++;
        e.releases[releaseId] = ReleaseRequest({
            amount: amount,
            recipient: recipient,
            evidenceHash: evidenceHash,
            state: ReleaseState.Pending
        });
        e.pendingReleaseTotal += amount;

        emit ReleaseRequested(escrowId, releaseId, amount, recipient, evidenceHash);
    }

    // ===================== approveRelease (FR-P21) =====================

    /// @notice Approve a pending release request. Only buyer or arbiter.
    /// @param escrowId The escrow ID.
    /// @param releaseId The release request to approve.
    function approveRelease(uint256 escrowId, uint256 releaseId)
        external
        nonReentrant
        escrowExists(escrowId)
    {
        Escrow storage e = _escrows[escrowId];
        if (msg.sender != e.buyer && msg.sender != e.arbiter) {
            revert NotParticipant(escrowId, msg.sender);
        }

        ReleaseRequest storage r = e.releases[releaseId];
        if (r.amount == 0) revert ReleaseNotFound(escrowId, releaseId);
        if (r.state != ReleaseState.Pending) revert ReleaseNotPending(escrowId, releaseId);

        r.state = ReleaseState.Approved;

        emit ReleaseApproved(escrowId, releaseId);
    }

    // ===================== withdraw =====================

    /// @notice Withdraw funds from an approved release. Anyone can call (pull pattern).
    /// @param escrowId The escrow ID.
    /// @param releaseId The approved release to withdraw.
    function withdraw(uint256 escrowId, uint256 releaseId)
        external
        nonReentrant
        escrowExists(escrowId)
    {
        Escrow storage e = _escrows[escrowId];

        ReleaseRequest storage r = e.releases[releaseId];
        if (r.amount == 0) revert ReleaseNotFound(escrowId, releaseId);
        if (r.state != ReleaseState.Approved) revert ReleaseNotApproved(escrowId, releaseId);

        r.state = ReleaseState.Withdrawn;
        uint256 amount = r.amount;
        e.balance -= amount;
        e.pendingReleaseTotal -= amount;

        if (e.balance == 0) {
            e.state = EscrowState.Released;
        }

        e.token.safeTransfer(r.recipient, amount);

        emit Withdrawn(escrowId, releaseId, r.recipient, amount);
    }

    // ===================== claimRefund (FR-P22, FR-P25b) =====================

    /// @notice Refund remaining balance to buyer after timeout.
    /// @param escrowId The escrow ID.
    function claimRefund(uint256 escrowId) external nonReentrant escrowExists(escrowId) {
        Escrow storage e = _escrows[escrowId];
        if (msg.sender != e.buyer) revert NotBuyer(escrowId, msg.sender);
        if (block.timestamp < e.timeout) {
            revert TimeoutNotElapsed(uint64(block.timestamp), e.timeout);
        }

        uint256 refundAmount = e.balance;
        if (refundAmount == 0) revert InvalidAmount();

        e.balance = 0;
        e.pendingReleaseTotal = 0;
        e.state = EscrowState.Refunded;

        e.token.safeTransfer(e.buyer, refundAmount);

        emit RefundClaimed(escrowId, e.buyer, refundAmount);
    }

    // ===================== View Helpers =====================

    /// @notice Get escrow details (excluding release requests).
    function getEscrow(uint256 escrowId)
        external
        view
        escrowExists(escrowId)
        returns (
            address buyer,
            address seller,
            address arbiter,
            address token,
            uint64 threshold,
            bytes32 agreementHash,
            uint64 timeout,
            uint256 balance,
            uint256 pendingReleaseTotal,
            EscrowState state
        )
    {
        Escrow storage e = _escrows[escrowId];
        return (
            e.buyer,
            e.seller,
            e.arbiter,
            address(e.token),
            e.threshold,
            e.agreementHash,
            e.timeout,
            e.balance,
            e.pendingReleaseTotal,
            e.state
        );
    }

    /// @notice Get a specific release request.
    function getRelease(uint256 escrowId, uint256 releaseId)
        external
        view
        escrowExists(escrowId)
        returns (uint256 amount, address recipient, bytes32 evidenceHash, ReleaseState state)
    {
        ReleaseRequest storage r = _escrows[escrowId].releases[releaseId];
        return (r.amount, r.recipient, r.evidenceHash, r.state);
    }
}
