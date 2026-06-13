# API Contract: Service Schemas

**Source**: spec.md FR-T14, FR-T15, FR-T17, FR-T18

---

## NeuronTopicService JSON Schema

EIP-8004 service object representing a topic channel in the agentURI `services` array.

### Schema

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "NeuronTopicService",
  "description": "EIP-8004 service object for a Neuron topic channel (FR-T14)",
  "type": "object",
  "required": ["type", "name", "version", "channel", "transport", "anchor", "config"],
  "properties": {
    "type": {
      "type": "string",
      "const": "neuron-topic",
      "description": "Discriminator. Always 'neuron-topic' (FR-T14)"
    },
    "name": {
      "type": "string",
      "description": "Channel role: 'stdIn', 'stdOut', 'stdErr', or 'custom:<name>' (FR-T07, FR-T08)",
      "pattern": "^(stdIn|stdOut|stdErr|custom:[a-zA-Z0-9_-]+)$"
    },
    "endpoint": {
      "type": "string",
      "description": "Compact Topic URI for backward-compatible EIP-8004 consumers (FR-T14). SHOULD be present but structured fields are authoritative"
    },
    "version": {
      "type": "string",
      "description": "Topic protocol version in semver format (FR-T14)",
      "pattern": "^\\d+\\.\\d+\\.\\d+$"
    },
    "channel": {
      "type": "string",
      "description": "Neuron channel role (FR-T14). Same value as 'name' for standard channels",
      "pattern": "^(stdIn|stdOut|stdErr|custom:[a-zA-Z0-9_-]+)$"
    },
    "transport": {
      "type": "string",
      "description": "Backend kind (FR-T14, FR-T15)",
      "pattern": "^(hcs|erc-log|kafka|custom:[a-zA-Z0-9_-]+)$"
    },
    "anchor": {
      "type": "string",
      "description": "Ledger that anchors the topic (FR-T14). For ledger-native transports, anchor = the transport's own ledger"
    },
    "config": {
      "type": "object",
      "description": "Transport-specific configuration (FR-T15). Schema varies by transport value"
    }
  },
  "additionalProperties": true
}
```

### HCS Config Schema (`transport: "hcs"`)

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "HCSConfig",
  "description": "Transport config for Hedera Consensus Service (FR-T15)",
  "type": "object",
  "required": ["network", "topicId"],
  "properties": {
    "network": {
      "type": "string",
      "description": "Hedera network identifier",
      "enum": ["hedera-mainnet", "hedera-testnet", "hedera-previewnet"]
    },
    "topicId": {
      "type": "string",
      "description": "HCS topic ID in shard.realm.num format",
      "pattern": "^\\d+\\.\\d+\\.\\d+$"
    }
  },
  "additionalProperties": false
}
```

**Example** (FR-T15):

```json
{
  "type": "neuron-topic",
  "name": "stdIn",
  "endpoint": "hcs://0.0.4515382",
  "version": "1.0.0",
  "channel": "stdIn",
  "transport": "hcs",
  "anchor": "hedera-mainnet",
  "config": {
    "network": "hedera-mainnet",
    "topicId": "0.0.4515382"
  }
}
```

**Validation rules**:
- `config.network` MUST be a recognized Hedera network identifier.
- `config.topicId` MUST match the `shard.realm.num` format (e.g. `"0.0.4515382"`).
- `anchor` SHOULD match `config.network` for HCS (the anchor ledger IS the Hedera network).
- `endpoint` SHOULD follow the `hcs://<topicId>` URI scheme. If present, it SHOULD be consistent with `config.topicId`. Structured fields are authoritative if inconsistent.

---

### ERC Event Log Config Schema (`transport: "erc-log"`)

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "ERCLogConfig",
  "description": "Transport config for ERC event logs on Ethereum/EVM chains (FR-T15)",
  "type": "object",
  "required": ["chainId", "contractAddress", "eventSignature"],
  "properties": {
    "chainId": {
      "type": "integer",
      "minimum": 1,
      "description": "EVM chain ID (e.g. 1 for Ethereum mainnet)"
    },
    "contractAddress": {
      "type": "string",
      "description": "EVM contract address (EIP-55 checksummed or lowercase)",
      "pattern": "^0x[0-9a-fA-F]{40}$"
    },
    "eventSignature": {
      "type": "string",
      "description": "Solidity event signature (e.g. 'TopicMessage(address,uint256,bytes)')"
    }
  },
  "additionalProperties": false
}
```

**Example** (FR-T15):

```json
{
  "type": "neuron-topic",
  "name": "stdOut",
  "endpoint": "erc-log://1:0xAbCdEf1234567890AbCdEf1234567890AbCdEf12",
  "version": "1.0.0",
  "channel": "stdOut",
  "transport": "erc-log",
  "anchor": "eip155:1",
  "config": {
    "chainId": 1,
    "contractAddress": "0xAbCdEf1234567890AbCdEf1234567890AbCdEf12",
    "eventSignature": "TopicMessage(address,uint256,bytes)"
  }
}
```

**Validation rules**:
- `config.chainId` MUST be a positive integer.
- `config.contractAddress` MUST be a valid 40-hex-character EVM address with `0x` prefix.
- `config.eventSignature` MUST be a valid Solidity event signature string.
- `anchor` SHOULD follow the `eip155:<chainId>` format matching `config.chainId`.
- `endpoint` SHOULD follow the `erc-log://<chainId>:<contractAddress>` URI scheme.

---

### Kafka Config Schema (`transport: "kafka"`)

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "KafkaConfig",
  "description": "Transport config for Kafka with ledger anchoring (FR-T15, FR-T16)",
  "type": "object",
  "required": ["bootstrapServers", "topicName", "anchoring"],
  "properties": {
    "bootstrapServers": {
      "type": "array",
      "items": { "type": "string" },
      "minItems": 1,
      "description": "Kafka broker addresses (host:port)"
    },
    "topicName": {
      "type": "string",
      "description": "Kafka topic name",
      "minLength": 1
    },
    "saslMechanism": {
      "type": "string",
      "description": "SASL authentication mechanism (e.g. 'SCRAM-SHA-512')",
      "enum": ["PLAIN", "SCRAM-SHA-256", "SCRAM-SHA-512"]
    },
    "anchoring": {
      "type": "object",
      "required": ["method", "anchorTopicId", "anchorNetwork", "interval"],
      "description": "Ledger anchoring configuration (FR-T16). REQUIRED for non-ledger-native transports",
      "properties": {
        "method": {
          "type": "string",
          "description": "Anchoring method (e.g. 'hcs-hash-chain')"
        },
        "anchorTopicId": {
          "type": "string",
          "description": "Topic ID on the anchor ledger for proof submission",
          "pattern": "^\\d+\\.\\d+\\.\\d+$"
        },
        "anchorNetwork": {
          "type": "string",
          "description": "Network identifier of the anchor ledger (e.g. 'hedera-mainnet')"
        },
        "interval": {
          "type": "string",
          "description": "Anchoring frequency (e.g. 'every-batch', 'every-100-messages', 'every-60-seconds')"
        }
      },
      "additionalProperties": false
    }
  },
  "additionalProperties": false
}
```

**Example** (FR-T15, FR-T16):

```json
{
  "type": "neuron-topic",
  "name": "stdOut",
  "endpoint": "kafka+ledger://kafka1.neuron.network:9092/neuron.agent.alice.stdout",
  "version": "1.0.0",
  "channel": "stdOut",
  "transport": "kafka",
  "anchor": "hedera-mainnet",
  "config": {
    "bootstrapServers": ["kafka1.neuron.network:9092"],
    "topicName": "neuron.agent.alice.stdout",
    "saslMechanism": "SCRAM-SHA-512",
    "anchoring": {
      "method": "hcs-hash-chain",
      "anchorTopicId": "0.0.9999999",
      "anchorNetwork": "hedera-mainnet",
      "interval": "every-batch"
    }
  }
}
```

**Validation rules** (FR-T15, FR-T16):
- `config.bootstrapServers` MUST contain at least one broker address.
- `config.topicName` MUST be non-empty.
- `config.anchoring` MUST be present (FR-T16 -- non-ledger-native transports MUST have valid anchoring config).
- `config.anchoring.method` MUST be a recognized anchoring method.
- `config.anchoring.anchorTopicId` MUST be a valid topic ID on the anchor ledger.
- `config.anchoring.anchorNetwork` MUST be a recognized network identifier.
- `config.anchoring.interval` MUST be a recognized interval specification.
- `anchor` SHOULD match `config.anchoring.anchorNetwork`.
- Missing or invalid `anchoring` config on a Kafka service MUST produce `InvalidConfig` error.
- `endpoint` SHOULD follow the `kafka+ledger://<broker>/<topicName>` URI scheme.

---

## NeuronP2PExchangeService JSON Schema

EIP-8004 service object defining the method for multiaddress discovery.

### Schema

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "NeuronP2PExchangeService",
  "description": "EIP-8004 service object for peer-to-peer multiaddress exchange (FR-T17)",
  "type": "object",
  "required": ["type", "name", "version", "peerID", "protocol", "topicRef"],
  "properties": {
    "type": {
      "type": "string",
      "const": "neuron-p2p-exchange",
      "description": "Discriminator. Always 'neuron-p2p-exchange' (FR-T17)"
    },
    "name": {
      "type": "string",
      "description": "Service name (e.g. 'p2p')"
    },
    "version": {
      "type": "string",
      "description": "Exchange protocol version in semver format (FR-T17)",
      "pattern": "^\\d+\\.\\d+\\.\\d+$"
    },
    "peerID": {
      "type": "string",
      "description": "Libp2p PeerID derived from NeuronPublicKey via Key Library 002 (FR-T17). Base58btc-encoded multihash"
    },
    "protocol": {
      "type": "string",
      "description": "Protocol ID for multiaddress exchange (FR-T17). Must start with '/'",
      "pattern": "^/"
    },
    "topicRef": {
      "type": "string",
      "description": "Cross-reference to a neuron-topic service 'name' in the same agentURI (FR-T17, FR-T18)",
      "pattern": "^(stdIn|stdOut|stdErr|custom:[a-zA-Z0-9_-]+)$"
    }
  },
  "additionalProperties": true
}
```

### Example (FR-T17)

```json
{
  "type": "neuron-p2p-exchange",
  "name": "p2p",
  "version": "1.0.0",
  "peerID": "12D3KooWA1b2c3D4e5F6g7H8i9J0kLmNoPqRsTuVwXyZ",
  "protocol": "/neuron/multiaddr-exchange/1.0.0",
  "topicRef": "stdIn"
}
```

### Validation Rules (FR-T17, FR-T18)

**Field validation**:
- `type` MUST be exactly `"neuron-p2p-exchange"`.
- `name` MUST be non-empty.
- `version` MUST be valid semver.
- `peerID` MUST be a valid libp2p PeerID string (base58btc-encoded multihash). Validation: base58btc-decode the string and verify it is a valid multihash.
- `protocol` MUST be a valid protocol ID string starting with `/`.
- `topicRef` MUST be a valid channel role name (standard or custom).

**Cross-reference validation** (FR-T18):
- `topicRef` MUST resolve to an existing `neuron-topic` service `name` in the same agentURI document.
- Validation is performed at the document level, not per-service.
- If `topicRef` references a name that does not exist in the `services` array, produce `BrokenTopicRef` error (SC-T10).

---

## Service Parsing Functions

### ParseAgentURIServices

Parses the `services` array from an EIP-8004 agentURI JSON document.

**Input**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `jsonBytes` | ByteArray | Complete agentURI JSON document |

**Output**: Returns (Array\<NeuronTopicService\>, Array\<NeuronP2PExchangeService\>). Raises Error if parsing fails.

**Behavior** (FR-T09):
1. Parse JSON into a generic structure.
2. Extract the `services` array.
3. For each service object, check the `type` field:
   - `"neuron-topic"` -> deserialize into `NeuronTopicService` and validate all required fields.
   - `"neuron-p2p-exchange"` -> deserialize into `NeuronP2PExchangeService` and validate all required fields.
   - Other types -> ignore (forward-compatible).
4. Return the typed service slices.
5. If any required field is missing or invalid, return `InvalidConfig` error identifying the specific field.

### ValidateCrossReferences

Validates that all `topicRef` fields in p2p exchange services resolve to existing topic services.

**Input**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `topics` | []NeuronTopicService | Parsed topic services from the agentURI |
| `p2p` | []NeuronP2PExchangeService | Parsed p2p exchange services |

**Output**: Raises Error if any cross-reference is broken.

**Behavior** (FR-T18, SC-T10):
1. Build a set of all topic service names from `topics`.
2. For each p2p service in `p2p`, check that `topicRef` exists in the name set.
3. If any `topicRef` is not found, raise `BrokenTopicRef` error with the broken reference name.
4. If all references are valid, return success.

### ExtractTopicRef

Extracts a TopicRef from a NeuronTopicService.

**Input**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `svc` | NeuronTopicService | A parsed neuron-topic service |

**Output**: Returns TopicRef. Raises Error if extraction fails.

**Behavior** (FR-T09):
1. Read `svc.Transport` as `BackendKind`.
2. Extract the locator from `svc.Config` based on transport:
   - HCS: `config.topicId`
   - ERC-log: `config.chainId` + `config.contractAddress` (composite locator)
   - Kafka: `config.topicName`
   - Custom: adapter-defined extraction
3. Construct and return `TopicRef{Transport: transport, Locator: locator}`.
4. Validate the TopicRef (FR-T12). Return `InvalidTopicRef` if invalid.

### SerializeTopicService

Serializes a NeuronTopicService to JSON for inclusion in an agentURI document.

**Input**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `svc` | NeuronTopicService | Service object to serialize |

**Output**: Returns ByteArray. Raises Error if validation fails.

**Behavior** (FR-T14, SC-T09):
1. Validate all required fields are present.
2. Serialize to JSON with canonical field order: `type`, `name`, `endpoint`, `version`, `channel`, `transport`, `anchor`, `config`.
3. Return JSON bytes.

**Round-trip guarantee** (SC-T09): `SerializeTopicService(ParseNeuronTopicService(bytes))` produces identical JSON bytes for all registered transports.

---

## Topic URI Serialization

### TopicRefToURI

Serializes a TopicRef to a compact Topic URI string.

**Input**: `TopicRef`

**Output**: `string`

**URI formats** (spec Topic URI Scheme):

| Transport | URI Pattern | Example |
|-----------|-------------|---------|
| `hcs` | `hcs://<topicId>` | `hcs://0.0.4515382` |
| `erc-log` | `erc-log://<chainId>:<contractAddress>` | `erc-log://1:0xAbCdEf...` |
| `kafka` | `kafka+ledger://<broker>/<topicName>` | `kafka+ledger://kafka1.neuron.network:9092/neuron.agent.stdin` |

### TopicRefFromURI

Parses a compact Topic URI string into a TopicRef.

**Input**: `string` (URI)

**Output**: `(TopicRef, error)`

**Behavior**:
1. Parse the URI scheme to determine `BackendKind`.
2. Extract the locator from the URI path/authority.
3. Construct and validate the TopicRef.
4. Return `InvalidTopicRef` error for unrecognized schemes or malformed URIs.

---

## Complete agentURI Example

A complete agentURI JSON document with all Neuron service types:

```json
{
  "type": "https://eips.ethereum.org/EIPS/eip-8004#registration-v1",
  "name": "agent-alice-sensor-01",
  "description": "Neuron IoT data agent for Alice's sensor fleet",
  "services": [
    {
      "type": "neuron-topic",
      "name": "stdIn",
      "endpoint": "hcs://0.0.4515382",
      "version": "1.0.0",
      "channel": "stdIn",
      "transport": "hcs",
      "anchor": "hedera-mainnet",
      "config": {
        "network": "hedera-mainnet",
        "topicId": "0.0.4515382"
      }
    },
    {
      "type": "neuron-topic",
      "name": "stdOut",
      "endpoint": "kafka+ledger://kafka1.neuron.network:9092/neuron.agent.alice.stdout",
      "version": "1.0.0",
      "channel": "stdOut",
      "transport": "kafka",
      "anchor": "hedera-mainnet",
      "config": {
        "bootstrapServers": ["kafka1.neuron.network:9092"],
        "topicName": "neuron.agent.alice.stdout",
        "saslMechanism": "SCRAM-SHA-512",
        "anchoring": {
          "method": "hcs-hash-chain",
          "anchorTopicId": "0.0.9999999",
          "anchorNetwork": "hedera-mainnet",
          "interval": "every-batch"
        }
      }
    },
    {
      "type": "neuron-topic",
      "name": "stdErr",
      "endpoint": "hcs://0.0.4515383",
      "version": "1.0.0",
      "channel": "stdErr",
      "transport": "hcs",
      "anchor": "hedera-mainnet",
      "config": {
        "network": "hedera-mainnet",
        "topicId": "0.0.4515383"
      }
    },
    {
      "type": "neuron-p2p-exchange",
      "name": "p2p",
      "version": "1.0.0",
      "peerID": "12D3KooWA1b2c3D4e5F6g7H8i9J0kLmNoPqRsTuVwXyZ",
      "protocol": "/neuron/multiaddr-exchange/1.0.0",
      "topicRef": "stdIn"
    }
  ],
  "active": true,
  "registrations": [
    {
      "agentId": 42,
      "agentRegistry": "eip155:295:0xNeuronRegistryContract"
    }
  ]
}
```

**Parsing this document** (FR-T09):
1. `ParseAgentURIServices(json)` returns 3 `NeuronTopicService` objects (stdIn, stdOut, stdErr) and 1 `NeuronP2PExchangeService` object (p2p).
2. `ValidateCrossReferences(topics, p2p)` succeeds because `topicRef: "stdIn"` resolves to the stdIn topic service.
3. `ExtractTopicRef(stdInService)` returns `TopicRef{Transport: "hcs", Locator: "0.0.4515382"}`.
4. `ExtractTopicRef(stdOutService)` returns `TopicRef{Transport: "kafka", Locator: "neuron.agent.alice.stdout"}`.
