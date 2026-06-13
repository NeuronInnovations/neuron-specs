// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "forge-std/Test.sol";
import "../src/NeuronIdentityRegistry.sol";

contract NeuronIdentityRegistryTest is Test {
    NeuronIdentityRegistry public registry;
    address public admin;
    address public agent1;
    address public agent2;
    address public operator;

    string constant URI_1 = '{"services":[{"type":"neuron-topic","name":"stdOut"}]}';
    string constant URI_2 = '{"services":[{"type":"neuron-topic","name":"stdOut"},{"type":"neuron-p2p-exchange"}]}';
    string constant URI_UPDATED = '{"services":[{"type":"neuron-topic","name":"stdOut","updated":true}]}';

    // ERC-8004 events
    event Registered(uint256 indexed agentId, string agentURI, address indexed owner);
    event URIUpdated(uint256 indexed agentId, string newURI, address indexed updatedBy);
    // Project extension events
    event IdentityRevoked(uint256 indexed agentId, address indexed owner);

    function setUp() public {
        admin = address(this);
        agent1 = vm.addr(1);
        agent2 = vm.addr(2);
        operator = vm.addr(3);

        registry = new NeuronIdentityRegistry();
    }

    // ===================== Register =====================

    function test_register_success() public {
        vm.prank(agent1);
        uint256 agentId = registry.register(URI_1);

        assertEq(agentId, 1, "First agent should be 1");
        assertEq(registry.ownerOf(agentId), agent1);
        assertEq(registry.agentURI(agentId), URI_1);
    }

    function test_register_emits_erc8004_event() public {
        vm.prank(agent1);
        vm.expectEmit(true, true, false, true);
        emit Registered(1, URI_1, agent1);
        registry.register(URI_1);
    }

    function test_register_auto_increments_agentId() public {
        vm.prank(agent1);
        uint256 id1 = registry.register(URI_1);

        vm.prank(agent2);
        uint256 id2 = registry.register(URI_2);

        assertEq(id1, 1);
        assertEq(id2, 2);
    }

    function test_register_reverts_empty_uri() public {
        vm.prank(agent1);
        vm.expectRevert(NeuronIdentityRegistry.EmptyAgentURI.selector);
        registry.register("");
    }

    function test_register_reverts_duplicate() public {
        vm.prank(agent1);
        registry.register(URI_1);

        vm.prank(agent1);
        vm.expectRevert(abi.encodeWithSelector(NeuronIdentityRegistry.AlreadyRegistered.selector, agent1));
        registry.register(URI_2);
    }

    // ===================== Lookup =====================

    function test_lookup_registered() public {
        vm.prank(agent1);
        registry.register(URI_1);

        (uint256 agentId, string memory uri) = registry.lookup(agent1);
        assertEq(agentId, 1);
        assertEq(uri, URI_1);
    }

    function test_lookup_unregistered() public view {
        (uint256 agentId, string memory uri) = registry.lookup(agent1);
        assertEq(agentId, 0);
        assertEq(bytes(uri).length, 0);
    }

    // ===================== agentURI =====================

    function test_agentURI_returns_stored_uri() public {
        vm.prank(agent1);
        uint256 agentId = registry.register(URI_1);

        assertEq(registry.agentURI(agentId), URI_1);
    }

    function test_agentURI_reverts_nonexistent() public {
        vm.expectRevert();
        registry.agentURI(999);
    }

    // ===================== tokenURI (ERC-721 compatibility) =====================

    function test_tokenURI_returns_agentURI() public {
        vm.prank(agent1);
        uint256 agentId = registry.register(URI_1);

        // tokenURI() must return the same value as agentURI()
        assertEq(registry.tokenURI(agentId), URI_1);
        assertEq(registry.tokenURI(agentId), registry.agentURI(agentId));
    }

    function test_tokenURI_reverts_nonexistent() public {
        vm.expectRevert();
        registry.tokenURI(999);
    }

    // ===================== setAgentURI (ERC-8004) =====================

    function test_setAgentURI_by_owner() public {
        vm.prank(agent1);
        uint256 agentId = registry.register(URI_1);

        vm.prank(agent1);
        registry.setAgentURI(agentId, URI_UPDATED);

        assertEq(registry.agentURI(agentId), URI_UPDATED);
        // tokenURI also reflects the change
        assertEq(registry.tokenURI(agentId), URI_UPDATED);
    }

    function test_setAgentURI_emits_erc8004_event() public {
        vm.prank(agent1);
        uint256 agentId = registry.register(URI_1);

        vm.prank(agent1);
        vm.expectEmit(true, true, false, true);
        emit URIUpdated(agentId, URI_UPDATED, agent1);
        registry.setAgentURI(agentId, URI_UPDATED);
    }

    function test_setAgentURI_updatedBy_reflects_caller() public {
        vm.prank(agent1);
        uint256 agentId = registry.register(URI_1);

        // Approve operator
        vm.prank(agent1);
        registry.approve(operator, agentId);

        // Operator updates — event should show operator as updatedBy
        vm.prank(operator);
        vm.expectEmit(true, true, false, true);
        emit URIUpdated(agentId, URI_UPDATED, operator);
        registry.setAgentURI(agentId, URI_UPDATED);
    }

    function test_setAgentURI_by_approved_operator() public {
        vm.prank(agent1);
        uint256 agentId = registry.register(URI_1);

        vm.prank(agent1);
        registry.approve(operator, agentId);

        vm.prank(operator);
        registry.setAgentURI(agentId, URI_UPDATED);

        assertEq(registry.agentURI(agentId), URI_UPDATED);
    }

    function test_setAgentURI_by_approved_for_all() public {
        vm.prank(agent1);
        uint256 agentId = registry.register(URI_1);

        vm.prank(agent1);
        registry.setApprovalForAll(operator, true);

        vm.prank(operator);
        registry.setAgentURI(agentId, URI_UPDATED);

        assertEq(registry.agentURI(agentId), URI_UPDATED);
    }

    function test_setAgentURI_reverts_unauthorized() public {
        vm.prank(agent1);
        uint256 agentId = registry.register(URI_1);

        vm.prank(agent2);
        vm.expectRevert(
            abi.encodeWithSelector(NeuronIdentityRegistry.NotOwnerOrApproved.selector, agentId, agent2)
        );
        registry.setAgentURI(agentId, URI_UPDATED);
    }

    function test_setAgentURI_reverts_empty_uri() public {
        vm.prank(agent1);
        uint256 agentId = registry.register(URI_1);

        vm.prank(agent1);
        vm.expectRevert(NeuronIdentityRegistry.EmptyAgentURI.selector);
        registry.setAgentURI(agentId, "");
    }

    // ===================== Revoke =====================

    function test_revoke_by_owner() public {
        vm.prank(agent1);
        uint256 agentId = registry.register(URI_1);

        vm.prank(agent1);
        registry.revoke(agentId);

        vm.expectRevert();
        registry.ownerOf(agentId);
    }

    function test_revoke_emits_event() public {
        vm.prank(agent1);
        uint256 agentId = registry.register(URI_1);

        vm.prank(agent1);
        vm.expectEmit(true, true, false, false);
        emit IdentityRevoked(agentId, agent1);
        registry.revoke(agentId);
    }

    function test_revoke_clears_reverse_mapping() public {
        vm.prank(agent1);
        uint256 agentId = registry.register(URI_1);

        vm.prank(agent1);
        registry.revoke(agentId);

        (uint256 lookupId, string memory uri) = registry.lookup(agent1);
        assertEq(lookupId, 0);
        assertEq(bytes(uri).length, 0);
    }

    function test_revoke_clears_tokenURI() public {
        vm.prank(agent1);
        uint256 agentId = registry.register(URI_1);

        vm.prank(agent1);
        registry.revoke(agentId);

        // tokenURI should revert for burned token
        vm.expectRevert();
        registry.tokenURI(agentId);
    }

    function test_revoke_reverts_non_owner() public {
        vm.prank(agent1);
        uint256 agentId = registry.register(URI_1);

        vm.prank(agent2);
        vm.expectRevert(
            abi.encodeWithSelector(NeuronIdentityRegistry.NotTokenOwner.selector, agentId, agent2)
        );
        registry.revoke(agentId);
    }

    function test_revoke_reverts_approved_operator() public {
        vm.prank(agent1);
        uint256 agentId = registry.register(URI_1);

        vm.prank(agent1);
        registry.approve(operator, agentId);

        vm.prank(operator);
        vm.expectRevert(
            abi.encodeWithSelector(NeuronIdentityRegistry.NotTokenOwner.selector, agentId, operator)
        );
        registry.revoke(agentId);
    }

    // ===================== Re-registration after revoke =====================

    function test_reregister_after_revoke() public {
        vm.prank(agent1);
        uint256 id1 = registry.register(URI_1);

        vm.prank(agent1);
        registry.revoke(id1);

        vm.prank(agent1);
        uint256 id2 = registry.register(URI_UPDATED);

        assertTrue(id2 > id1, "Re-registration should get new agentId");
        assertEq(registry.ownerOf(id2), agent1);
        assertEq(registry.agentURI(id2), URI_UPDATED);
        assertEq(registry.tokenURI(id2), URI_UPDATED);
    }

    // ===================== ERC-721 Standard =====================

    function test_supports_interface_erc721() public view {
        assertTrue(registry.supportsInterface(0x80ac58cd));
    }

    function test_supports_interface_erc721_enumerable() public view {
        assertTrue(registry.supportsInterface(0x780e9d63));
    }

    function test_supports_interface_erc165() public view {
        assertTrue(registry.supportsInterface(0x01ffc9a7));
    }

    function test_token_of_owner_by_index() public {
        vm.prank(agent1);
        uint256 agentId = registry.register(URI_1);

        assertEq(registry.tokenOfOwnerByIndex(agent1, 0), agentId);
    }

    function test_balance_of() public {
        vm.prank(agent1);
        registry.register(URI_1);

        assertEq(registry.balanceOf(agent1), 1);
    }

    // ===================== Admin cannot act on behalf of agents =====================

    function test_admin_cannot_register_for_agent() public {
        address adminEOA = vm.addr(99);
        vm.prank(adminEOA);
        uint256 agentId = registry.register(URI_1);
        assertEq(registry.ownerOf(agentId), adminEOA);
        (uint256 agentLookup,) = registry.lookup(agent1);
        assertEq(agentLookup, 0);
    }

    // ===================== Transfer =====================

    function test_transfer_allowed() public {
        vm.prank(agent1);
        uint256 agentId = registry.register(URI_1);

        vm.prank(agent1);
        registry.transferFrom(agent1, agent2, agentId);

        assertEq(registry.ownerOf(agentId), agent2);
    }

    function test_transfer_updates_reverse_mapping() public {
        vm.prank(agent1);
        uint256 agentId = registry.register(URI_1);

        vm.prank(agent1);
        registry.transferFrom(agent1, agent2, agentId);

        (uint256 id1,) = registry.lookup(agent1);
        assertEq(id1, 0);

        (uint256 id2, string memory uri) = registry.lookup(agent2);
        assertEq(id2, agentId);
        assertEq(uri, URI_1);
    }

    function test_transfer_blocked_if_recipient_already_registered() public {
        vm.prank(agent1);
        registry.register(URI_1);

        vm.prank(agent2);
        uint256 id2 = registry.register(URI_2);

        vm.prank(agent2);
        vm.expectRevert(abi.encodeWithSelector(NeuronIdentityRegistry.AlreadyRegistered.selector, agent1));
        registry.transferFrom(agent2, agent1, id2);
    }
}
