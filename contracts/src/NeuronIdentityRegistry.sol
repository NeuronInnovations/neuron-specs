// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "@openzeppelin/contracts/token/ERC721/extensions/ERC721Enumerable.sol";
import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";

/// @title NeuronIdentityRegistry
/// @notice ERC-8004 Identity Registry — maps an agent's EVMAddress to a registration
///         NFT containing an opaque agentURI string.
/// @dev Implements ERC-8004 core interface (register, setAgentURI, tokenURI) plus
///      Neuron project extensions (revoke, lookup, one-per-address).
///      Spec 007 FR-C-01 through FR-C-19 (minimum viable subset).
///
///      ERC-8004 conformance:
///        - register(agentURI) → agentId                          ✓
///        - setAgentURI(agentId, newURI)                          ✓
///        - tokenURI(agentId) returns agentURI                    ✓
///        - Registered / URIUpdated events                        ✓
///      Deferred (per spec 007):
///        - register(agentURI, MetadataEntry[]) overload
///        - register() bare overload
///        - getMetadata / setMetadata
///        - setAgentWallet / getAgentWallet / unsetAgentWallet
///      Project extensions (not in ERC-8004):
///        - revoke(agentId) — burn with reverse mapping cleanup
///        - lookup(address) — O(1) reverse discovery
///        - One-registration-per-address enforcement
///        - IdentityRevoked event
contract NeuronIdentityRegistry is ERC721Enumerable, Ownable, ReentrancyGuard {
    // ===================== Errors =====================

    /// @notice Thrown when an address already has a registration. FR-C-06.
    error AlreadyRegistered(address account);

    /// @notice Thrown when caller is not token owner or approved operator. FR-C-07.
    error NotOwnerOrApproved(uint256 agentId, address caller);

    /// @notice Thrown when caller is not the token owner (for owner-only operations). FR-C-08.
    error NotTokenOwner(uint256 agentId, address caller);

    /// @notice Thrown when agentURI is empty. FR-C-04.
    error EmptyAgentURI();

    // ===================== ERC-8004 Events =====================

    /// @notice Emitted when a new agent is registered. ERC-8004 conformant.
    event Registered(
        uint256 indexed agentId,
        string agentURI,
        address indexed owner
    );

    /// @notice Emitted when an agent's URI is updated. ERC-8004 conformant.
    event URIUpdated(
        uint256 indexed agentId,
        string newURI,
        address indexed updatedBy
    );

    // ===================== Project-Specific Events =====================

    /// @notice Emitted when an identity is revoked (token burned). Neuron extension.
    event IdentityRevoked(uint256 indexed agentId, address indexed owner);

    // ===================== State =====================

    /// @dev Reverse mapping: address → agentId. 0 means not registered.
    mapping(address => uint256) private _addressToAgentId;

    /// @dev agentId → agentURI string.
    mapping(uint256 => string) private _agentURIs;

    /// @dev Auto-incrementing agent ID counter. Starts at 1 (0 = unregistered sentinel).
    uint256 private _nextAgentId = 1;

    // ===================== Constructor =====================

    constructor() ERC721("Neuron Identity", "NID") Ownable(msg.sender) {}

    // ===================== ERC-8004: Registration =====================

    /// @notice Register a new identity by minting an NFT to msg.sender. ERC-8004.
    /// @param _agentURI The opaque agentURI string (must not be empty).
    /// @return agentId The newly minted agent ID (ERC-721 tokenId).
    function register(
        string calldata _agentURI
    ) external nonReentrant returns (uint256 agentId) {
        if (bytes(_agentURI).length == 0) revert EmptyAgentURI();
        if (_addressToAgentId[msg.sender] != 0)
            revert AlreadyRegistered(msg.sender);

        agentId = _nextAgentId++;
        _safeMint(msg.sender, agentId);
        _agentURIs[agentId] = _agentURI;
        _addressToAgentId[msg.sender] = agentId;

        emit Registered(agentId, _agentURI, msg.sender);
    }

    // ===================== ERC-8004: URI Management =====================

    /// @notice Update the agentURI for an existing registration. ERC-8004 `setAgentURI`.
    /// @param agentId The agent to update.
    /// @param newURI The new agentURI (must not be empty).
    function setAgentURI(
        uint256 agentId,
        string calldata newURI
    ) external nonReentrant {
        if (bytes(newURI).length == 0) revert EmptyAgentURI();
        if (!_isOwnerOrApproved(agentId, msg.sender)) {
            revert NotOwnerOrApproved(agentId, msg.sender);
        }

        _agentURIs[agentId] = newURI;
        emit URIUpdated(agentId, newURI, msg.sender);
    }

    // ===================== ERC-721 URIStorage Compatibility =====================

    /// @notice Returns the agentURI for the given token. ERC-721 tokenURI standard.
    /// @dev Overrides ERC721.tokenURI() so NFT tools, wallets, and marketplaces
    ///      can discover the agent's service metadata via the standard interface.
    function tokenURI(
        uint256 agentId
    ) public view override returns (string memory) {
        _requireOwned(agentId);
        return _agentURIs[agentId];
    }

    // ===================== Query Functions =====================

    /// @notice Get the agentURI for an agent. Convenience alias for tokenURI().
    /// @param agentId The agent to query.
    /// @return The stored agentURI string.
    function agentURI(uint256 agentId) external view returns (string memory) {
        _requireOwned(agentId);
        return _agentURIs[agentId];
    }

    /// @notice Look up registration by address. Neuron project extension.
    /// @param account The address to look up.
    /// @return agentId The agent ID (0 if not registered).
    /// @return uri The agentURI (empty if not registered).
    function lookup(
        address account
    ) external view returns (uint256 agentId, string memory uri) {
        agentId = _addressToAgentId[account];
        if (agentId != 0) {
            uri = _agentURIs[agentId];
        }
    }

    // ===================== Project Extension: Revoke =====================

    /// @notice Revoke a registration by burning the token. Owner only. Neuron extension.
    /// @param agentId The agent to revoke.
    function revoke(uint256 agentId) external nonReentrant {
        address tokenOwner = ownerOf(agentId);
        if (msg.sender != tokenOwner) revert NotTokenOwner(agentId, msg.sender);

        _addressToAgentId[tokenOwner] = 0;
        delete _agentURIs[agentId];
        _burn(agentId);

        emit IdentityRevoked(agentId, tokenOwner);
    }

    // ===================== Transfer Hook =====================

    /// @dev Override to maintain the reverse mapping on transfers.
    ///      Enforces one-registration-per-address for the recipient.
    function _update(
        address to,
        uint256 agentId,
        address auth
    ) internal override returns (address) {
        address from = super._update(to, agentId, auth);

        if (from != address(0) && to != address(0)) {
            if (_addressToAgentId[to] != 0) revert AlreadyRegistered(to);
            _addressToAgentId[from] = 0;
            _addressToAgentId[to] = agentId;
        }

        return from;
    }

    // ===================== Internal =====================

    /// @dev Check if caller is owner or approved for the agent.
    function _isOwnerOrApproved(
        uint256 agentId,
        address caller
    ) internal view returns (bool) {
        address tokenOwner = ownerOf(agentId);
        return
            caller == tokenOwner ||
            getApproved(agentId) == caller ||
            isApprovedForAll(tokenOwner, caller);
    }
}
