// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "forge-std/Script.sol";
import "../src/NeuronIdentityRegistry.sol";
import "../src/NeuronEscrow.sol";
import "../src/TestToken.sol";

/// @title DeployAll
/// @notice Deploys all Neuron contracts to the target network.
///         Usage: forge script script/Deploy.s.sol --rpc-url $RPC --private-key $KEY --broadcast
contract DeployAll is Script {
    function run() external {
        vm.startBroadcast();

        NeuronIdentityRegistry registry = new NeuronIdentityRegistry();
        console.log("NeuronIdentityRegistry deployed at:", address(registry));

        NeuronEscrow escrow = new NeuronEscrow();
        console.log("NeuronEscrow deployed at:", address(escrow));

        TestToken token = new TestToken();
        console.log("TestToken deployed at:", address(token));

        // Mint initial test tokens to deployer
        token.mint(msg.sender, 100_000_000_000); // 100B tokens
        console.log("Minted 100B TestToken to deployer:", msg.sender);

        vm.stopBroadcast();
    }
}

/// @title DeployRegistry
/// @notice Deploy only the Identity Registry.
contract DeployRegistry is Script {
    function run() external {
        vm.startBroadcast();
        NeuronIdentityRegistry registry = new NeuronIdentityRegistry();
        console.log("NeuronIdentityRegistry deployed at:", address(registry));
        vm.stopBroadcast();
    }
}

/// @title DeployEscrow
/// @notice Deploy Escrow + TestToken.
contract DeployEscrow is Script {
    function run() external {
        vm.startBroadcast();

        NeuronEscrow escrow = new NeuronEscrow();
        console.log("NeuronEscrow deployed at:", address(escrow));

        TestToken token = new TestToken();
        console.log("TestToken deployed at:", address(token));

        token.mint(msg.sender, 100_000_000_000);
        console.log("Minted 100B TestToken to deployer:", msg.sender);

        vm.stopBroadcast();
    }
}
