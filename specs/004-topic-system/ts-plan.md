# Implementation Plan: Topic System (TypeScript)

**Date**: 2026-03-17 | **Spec**: [spec.md](spec.md)
**Language**: TypeScript | **Derivation Source**: Language-neutral artifacts ONLY

## Summary

Spec 004 defines a unified topic system for Neuron agent communication. TopicMessage is the signed envelope (senderAddress, signature, timestamp, sequenceNumber, payload) with deterministic JSON serialization. TopicAdapter is the pluggable backend interface (HCS, ERC-log, Kafka). Channel roles (stdIn/stdOut/stdErr) and service schemas enable peer discovery via EIP-8004.

**Conformance Gates**: Chain 2 (TopicMessage signing) and Chain 3 (HeartbeatPayload signing) test vectors.

## Technical Context

**Dependencies**: `@neuron-sdk/keylib` (NeuronPrivateKey, Signature, EVMAddress, PeerID), wire utilities
**Key TS Adaptations**:
- TopicMessage: canonical JSON via wire serializer (field order: senderAddress→signature→timestamp→sequenceNumber→payload)
- Subscribe: `AsyncGenerator<MessageDelivery>` pattern
- UnsignedInt64: `bigint` with FR-W02 string encoding
- Adapters: interface-based, runtime registration

## Constitution Check

All 11 gates PASS. Constitution VIII (HCS primary transport) noted — HCS adapter is first but not blocking for core TopicMessage implementation.

## Source Structure

```text
impl/typescript/src/topic/
├── index.ts              # Public exports
├── message.ts            # TopicMessage — signing, canonical JSON, pre-image, verification
├── topic-ref.ts          # TopicRef — (transport, locator) pair, URI parsing
├── adapter.ts            # TopicAdapter interface + adapter registry
├── channel.ts            # ChannelRole enum (stdIn, stdOut, stdErr, custom:*)
├── publish-result.ts     # PublishResult type
├── message-delivery.ts   # MessageDelivery type
├── service.ts            # NeuronTopicService, NeuronP2PExchangeService parsing
├── errors.ts             # TopicError, NEURON-TOPIC-001..010
└── types.ts              # BackendKind, TransportConfig, ConfirmationMode
```

## Phases

### Phase 1: Foundational — Types + Errors
TopicError (10 codes), BackendKind, ChannelRole, ConfirmationMode enums.

### Phase 2: US1 — TopicMessage Core (P1) 🎯 MVP
TopicMessage envelope, signing pre-image, Keccak256 hash, canonical JSON, verification.
**Conformance Gate**: Chain 2 test vectors MUST pass.

### Phase 3: US2 — TopicRef + Adapter Interface (P2)
TopicRef, TopicAdapter interface, adapter registry, PublishResult, MessageDelivery.

### Phase 4: US3 — Service Schemas (P3)
NeuronTopicService, NeuronP2PExchangeService parsing from agentURI JSON.

### Phase 5: Polish
Integration tests, Chain 3 conformance (requires health module payload — deferred to health spec).
