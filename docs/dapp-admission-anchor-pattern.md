# DApp Admission-Policy Anchor Pattern

> **Authority**: Constitution Principle XII (v1.7.0). This document is informative; it elaborates the boundary test in XII for one specific DApp design decision (where AdmissionPolicy data lives) without amending the principle itself.
> **Created**: 2026-05-08
> **Status**: Normative-informative. Future DApp specs that implement an `AdmissionPolicy` (per 008 FR-P55–P57) MUST consult this document and cite the anchor backend they chose.

## Purpose

Spec 008 FR-P55–P57 defines a small, declarative `AdmissionPolicy` interface in the Core SDK. The interface returns one of `"allow-direct"`, `"allow-via-fanout"`, or `"deny"` per buyer per service. Core SDK ships only one default implementation: `AllowAll`. Concrete admission semantics — partner allowlists, denylists, priority lists, per-tenant rules — are DApp choices per Constitution Principle XII (FR-P57 explicitly forbids them in Core SDK).

This document tells DApp authors **where** the data backing those concrete policies lives. Three approved anchor backends are defined below. A DApp's `AdmissionPolicy` implementation reads from one of them (or migrates between them as the deployment matures).

## Anchor A1 — Deployment-config static

**Description**: The partner allowlist, denylist, and priority list live in a configuration file or environment variables on the seller. Updates require a process restart (or, optionally, a SIGHUP reload).

**Suitable for**:
- June 2026 reference demo.
- Small private deployments where the operator owns both seller and partner-buyer infrastructure.
- Bootstrapping new DApps before the EIP-8004 service-metadata schema for the policy reference is finalized.

**How to implement**:
- The DApp's `AdmissionPolicy` implementation reads a config struct populated from `internal/edgeapp/config.go` (or equivalent in non-Go reference impls).
- The config struct MAY be populated from a YAML/TOML file, environment variables, or command-line flags.
- The implementation MUST log a warning if the policy is empty (no allowlist / no denylist) — this often indicates a misconfigured deployment rather than an intentional `AllowAll`.

**Trade-offs**:
- ✅ Simplest possible backend; no external dependencies.
- ✅ Deployment owners retain full control; no on-chain footprint.
- ❌ No external auditability; partners cannot verify they are on the allowlist without trusting the seller's deployment.
- ❌ Updates require operational access to the seller; not suitable for federated deployments.

**Reference DApp**: 016 (ADS-B) and 017 (Remote ID) both use A1 for the reference demo per FR-A10 / FR-R09.

## Anchor A2 — EIP-8004 service metadata

**Description**: The seller publishes a structured `admissionPolicyRef` field inside one of its `services[]` entries in its agentURI (per Spec 003). The reference points to a signed policy document — typically a URL to a JSON file hosted on the seller's domain or in IPFS — containing the allowlist / denylist / priority list. Buyers fetch and verify the document **before** sending a `serviceRequest`.

**Suitable for**:
- Production deployments where buyers need pre-negotiation visibility into admission rules (e.g., to know whether to even attempt a connection).
- Federated deployments where multiple DApp providers share a partner ecosystem and want to express overlapping allowlists.

**How to implement**:
- The DApp's seller adds an `admissionPolicyRef` field to its `neuron-commerce` service entry. Recommended shape:
  ```json
  {
    "type": "neuron-commerce",
    "name": "adsb",
    ...,
    "admissionPolicyRef": {
      "url": "https://seller.example/policy/adsb.v1.json",
      "signatureRef": "https://seller.example/policy/adsb.v1.sig",
      "version": "1.0.0"
    }
  }
  ```
- The policy document is itself a signed canonical-JSON object listing allowed / denied EVMs and any priority hints. Signature is by the seller's NeuronPrivateKey (002).
- Buyers fetch the document, verify the signature against the seller's registered public key (from 003), then locally evaluate whether they are allowed.
- The seller's `AdmissionPolicy` implementation reads the same document on startup and on file-change events; the document IS the source of truth.

**Trade-offs**:
- ✅ Buyers see admission rules without trusting the seller's runtime behavior.
- ✅ Updates ride the existing agentURI update path (003 FR-R11) — semi-on-chain audit trail via the registry contract.
- ✅ Signatures provide tamper-evidence for the policy document.
- ❌ Requires URL hosting (or IPFS pinning); MORE moving parts than A1.
- ❌ Stale-document handling (buyer cached an old version) needs explicit version negotiation.
- ❌ Document signing introduces a separate key-management surface.

**Status**: A2 is approved as a forward-compatible design pattern. The exact `admissionPolicyRef` schema is informative in this document; a future minor 003 / 008 amendment MAY normatively register the field name. Until then, DApps adopting A2 SHOULD use the schema above and document the choice in their spec.

## Anchor A3 — On-chain admission registry (deployment-provided)

**Description**: An EIP-8004-style admission registry contract where sellers commit a hash of their current admission policy and buyers read the commitment before attempting negotiation. The registry contract is a sibling of the Identity / Reputation / Validation registries from 007 but governs admission specifically.

**Suitable for**:
- Regulated production deployments requiring auditable admission traces (e.g., coalition deployments with formal audit requirements).
- Multi-jurisdiction deployments where regulators require on-chain commitments for admission decisions.

**How to implement**:
- Deploy an `AdmissionRegistry.sol` contract (out of scope for this document).
- The DApp's seller calls `registerPolicy(policyHash, version, agentId)` and stores the policy document off-chain (on IPFS or similar).
- Buyers read `lookupPolicy(sellerAgentId)` to get the committed hash, fetch the off-chain document, verify the hash matches.
- The DApp's `AdmissionPolicy` implementation reads from the contract on startup and on contract-event subscription.

**Trade-offs**:
- ✅ Strongest auditability; admission changes have on-chain timestamps.
- ✅ Compatible with reputation / dispute frameworks (010 validators can attest to admission compliance).
- ❌ Highest complexity; requires contract deployment, ABI maintenance, gas costs for updates.
- ❌ The Core SDK does not provide this contract; an A3 deployment owns the contract schema and its maintenance.

**Status**: A3 is an approved design pattern. No DApp is required to use it; the deploying platform supplies the contract.

## Migration paths

### A1 → A2

1. Author the partner allowlist / denylist as a canonical-JSON document.
2. Sign the document with the seller's NeuronPrivateKey (002).
3. Host the document at a stable URL.
4. Add the `admissionPolicyRef` to the `neuron-commerce` service entry in the seller's agentURI.
5. Update the `AdmissionPolicy` implementation to read from the document instead of the deployment-config struct. Tests SHOULD verify the migration produces identical decisions to the A1 baseline.

### A2 → A3

1. Wait for the canonical `AdmissionRegistry` contract spec to land (out of scope for this document).
2. Deploy the contract or use a shared deployment.
3. Call `registerPolicy(policyHash, version, agentId)` with the hash of the existing A2 document.
4. Update the `AdmissionPolicy` implementation to read the commitment from the contract before fetching the off-chain document.
5. Continue serving the off-chain document at the same URL during the migration window so older buyers (still on A2) continue to work.

## Constitution Principle XII reference

Core SDK MUST NOT define admission policy semantics (FR-P57). This document is informative; the Core SDK helper interface (008 FR-P55–P57) remains policy-free. DApps choosing A1, A2, or A3 are following Principle XII correctly — the policy lives in DApp-controlled code or DApp-controlled data, never in Core SDK normative requirements.

A DApp spec MUST cite the anchor it chose (A1, A2, A3) in its `AdmissionPolicy` section and document the rationale. June 2026 reference demo specs (016 ADS-B, 017 Remote ID) cite A1.
