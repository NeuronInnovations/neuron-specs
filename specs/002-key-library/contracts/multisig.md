# API Contract: MultisigKey

**Source**: spec.md FR-023, FR-024

---

## NewMultisigKey

Creates a MultisigKey from multiple NeuronPrivateKey instances (secp256k1-aggregated mode).

**Input**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `keys` | Array\<NeuronPrivateKey\> | Participating keys |
| `threshold` | UnsignedInteger | m in m-of-n (must be 1 ≤ m ≤ n) |

**Output**: Returns MultisigKey. Raises Error if validation fails.

**Behavior** (FR-024):
1. Validate `threshold ≤ len(keys)` and `threshold ≥ 1`
2. Set protocol = `"secp256k1-aggregated"`
3. Store key references and threshold
4. **Note**: Aggregation algorithm deferred (GAP-005) — signing operations not available until specified

---

## MultisigKeyFromBlockchainKey

Creates a MultisigKey from a blockchain-specific threshold key.

**Input**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `key` | (implementation-specific blockchain SDK threshold key) | Blockchain SDK threshold key (e.g., Hedera threshold key) |
| `protocol` | string | Protocol identifier (e.g., `"hedera-threshold"`, `"frost"`, `"bls"`) |

**Output**: Returns MultisigKey. Raises Error if protocol is unrecognized or extraction fails.

**Behavior** (FR-023):
1. Validate protocol is recognized
2. Wrap blockchain key with protocol metadata
3. Return MultisigKey

---

## MultisigKey.Protocol

Returns the threshold signature protocol identifier.

**Output**: `string` (e.g., `"secp256k1-aggregated"`, `"hedera-threshold"`)

---

## MultisigKey.EVMAddress / PeerID

Derives EVM address or PeerID (only for `"secp256k1-aggregated"` mode).

**Output**: Returns EVMAddress or Error / Returns PeerID or Error

**Behavior** (FR-023):
- If protocol == `"secp256k1-aggregated"`: derive from aggregated key (when algorithm is specified)
- Otherwise: return `UnsupportedKeyType` error with message indicating the operation is not supported for the protocol

---

## MultisigKey.ToBlockchainKey / FromBlockchainKey

Bidirectional conversion to/from blockchain SDK multisig types.

**Behavior** (FR-023): Supports conversion for all tracked protocols. The underlying blockchain SDK handles protocol-specific details.
