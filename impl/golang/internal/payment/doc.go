// Package payment implements Spec 008 — Payment, the commerce protocol layer
// for Neuron agent-to-agent service transactions.
//
// # Architecture
//
// The payment package sits at the top of the SDK dependency chain:
//
//	keylib → account → topic → registry → payment
//
// It defines:
//   - Service offering types (NeuronCommerceService, DeliveryDescriptor,
//     SettlementDescriptor, PricingDescriptor) for agentURI services[]
//   - Six negotiation payload types (serviceRequest, serviceResponse,
//     connectionSetup, escrowCreated, invoice, invoiceAck) carried as
//     TopicMessage payloads
//   - An agreement lifecycle state machine (10 states, 13 transitions)
//     tracked per requestId
//   - An abstract EscrowAdapter interface (6 operations) with pluggable
//     settlement bindings (hedera-native, evm-escrow)
//   - Trust-gated engagement checks (SHOULD-level)
//   - NEURON-PAYMENT-* structured error taxonomy (16 error kinds)
//
// # Design Decisions
//
// DD-P01: Commerce service types live in this package but are consumed by
// the registry package for AgentURI serialization. The registry imports
// payment types — same pattern as topic types consumed by registry.
//
// DD-P02: Agreement state is tracked in-memory per requestId. No persistence
// layer — state is reconstructable from topic message history.
//
// DD-P03: EscrowAdapter is an interface, not a concrete implementation.
// Settlement bindings (hedera-native, evm-escrow) implement it separately.
// This follows the TopicAdapter pattern from Spec 004.
//
// DD-P04: Negotiation payloads are pure data types with canonical JSON
// serialization. Signing happens when wrapped in a TopicMessage via
// topic.NewTopicMessage — payment defines what to say, topic defines
// how to sign and send it.
//
// # Canonical JSON
//
// All payload types implement custom MarshalJSON methods that emit fields
// in the canonical order defined by Spec 008 FR-P12. Numeric fields use
// JSON string decimal per Spec 006 FR-W02. Optional fields are omitted
// when absent per FR-W04.
//
// # Spec References
//
// Spec 008: specs/008-payment/spec.md
// Data Model: specs/008-payment/data-model.md
// Contracts: specs/008-payment/contracts/
package payment
