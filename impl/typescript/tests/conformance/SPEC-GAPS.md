# Spec Gaps Discovered During TypeScript Implementation

## GAP-TS-001: PeerID/DID:key String Values in Test Vectors Don't Match Intermediate Hex

**Spec**: 006-protocol-determinism/contracts/test-vectors.md, Chain 1, §1.4 and §1.5
**Date**: 2026-03-17
**Severity**: Medium (intermediate hex values are correct; only display strings mismatch)

### Description

The PeerID and DID:key **string values** in Chain 1 do not match the base58btc encoding of the **intermediate hex values** in the same test vector.

**PeerID (§1.4)**:
- Test vector `multihash_hex`: `0x0025080212210279BE66...` (39 bytes, identity multihash of Secp256k1 KeyType=2)
- Test vector `peer_id`: `12D3KooWHCRh8jRUVi5aBzBSfuGJsh8jLEMM63RVUipMggsMEfRo`
- Base58btc decode of `peer_id`: `0x0024080112206da892...` (38 bytes, Ed25519 KeyType=1) — **different bytes**
- Base58btc encode of `multihash_hex`: `16Uiu2HAm3cuhhRL2msUuLF62KRSfneFDx94RsuouyW25Ho42cFMq` — **different string**

**DID:key (§1.5)**:
- Test vector `multicodec_hex`: `0xE7010279BE667EF9DC...` (35 bytes)
- Test vector `did_key`: `did:key:zQ3shZc2PiSn2RAhidVQ5C7JkZiimjC4bMU6pDr4eV45sWAkp`
- Base58btc decode of DID:key value starts with `0xE7 0x01 0x02 0xB5` — **0xB5 ≠ 0x79** (our key's X coordinate starts with 0x79)

### Root Cause

The PeerID/DID:key strings appear to have been generated from a different key or using a different PeerID derivation path (e.g., Go's `go-libp2p` may use uncompressed key → SHA-256 multihash for secp256k1, while the spec algorithm §5 specifies compressed key → identity multihash).

### Impact

The TypeScript implementation correctly follows the algorithm as specified in §5 and §6. All **intermediate hex values** match the test vector exactly. Only the final base58btc string representation differs.

### Resolution

The test vector PeerID and DID:key strings need to be regenerated from the correct multihash/multicodec hex values. Until then, the TS conformance tests verify byte-level correctness of intermediate values.
