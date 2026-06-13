# API Contract: Negotiation Payloads

**Source**: spec.md FR-P06–P12, FR-P33–P35

---

## Payload Types

Six TopicMessage payload types, discriminated by `type` field (FR-P06).

All payloads:
- Carried in `TopicMessage.payload` (004 FR-T20)
- Use canonical JSON field ordering (006 FR-W05)
- Numeric fields as JSON string decimal (006 FR-W02)
- Optional fields omitted when absent (006 FR-W04)
- Version compatibility: accept `1.x.y`, reject `2.x.y` (FR-P12a)
- NOT encrypted (FR-P12b), except `connectionSetup.encryptedMultiaddrs` (FR-P34)
- Verifiable-state payloads SHOULD use WaitForConsensus (FR-P12c)

## serviceRequest

**Direction**: buyer → seller's stdIn
**Canonical order**: type → version → requestId → serviceRef → settlementBinding → proposedAmount → proposedCurrency → proposedInterval → serviceParams* → negotiationDeadline → arbiter* → buyerStdIn

```json
{
  "type": "serviceRequest",
  "version": "1.0.0",
  "requestId": "550e8400-e29b-41d4-a716-446655440000",
  "serviceRef": "adsb-v0.1",
  "settlementBinding": "evm-escrow",
  "proposedAmount": "10",
  "proposedCurrency": "USDC",
  "proposedInterval": "3600",
  "serviceParams": { "region": "ICAO:VVTS" },
  "negotiationDeadline": "1711382400",
  "buyerStdIn": "0.0.54321"
}
```

## serviceResponse

**Direction**: seller → buyer's stdIn
**Canonical order**: type → version → requestId → action → counterAmount* → counterInterval*

```json
{
  "type": "serviceResponse",
  "version": "1.0.0",
  "requestId": "550e8400-e29b-41d4-a716-446655440000",
  "action": "accept"
}
```

## connectionSetup

**Direction**: bidirectional (buyer ↔ seller)
**Canonical order**: type → version → requestId → peerID → encryptedMultiaddrs → protocol → natStatus*
**Required when**: delivery.mode = "p2p" (FR-P35)
**Encryption**: encryptedMultiaddrs field is encrypted (FR-P34); algorithm defined by Spec 009

```json
{
  "type": "connectionSetup",
  "version": "1.0.0",
  "requestId": "550e8400-e29b-41d4-a716-446655440000",
  "peerID": "12D3KooWBuyerPeerID...",
  "encryptedMultiaddrs": "base64encodedciphertext...",
  "protocol": "/neuron/adsb/1.0.0",
  "natStatus": "private"
}
```

## escrowCreated

**Direction**: buyer → seller's stdIn
**Canonical order**: type → version → requestId → escrowRef → depositAmount → depositCurrency
**Confirmation**: SHOULD use WaitForConsensus (FR-P12c)

```json
{
  "type": "escrowCreated",
  "version": "1.0.0",
  "requestId": "550e8400-e29b-41d4-a716-446655440000",
  "escrowRef": "evm-escrow:296:0xContractAddress",
  "depositAmount": "10",
  "depositCurrency": "USDC"
}
```

## invoice

**Direction**: seller → buyer's stdIn
**Canonical order**: type → version → requestId → releaseRequestRef → escrowRef → amount → currency → period
**Confirmation**: SHOULD use WaitForConsensus (FR-P12c)

```json
{
  "type": "invoice",
  "version": "1.0.0",
  "requestId": "550e8400-e29b-41d4-a716-446655440000",
  "releaseRequestRef": "evm-escrow:296:0xReleaseRef",
  "escrowRef": "evm-escrow:296:0xContractAddress",
  "amount": "10",
  "currency": "USDC",
  "period": "PT1H"
}
```

## invoiceAck

**Direction**: buyer → seller's stdIn
**Canonical order**: type → version → requestId → releaseRequestRef → action → depositedMore* → newBalance*
**Confirmation**: SHOULD use WaitForConsensus (FR-P12c)

```json
{
  "type": "invoiceAck",
  "version": "1.0.0",
  "requestId": "550e8400-e29b-41d4-a716-446655440000",
  "releaseRequestRef": "evm-escrow:296:0xReleaseRef",
  "action": "approved"
}
```
