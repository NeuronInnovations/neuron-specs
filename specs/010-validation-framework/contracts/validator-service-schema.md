# Contract: Validator Service Schema

## neuron-validator Service Type

The `neuron-validator` service object follows the same agentURI service pattern as `neuron-topic` (004 FR-T14), `neuron-p2p-exchange` (004 FR-T17), and `neuron-commerce` (008 FR-P01).

### Schema

```json
{
  "type": "neuron-validator",
  "name": "validation",
  "version": "1.0.0",
  "domains": ["005-health", "008-commerce"],
  "verdictDelivery": "topic"
}
```

### Fields

| Field | Type | Req | Description | Values |
|-------|------|-----|-------------|--------|
| `type` | string | MUST | Service type discriminator | Always `"neuron-validator"` |
| `name` | string | MUST | Service name | Free-form, e.g. `"validation"` |
| `version` | string | MUST | Service version | Semver, e.g. `"1.0.0"` |
| `domains` | string[] | MUST | Spec references or domain tags | e.g. `["005-health"]`, `["aviation"]` |
| `verdictDelivery` | string | MUST | Verdict publication mechanism | `"topic"` (stdOut publication) |

### Domain Values

The `domains` array uses the same format as the `specRef` field in evidence envelopes (FR-V24):

- **Standard spec references**: `"NNN-short-name"` matching spec directory names (e.g. `"005-health"`, `"008-payment"`)
- **Custom domain strings**: Application-specific validation domains (e.g. `"aviation"`, `"adsb-timing"`)

### Example: Full agentURI with neuron-validator

```json
{
  "services": [
    {
      "type": "neuron-topic",
      "name": "stdIn",
      "version": "1.0.0",
      "channel": "stdIn",
      "transport": "hcs",
      "anchor": "hedera-mainnet",
      "config": { "topicId": "0.0.12345" }
    },
    {
      "type": "neuron-topic",
      "name": "stdOut",
      "version": "1.0.0",
      "channel": "stdOut",
      "transport": "hcs",
      "anchor": "hedera-mainnet",
      "config": { "topicId": "0.0.12346" }
    },
    {
      "type": "neuron-topic",
      "name": "stdErr",
      "version": "1.0.0",
      "channel": "stdErr",
      "transport": "hcs",
      "anchor": "hedera-mainnet",
      "config": { "topicId": "0.0.12347" }
    },
    {
      "type": "neuron-p2p-exchange",
      "name": "p2p",
      "version": "1.0.0",
      "peerID": "12D3KooWA1b2c3D...",
      "protocol": "/neuron/multiaddr-exchange/1.0.0",
      "topicRef": "stdIn"
    },
    {
      "type": "neuron-validator",
      "name": "validation",
      "version": "1.0.0",
      "domains": ["005-health"],
      "verdictDelivery": "topic"
    }
  ]
}
```

### Relationship to Other Service Types

- `neuron-validator` is an **additional** service — it does not replace the mandatory services (three `neuron-topic` + one `neuron-p2p-exchange` per 003 FR-R02)
- A validator's stdOut topic (declared via `neuron-topic` service with `channel: "stdOut"`) is where evidence envelopes are published
- The `verdictDelivery: "topic"` field confirms the evidence publication channel

### FR Traceability

- FR-V11: Validators register as standard agents with neuron-validator service
- FR-V12: Service object fields (type, name, version, domains, verdictDelivery)
- FR-V15: Mandatory services (stdIn, stdOut, stdErr, p2p-exchange) still required
- FR-V24: specRef/domain format
