// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "forge-std/Test.sol";
import "../src/NeuronEscrow.sol";
import "../src/TestToken.sol";

contract NeuronEscrowTest is Test {
    NeuronEscrow public escrow;
    TestToken public token;

    address public buyer;
    address public seller;
    address public arbiter;
    address public outsider;

    bytes32 constant AGREEMENT_HASH = keccak256("test-agreement");
    bytes32 constant EVIDENCE_HASH = keccak256("delivery-proof");
    uint256 constant DEPOSIT_AMOUNT = 1_000_000; // 1M tokens (6 decimals = 1 USDC)
    uint64 constant TIMEOUT = 1_700_000_000; // ~2023 timestamp

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

    function setUp() public {
        buyer = vm.addr(1);
        seller = vm.addr(2);
        arbiter = vm.addr(3);
        outsider = vm.addr(4);

        escrow = new NeuronEscrow();
        token = new TestToken();

        // Mint tokens to buyer
        token.mint(buyer, 10_000_000);
    }

    // ===================== Helper =====================

    function _createAndFund() internal returns (uint256 escrowId) {
        escrowId = escrow.createEscrow(buyer, seller, arbiter, address(token), 1, AGREEMENT_HASH, TIMEOUT);
        vm.startPrank(buyer);
        token.approve(address(escrow), DEPOSIT_AMOUNT);
        escrow.deposit(escrowId, DEPOSIT_AMOUNT);
        vm.stopPrank();
    }

    // ===================== createEscrow =====================

    function test_createEscrow_success() public {
        uint256 escrowId = escrow.createEscrow(buyer, seller, arbiter, address(token), 1, AGREEMENT_HASH, TIMEOUT);
        assertEq(escrowId, 1);

        (
            address b,
            address s,
            address a,
            address t,
            uint64 threshold,
            bytes32 agHash,
            uint64 timeout,
            uint256 balance,
            uint256 pending,
            NeuronEscrow.EscrowState state
        ) = escrow.getEscrow(escrowId);

        assertEq(b, buyer);
        assertEq(s, seller);
        assertEq(a, arbiter);
        assertEq(t, address(token));
        assertEq(threshold, 1);
        assertEq(agHash, AGREEMENT_HASH);
        assertEq(timeout, TIMEOUT);
        assertEq(balance, 0);
        assertEq(pending, 0);
        assertEq(uint8(state), uint8(NeuronEscrow.EscrowState.Created));
    }

    function test_createEscrow_emits_event() public {
        vm.expectEmit(true, true, true, true);
        emit EscrowCreated(1, buyer, seller, address(token), AGREEMENT_HASH, TIMEOUT);
        escrow.createEscrow(buyer, seller, arbiter, address(token), 1, AGREEMENT_HASH, TIMEOUT);
    }

    function test_createEscrow_auto_increments() public {
        uint256 id1 = escrow.createEscrow(buyer, seller, arbiter, address(token), 1, AGREEMENT_HASH, TIMEOUT);
        uint256 id2 = escrow.createEscrow(buyer, seller, arbiter, address(token), 1, AGREEMENT_HASH, TIMEOUT);
        assertEq(id1, 1);
        assertEq(id2, 2);
    }

    function test_createEscrow_reverts_zero_buyer() public {
        vm.expectRevert(NeuronEscrow.InvalidParticipant.selector);
        escrow.createEscrow(address(0), seller, arbiter, address(token), 1, AGREEMENT_HASH, TIMEOUT);
    }

    function test_createEscrow_reverts_zero_token() public {
        vm.expectRevert(NeuronEscrow.InvalidParticipant.selector);
        escrow.createEscrow(buyer, seller, arbiter, address(0), 1, AGREEMENT_HASH, TIMEOUT);
    }

    function test_createEscrow_no_arbiter() public {
        uint256 escrowId =
            escrow.createEscrow(buyer, seller, address(0), address(token), 1, AGREEMENT_HASH, TIMEOUT);
        (, , address a, , , , , , , ) = escrow.getEscrow(escrowId);
        assertEq(a, address(0));
    }

    // ===================== deposit =====================

    function test_deposit_success() public {
        uint256 escrowId =
            escrow.createEscrow(buyer, seller, arbiter, address(token), 1, AGREEMENT_HASH, TIMEOUT);

        vm.startPrank(buyer);
        token.approve(address(escrow), DEPOSIT_AMOUNT);
        escrow.deposit(escrowId, DEPOSIT_AMOUNT);
        vm.stopPrank();

        uint256 available = escrow.getBalance(escrowId);
        assertEq(available, DEPOSIT_AMOUNT);
    }

    function test_deposit_emits_event() public {
        uint256 escrowId =
            escrow.createEscrow(buyer, seller, arbiter, address(token), 1, AGREEMENT_HASH, TIMEOUT);

        vm.startPrank(buyer);
        token.approve(address(escrow), DEPOSIT_AMOUNT);

        vm.expectEmit(true, true, false, true);
        emit Deposited(escrowId, buyer, DEPOSIT_AMOUNT, DEPOSIT_AMOUNT);
        escrow.deposit(escrowId, DEPOSIT_AMOUNT);
        vm.stopPrank();
    }

    function test_deposit_transitions_to_funded() public {
        uint256 escrowId =
            escrow.createEscrow(buyer, seller, arbiter, address(token), 1, AGREEMENT_HASH, TIMEOUT);

        vm.startPrank(buyer);
        token.approve(address(escrow), DEPOSIT_AMOUNT);
        escrow.deposit(escrowId, DEPOSIT_AMOUNT);
        vm.stopPrank();

        (, , , , , , , , , NeuronEscrow.EscrowState state) = escrow.getEscrow(escrowId);
        assertEq(uint8(state), uint8(NeuronEscrow.EscrowState.Funded));
    }

    function test_deposit_reverts_zero_amount() public {
        uint256 escrowId =
            escrow.createEscrow(buyer, seller, arbiter, address(token), 1, AGREEMENT_HASH, TIMEOUT);

        vm.expectRevert(NeuronEscrow.InvalidAmount.selector);
        escrow.deposit(escrowId, 0);
    }

    function test_deposit_reverts_nonexistent() public {
        vm.expectRevert(abi.encodeWithSelector(NeuronEscrow.EscrowNotFound.selector, 999));
        escrow.deposit(999, DEPOSIT_AMOUNT);
    }

    function test_deposit_multiple() public {
        uint256 escrowId =
            escrow.createEscrow(buyer, seller, arbiter, address(token), 1, AGREEMENT_HASH, TIMEOUT);

        vm.startPrank(buyer);
        token.approve(address(escrow), DEPOSIT_AMOUNT * 2);
        escrow.deposit(escrowId, DEPOSIT_AMOUNT);
        escrow.deposit(escrowId, DEPOSIT_AMOUNT);
        vm.stopPrank();

        uint256 available = escrow.getBalance(escrowId);
        assertEq(available, DEPOSIT_AMOUNT * 2);
    }

    // ===================== requestRelease =====================

    function test_requestRelease_by_seller() public {
        uint256 escrowId = _createAndFund();

        vm.prank(seller);
        uint256 releaseId = escrow.requestRelease(escrowId, DEPOSIT_AMOUNT, seller, EVIDENCE_HASH);
        assertEq(releaseId, 1);

        (uint256 amount, address recipient, bytes32 evHash, NeuronEscrow.ReleaseState state) =
            escrow.getRelease(escrowId, releaseId);
        assertEq(amount, DEPOSIT_AMOUNT);
        assertEq(recipient, seller);
        assertEq(evHash, EVIDENCE_HASH);
        assertEq(uint8(state), uint8(NeuronEscrow.ReleaseState.Pending));
    }

    function test_requestRelease_emits_event() public {
        uint256 escrowId = _createAndFund();

        vm.prank(seller);
        vm.expectEmit(true, true, false, true);
        emit ReleaseRequested(escrowId, 1, DEPOSIT_AMOUNT, seller, EVIDENCE_HASH);
        escrow.requestRelease(escrowId, DEPOSIT_AMOUNT, seller, EVIDENCE_HASH);
    }

    function test_requestRelease_reduces_available() public {
        uint256 escrowId = _createAndFund();

        vm.prank(seller);
        escrow.requestRelease(escrowId, DEPOSIT_AMOUNT / 2, seller, EVIDENCE_HASH);

        uint256 available = escrow.getBalance(escrowId);
        assertEq(available, DEPOSIT_AMOUNT / 2);
    }

    function test_requestRelease_reverts_insufficient_balance() public {
        uint256 escrowId = _createAndFund();

        vm.prank(seller);
        vm.expectRevert(
            abi.encodeWithSelector(NeuronEscrow.InsufficientBalance.selector, DEPOSIT_AMOUNT + 1, DEPOSIT_AMOUNT)
        );
        escrow.requestRelease(escrowId, DEPOSIT_AMOUNT + 1, seller, EVIDENCE_HASH);
    }

    function test_requestRelease_reverts_unauthorized() public {
        uint256 escrowId = _createAndFund();

        vm.prank(outsider);
        vm.expectRevert(abi.encodeWithSelector(NeuronEscrow.NotParticipant.selector, escrowId, outsider));
        escrow.requestRelease(escrowId, DEPOSIT_AMOUNT, seller, EVIDENCE_HASH);
    }

    function test_requestRelease_by_arbiter() public {
        uint256 escrowId = _createAndFund();

        vm.prank(arbiter);
        uint256 releaseId = escrow.requestRelease(escrowId, DEPOSIT_AMOUNT, seller, EVIDENCE_HASH);
        assertEq(releaseId, 1);
    }

    // ===================== approveRelease =====================

    function test_approveRelease_by_buyer() public {
        uint256 escrowId = _createAndFund();

        vm.prank(seller);
        uint256 releaseId = escrow.requestRelease(escrowId, DEPOSIT_AMOUNT, seller, EVIDENCE_HASH);

        vm.prank(buyer);
        escrow.approveRelease(escrowId, releaseId);

        (, , , NeuronEscrow.ReleaseState state) = escrow.getRelease(escrowId, releaseId);
        assertEq(uint8(state), uint8(NeuronEscrow.ReleaseState.Approved));
    }

    function test_approveRelease_emits_event() public {
        uint256 escrowId = _createAndFund();

        vm.prank(seller);
        uint256 releaseId = escrow.requestRelease(escrowId, DEPOSIT_AMOUNT, seller, EVIDENCE_HASH);

        vm.prank(buyer);
        vm.expectEmit(true, true, false, false);
        emit ReleaseApproved(escrowId, releaseId);
        escrow.approveRelease(escrowId, releaseId);
    }

    function test_approveRelease_reverts_unauthorized() public {
        uint256 escrowId = _createAndFund();

        vm.prank(seller);
        uint256 releaseId = escrow.requestRelease(escrowId, DEPOSIT_AMOUNT, seller, EVIDENCE_HASH);

        vm.prank(outsider);
        vm.expectRevert(abi.encodeWithSelector(NeuronEscrow.NotParticipant.selector, escrowId, outsider));
        escrow.approveRelease(escrowId, releaseId);
    }

    function test_approveRelease_reverts_nonexistent() public {
        uint256 escrowId = _createAndFund();

        vm.prank(buyer);
        vm.expectRevert(abi.encodeWithSelector(NeuronEscrow.ReleaseNotFound.selector, escrowId, 999));
        escrow.approveRelease(escrowId, 999);
    }

    // ===================== withdraw =====================

    function test_withdraw_success() public {
        uint256 escrowId = _createAndFund();

        vm.prank(seller);
        uint256 releaseId = escrow.requestRelease(escrowId, DEPOSIT_AMOUNT, seller, EVIDENCE_HASH);

        vm.prank(buyer);
        escrow.approveRelease(escrowId, releaseId);

        uint256 sellerBefore = token.balanceOf(seller);
        escrow.withdraw(escrowId, releaseId);
        uint256 sellerAfter = token.balanceOf(seller);

        assertEq(sellerAfter - sellerBefore, DEPOSIT_AMOUNT);
    }

    function test_withdraw_emits_event() public {
        uint256 escrowId = _createAndFund();

        vm.prank(seller);
        uint256 releaseId = escrow.requestRelease(escrowId, DEPOSIT_AMOUNT, seller, EVIDENCE_HASH);

        vm.prank(buyer);
        escrow.approveRelease(escrowId, releaseId);

        vm.expectEmit(true, true, true, true);
        emit Withdrawn(escrowId, releaseId, seller, DEPOSIT_AMOUNT);
        escrow.withdraw(escrowId, releaseId);
    }

    function test_withdraw_transitions_to_released_when_empty() public {
        uint256 escrowId = _createAndFund();

        vm.prank(seller);
        uint256 releaseId = escrow.requestRelease(escrowId, DEPOSIT_AMOUNT, seller, EVIDENCE_HASH);

        vm.prank(buyer);
        escrow.approveRelease(escrowId, releaseId);

        escrow.withdraw(escrowId, releaseId);

        (, , , , , , , , , NeuronEscrow.EscrowState state) = escrow.getEscrow(escrowId);
        assertEq(uint8(state), uint8(NeuronEscrow.EscrowState.Released));
    }

    function test_withdraw_reverts_not_approved() public {
        uint256 escrowId = _createAndFund();

        vm.prank(seller);
        uint256 releaseId = escrow.requestRelease(escrowId, DEPOSIT_AMOUNT, seller, EVIDENCE_HASH);

        // Not approved yet
        vm.expectRevert(abi.encodeWithSelector(NeuronEscrow.ReleaseNotApproved.selector, escrowId, releaseId));
        escrow.withdraw(escrowId, releaseId);
    }

    function test_withdraw_reverts_double() public {
        uint256 escrowId = _createAndFund();

        vm.prank(seller);
        uint256 releaseId = escrow.requestRelease(escrowId, DEPOSIT_AMOUNT, seller, EVIDENCE_HASH);

        vm.prank(buyer);
        escrow.approveRelease(escrowId, releaseId);

        escrow.withdraw(escrowId, releaseId);

        // Second withdrawal should fail
        vm.expectRevert(abi.encodeWithSelector(NeuronEscrow.ReleaseNotApproved.selector, escrowId, releaseId));
        escrow.withdraw(escrowId, releaseId);
    }

    // ===================== claimRefund =====================

    function test_claimRefund_success() public {
        uint256 escrowId = _createAndFund();

        // Advance time past timeout
        vm.warp(TIMEOUT + 1);

        vm.prank(buyer);
        escrow.claimRefund(escrowId);

        uint256 buyerBalance = token.balanceOf(buyer);
        // buyer started with 10M, deposited 1M, got 1M back
        assertEq(buyerBalance, 10_000_000);
    }

    function test_claimRefund_emits_event() public {
        uint256 escrowId = _createAndFund();
        vm.warp(TIMEOUT + 1);

        vm.prank(buyer);
        vm.expectEmit(true, true, false, true);
        emit RefundClaimed(escrowId, buyer, DEPOSIT_AMOUNT);
        escrow.claimRefund(escrowId);
    }

    function test_claimRefund_transitions_to_refunded() public {
        uint256 escrowId = _createAndFund();
        vm.warp(TIMEOUT + 1);

        vm.prank(buyer);
        escrow.claimRefund(escrowId);

        (, , , , , , , , , NeuronEscrow.EscrowState state) = escrow.getEscrow(escrowId);
        assertEq(uint8(state), uint8(NeuronEscrow.EscrowState.Refunded));
    }

    function test_claimRefund_reverts_before_timeout() public {
        uint256 escrowId = _createAndFund();

        vm.warp(TIMEOUT - 1);

        vm.prank(buyer);
        vm.expectRevert(
            abi.encodeWithSelector(NeuronEscrow.TimeoutNotElapsed.selector, uint64(TIMEOUT - 1), TIMEOUT)
        );
        escrow.claimRefund(escrowId);
    }

    function test_claimRefund_reverts_not_buyer() public {
        uint256 escrowId = _createAndFund();
        vm.warp(TIMEOUT + 1);

        vm.prank(seller);
        vm.expectRevert(abi.encodeWithSelector(NeuronEscrow.NotBuyer.selector, escrowId, seller));
        escrow.claimRefund(escrowId);
    }

    // ===================== Full Lifecycle =====================

    function test_full_lifecycle() public {
        // 1. Create escrow
        uint256 escrowId =
            escrow.createEscrow(buyer, seller, arbiter, address(token), 1, AGREEMENT_HASH, TIMEOUT);

        // 2. Deposit
        vm.startPrank(buyer);
        token.approve(address(escrow), DEPOSIT_AMOUNT);
        escrow.deposit(escrowId, DEPOSIT_AMOUNT);
        vm.stopPrank();

        assertEq(escrow.getBalance(escrowId), DEPOSIT_AMOUNT);

        // 3. Seller requests release
        vm.prank(seller);
        uint256 releaseId = escrow.requestRelease(escrowId, DEPOSIT_AMOUNT, seller, EVIDENCE_HASH);

        // Available should be 0 (all pending)
        assertEq(escrow.getBalance(escrowId), 0);

        // 4. Buyer approves
        vm.prank(buyer);
        escrow.approveRelease(escrowId, releaseId);

        // 5. Withdraw
        uint256 sellerBefore = token.balanceOf(seller);
        escrow.withdraw(escrowId, releaseId);
        assertEq(token.balanceOf(seller) - sellerBefore, DEPOSIT_AMOUNT);

        // Escrow balance is 0
        assertEq(escrow.getBalance(escrowId), 0);

        // State is Released
        (, , , , , , , , , NeuronEscrow.EscrowState state) = escrow.getEscrow(escrowId);
        assertEq(uint8(state), uint8(NeuronEscrow.EscrowState.Released));
    }

    function test_partial_release_lifecycle() public {
        uint256 escrowId = _createAndFund();
        uint256 half = DEPOSIT_AMOUNT / 2;

        // Release half
        vm.prank(seller);
        uint256 r1 = escrow.requestRelease(escrowId, half, seller, EVIDENCE_HASH);

        vm.prank(buyer);
        escrow.approveRelease(escrowId, r1);
        escrow.withdraw(escrowId, r1);

        // Half remaining
        assertEq(escrow.getBalance(escrowId), half);

        // Release other half
        vm.prank(seller);
        uint256 r2 = escrow.requestRelease(escrowId, half, seller, EVIDENCE_HASH);

        vm.prank(buyer);
        escrow.approveRelease(escrowId, r2);
        escrow.withdraw(escrowId, r2);

        // All released
        assertEq(escrow.getBalance(escrowId), 0);
        assertEq(token.balanceOf(seller), DEPOSIT_AMOUNT);
    }
}
