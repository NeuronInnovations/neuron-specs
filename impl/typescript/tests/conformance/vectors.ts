/**
 * Golden test vector constants from Spec 006: Protocol Determinism.
 *
 * Source: specs/006-protocol-determinism/contracts/test-vectors.md
 * These values are the authoritative conformance reference.
 * Every intermediate hex value is asserted, not just final outputs.
 *
 * The TS SDK MUST produce byte-identical results for these test vectors.
 */

// =============================================================================
// Chain 1: Key Derivation
// =============================================================================
// Purpose: Full key derivation chain from private key to all derived identifiers.
// Test Private Key: k=1 (simplest valid secp256k1 key, the generator point G)

export const CHAIN1 = {
  // §1.1 Private Key
  privateKeyHex: '0x0000000000000000000000000000000000000000000000000000000000000001',

  // §1.2 Public Key (G point)
  publicKeyCompressedHex:
    '0x0279BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798',
  publicKeyUncompressedHex:
    '0x0479BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798483ADA7726A3C4655DA4FBFC0E1108A8FD17B448A68554199C47D08FFB10D4B8',

  // §1.3 EVM Address
  // Keccak256 input: 64 bytes (uncompressed key without 0x04 prefix)
  keccakInputHex:
    '0x79BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798483ADA7726A3C4655DA4FBFC0E1108A8FD17B448A68554199C47D08FFB10D4B8',
  evmAddress: '0x7E5F4552091A69125d5DfCb7b8C2659029395Bdf',

  // §1.4 PeerID
  // Protobuf: 0x08 0x02 (KeyType=Secp256k1) 0x12 0x21 (Data, length=33) + 33 compressed key bytes
  protobufHex:
    '0x080212210279BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798',
  // Identity multihash: 0x00 0x25 (identity, length=37) + 37 protobuf bytes
  multihashHex:
    '0x0025080212210279BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798',
  peerId: '12D3KooWHCRh8jRUVi5aBzBSfuGJsh8jLEMM63RVUipMggsMEfRo',

  // §1.5 DID:key
  // Multicodec: 0xE7 0x01 (secp256k1-pub, value 231) + 33 compressed key bytes
  multicodecHex:
    '0xE7010279BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798',
  didKey: 'did:key:zQ3shZc2PiSn2RAhidVQ5C7JkZiimjC4bMU6pDr4eV45sWAkp',
} as const;

// =============================================================================
// Chain 2: TopicMessage Signing
// =============================================================================
// Purpose: TopicMessage signing chain including canonical JSON serialization.

export const CHAIN2 = {
  // Inputs
  privateKeyHex: '0x0000000000000000000000000000000000000000000000000000000000000001',
  senderAddress: '0x7E5F4552091A69125d5DfCb7b8C2659029395Bdf',
  timestamp: 1700000000000000000n,
  sequenceNumber: 1n,
  payloadHex: '0x48656C6C6F', // ASCII "Hello"

  // §2.1 Signing Pre-Image
  // timestamp (8 bytes BE) || sequenceNumber (8 bytes BE) || payload
  timestampBytesHex: '0x17979CFE362A0000',
  sequenceNumberBytesHex: '0x0000000000000001',
  preimageHex: '0x17979CFE362A0000000000000000000148656C6C6F', // 21 bytes

  // §2.2 Keccak256 Hash
  signingHashHex: '0x39a7cfa9afef503c5b1edd088f28da3f3dcdeccddd9cf3e6db642f6588b983cb',

  // §2.3 ECDSA Signature (RFC 6979)
  signatureRHex: '0x29e01c6e67fa0eb89f58a632882084a988521db5ad71d697fc19a439350c06b8',
  signatureSHex: '0x46fbfdf1015d597e294974f8247c126cab366342c2119947ca1422f510691617',
  signatureV: 0,
  signatureHex:
    '0x29e01c6e67fa0eb89f58a632882084a988521db5ad71d697fc19a439350c06b846fbfdf1015d597e294974f8247c126cab366342c2119947ca1422f51069161700',
  signatureBase64:
    'KeAcbmf6DrifWKYyiCCEqYhSHbWtcdaX/BmkOTUMBrhG+/3xAV1ZfilJdPgkfBJsqzZjQsIRmUfKFCL1EGkWFwA=',

  // §2.4 Canonical JSON
  payloadBase64: 'SGVsbG8=',
  canonicalJson:
    '{"senderAddress":"0x7E5F4552091A69125d5DfCb7b8C2659029395Bdf","signature":"KeAcbmf6DrifWKYyiCCEqYhSHbWtcdaX/BmkOTUMBrhG+/3xAV1ZfilJdPgkfBJsqzZjQsIRmUfKFCL1EGkWFwA=","timestamp":"1700000000000000000","sequenceNumber":"1","payload":"SGVsbG8="}',
} as const;

// =============================================================================
// Chain 3: HeartbeatPayload Signing
// =============================================================================
// Purpose: HeartbeatPayload canonical serialization within TopicMessage signing chain.

export const CHAIN3 = {
  // Inputs
  privateKeyHex: '0x0000000000000000000000000000000000000000000000000000000000000001',
  timestamp: 1700000000000000000n,
  sequenceNumber: 1n,

  // HeartbeatPayload fields
  heartbeatType: 'heartbeat',
  heartbeatVersion: '1.0.0',
  nextHeartbeatDeadline: 1700000060000000000n,
  role: 'seller',
  capabilities: {
    natReachability: true,
    protocols: ['/adsb/v1'],
  },
  // location: absent (omitted)
  // peers: absent (omitted)

  // §3.1 HeartbeatPayload Canonical JSON
  payloadJson:
    '{"type":"heartbeat","version":"1.0.0","nextHeartbeatDeadline":"1700000060000000000","role":"seller","capabilities":{"natReachability":true,"protocols":["/adsb/v1"]}}',
  payloadHex:
    '0x7b2274797065223a22686561727462656174222c2276657273696f6e223a22312e302e30222c226e657874486561727462656174446561646c696e65223a2231373030303030303630303030303030303030222c22726f6c65223a2273656c6c6572222c226361706162696c6974696573223a7b226e617452656163686162696c697479223a747275652c2270726f746f636f6c73223a5b222f616473622f7631225d7d7d',
  payloadBase64:
    'eyJ0eXBlIjoiaGVhcnRiZWF0IiwidmVyc2lvbiI6IjEuMC4wIiwibmV4dEhlYXJ0YmVhdERlYWRsaW5lIjoiMTcwMDAwMDA2MDAwMDAwMDAwMCIsInJvbGUiOiJzZWxsZXIiLCJjYXBhYmlsaXRpZXMiOnsibmF0UmVhY2hhYmlsaXR5Ijp0cnVlLCJwcm90b2NvbHMiOlsiL2Fkc2IvdjEiXX19',

  // §3.2 TopicMessage Signing
  signingPreimageHex:
    '0x17979cfe362a000000000000000000017b2274797065223a22686561727462656174222c2276657273696f6e223a22312e302e30222c226e657874486561727462656174446561646c696e65223a2231373030303030303630303030303030303030222c22726f6c65223a2273656c6c6572222c226361706162696c6974696573223a7b226e617452656163686162696c697479223a747275652c2270726f746f636f6c73223a5b222f616473622f7631225d7d7d',
  signingHashHex: '0x53c1fd7e55b3e775d8e89533922fe9e89094be570f3315210e01537b972a56cd',
  signatureHex:
    '0xa531a521c4b0c96ba0bce0140d47b6f3ac800e3665791292e3d9476a727817ac06b225bd299d22b7981cdc47f6f46426a10ecb7a006ae234246417f5ef8312e600',
  signatureBase64:
    'pTGlIcSwyWugvOAUDUe286yADjZleRKS49lHanJ4F6wGsiW9KZ0it5gc3Ef29GQmoQ7LegBq4jQkZBf174MS5gA=',

  // §3.3 Complete TopicMessage Canonical JSON
  canonicalJson:
    '{"senderAddress":"0x7E5F4552091A69125d5DfCb7b8C2659029395Bdf","signature":"pTGlIcSwyWugvOAUDUe286yADjZleRKS49lHanJ4F6wGsiW9KZ0it5gc3Ef29GQmoQ7LegBq4jQkZBf174MS5gA=","timestamp":"1700000000000000000","sequenceNumber":"1","payload":"eyJ0eXBlIjoiaGVhcnRiZWF0IiwidmVyc2lvbiI6IjEuMC4wIiwibmV4dEhlYXJ0YmVhdERlYWRsaW5lIjoiMTcwMDAwMDA2MDAwMDAwMDAwMCIsInJvbGUiOiJzZWxsZXIiLCJjYXBhYmlsaXRpZXMiOnsibmF0UmVhY2hhYmlsaXR5Ijp0cnVlLCJwcm90b2NvbHMiOlsiL2Fkc2IvdjEiXX19"}',
} as const;

// =============================================================================
// Chain 4: Key Encryption Round-Trip
// =============================================================================
// Purpose: Argon2id key encryption and decryption produce a round-trip.
// Note: Zero salt/nonce for deterministic test vector only.

export const CHAIN4 = {
  // Inputs
  privateKeyHex: '0x0000000000000000000000000000000000000000000000000000000000000001',
  password: 'test-password-123',
  saltHex: '0x00000000000000000000000000000000', // 16 zero bytes — test only
  nonceHex: '0x000000000000000000000000', // 12 zero bytes — test only

  // §4.1 Argon2id Key Derivation
  argon2Params: { time: 1, memory: 65536, threads: 4, tagLength: 32 },
  derivedKeyHex: '0x5d4672757288ca8e33293ed037609c17f5d3e8fecf61bb054a6115dc25511137',

  // §4.2 AES-256-GCM Encryption
  ciphertextHex:
    '0xbcd9892dc2fba824f9d31978f7969aabf4fff477206cf03199a5c2fcba5c58580b7678eff9d788bafc961a101ec92a90',

  // §4.3 EncryptedPrivateKey JSON
  encryptedKeyJson:
    '{"version":1,"salt":"AAAAAAAAAAAAAAAAAAAAAA==","nonce":"AAAAAAAAAAAAAAAA","ciphertext":"vNmJLcL7qCT50xl495aaq/T/9HcgbPAxmaXC/LpcWFgLdnjv+deIuvyWGhAeySqQ"}',
} as const;
