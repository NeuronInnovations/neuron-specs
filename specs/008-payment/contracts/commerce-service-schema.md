# API Contract: Commerce Service Schema

**Source**: spec.md FR-P01–P05, data-model.md NeuronCommerceService

---

## NeuronCommerceService JSON Schema

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "NeuronCommerceService",
  "description": "EIP-8004 service object for a Neuron commerce offering (FR-P01)",
  "type": "object",
  "required": ["type", "name", "version", "delivery", "settlement", "pricing"],
  "properties": {
    "type": {
      "type": "string",
      "const": "neuron-commerce",
      "description": "Discriminator. Always 'neuron-commerce' (FR-P01)"
    },
    "name": {
      "type": "string",
      "minLength": 1,
      "description": "Human-readable service name (FR-P01)"
    },
    "version": {
      "type": "string",
      "pattern": "^\\d+\\.\\d+\\.\\d+$",
      "description": "Semver version (FR-P01)"
    },
    "delivery": { "$ref": "#/$defs/DeliveryDescriptor" },
    "settlement": { "$ref": "#/$defs/SettlementDescriptor" },
    "pricing": { "$ref": "#/$defs/PricingDescriptor" },
    "termsRef": {
      "type": "string",
      "format": "uri",
      "description": "URI to full service terms document (FR-P04, SHOULD)"
    }
  },
  "additionalProperties": true,
  "$defs": {
    "DeliveryDescriptor": {
      "type": "object",
      "required": ["mode"],
      "properties": {
        "mode": {
          "type": "string",
          "description": "Delivery mechanism: 'p2p', 'topic', or 'custom:<type>' (FR-P01a)",
          "pattern": "^(p2p|topic|custom:[a-zA-Z0-9_-]+)$"
        },
        "serviceRef": {
          "type": "string",
          "description": "Cross-ref to neuron-p2p-exchange name (MUST when mode='p2p') (FR-P01a)"
        },
        "channelRef": {
          "type": "string",
          "description": "Cross-ref to neuron-topic name (MUST when mode='topic') (FR-P01a)"
        }
      }
    },
    "SettlementDescriptor": {
      "type": "object",
      "required": ["binding"],
      "properties": {
        "binding": {
          "type": "string",
          "description": "Settlement binding identifier (FR-P02)",
          "enum": ["hedera-native", "evm-escrow"]
        }
      },
      "additionalProperties": true
    },
    "PricingDescriptor": {
      "type": "object",
      "required": ["amount", "currency", "unit", "interval"],
      "properties": {
        "amount": { "type": "string", "description": "Decimal amount (FR-P03)" },
        "currency": { "type": "string", "description": "Currency symbol (FR-P03)" },
        "unit": { "type": "string", "description": "Denomination (FR-P03)" },
        "interval": { "type": "string", "description": "Billing interval in seconds; '0' = one-time (FR-P03)" }
      }
    }
  }
}
```

## Example (P2P delivery, EVM escrow)

```json
{
  "type": "neuron-commerce",
  "name": "adsb-v0.1",
  "version": "1.0.0",
  "delivery": {
    "mode": "p2p",
    "serviceRef": "p2p-adsb"
  },
  "settlement": {
    "binding": "evm-escrow",
    "chainId": "296",
    "contract": "0xEscrowContract...",
    "token": "0xUSDC..."
  },
  "pricing": {
    "amount": "10",
    "currency": "USDC",
    "unit": "token",
    "interval": "3600"
  },
  "termsRef": "https://dapp.example.com/adsb-v0.1/terms.json"
}
```

## Canonical Field Order (FR-P01)

`type` → `name` → `version` → `delivery` → `settlement` → `pricing` → `termsRef*`

## Cross-Reference Validation (FR-P01b)

- `delivery.serviceRef` MUST match `name` of existing `neuron-p2p-exchange` in same agentURI
- `delivery.channelRef` MUST match `name` of existing `neuron-topic` in same agentURI
- Invalid reference → `InvalidDeliveryRef` error
