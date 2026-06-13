// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";

/// @title TestToken
/// @notice Mintable ERC-20 token for testing the NeuronEscrow contract.
///         Anyone can mint — this is NOT for production use.
contract TestToken is ERC20 {
    constructor() ERC20("Neuron Test Token", "NTT") {}

    /// @notice Mint tokens to any address. Unrestricted for testing.
    function mint(address to, uint256 amount) external {
        _mint(to, amount);
    }
}
