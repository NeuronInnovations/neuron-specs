/**
 * keylib — Neuron SDK Key Library barrel exports.
 *
 * Spec reference: 002 spec.md
 *
 * Re-exports all public types, factories, and utilities from the key library.
 * Consumers should import from this module rather than individual files.
 *
 * Usage:
 *   import { NeuronPrivateKey, NeuronPublicKey, EVMAddress } from '@neuron-sdk/keylib';
 */

export { NeuronPrivateKey } from './private-key.js';
export { NeuronPublicKey } from './public-key.js';
export { EVMAddress } from './evm-address.js';
export { PeerID } from './peer-id.js';
export { DIDKey } from './did-key.js';
export { Signature } from './signature.js';
export { EncryptedPrivateKey } from './encrypted-key.js';
export type { Argon2Params } from './encrypted-key.js';
export { MultisigKey } from './multisig-key.js';
export { KeyError } from './errors.js';
export { constantTimeEqual } from './matching.js';
export * from './constants.js';
