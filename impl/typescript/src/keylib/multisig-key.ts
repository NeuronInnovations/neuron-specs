/**
 * MultisigKey — multi-signature key configuration for m-of-n threshold schemes.
 *
 * Spec reference: 002 spec.md
 *   - FR-023: MultisigKey declares a protocol identifier and m-of-n threshold.
 *   - FR-024: MultisigKey exposes protocol(), threshold(), and totalKeys().
 *
 * GAP-005: Aggregated key derivation (EVM address, PeerID) is not yet available
 * for any protocol. All calls to evmAddress() and peerId() throw NEURON-KEY-002.
 *
 * Immutable value type. Valid by construction.
 */

import { unsupportedKeyType, invalidFormat } from './errors.js';

/**
 * A multi-signature key configuration describing an m-of-n threshold scheme.
 *
 * FR-023: Declares a protocol identifier (e.g., "secp256k1-aggregated",
 * "hedera-threshold") and the m-of-n threshold parameters.
 * FR-024: Provides accessors for protocol, threshold (m), and totalKeys (n).
 *
 * GAP-005: evmAddress() and peerId() throw for all protocols because
 * aggregated key derivation is not yet specified.
 */
export class MultisigKey {
  private readonly _protocol: string;
  private readonly _threshold: number;
  private readonly _totalKeys: number;

  /** @internal — use {@link fromConfig} factory instead. */
  private constructor(protocol: string, threshold: number, totalKeys: number) {
    this._protocol = protocol;
    this._threshold = threshold;
    this._totalKeys = totalKeys;
  }

  /**
   * Construct a MultisigKey from a protocol identifier and m-of-n parameters.
   *
   * FR-023: Validates threshold >= 1 and threshold <= totalKeys.
   * FR-024: The resulting instance exposes protocol(), threshold(), totalKeys().
   *
   * @param protocol - Protocol identifier (e.g., "secp256k1-aggregated", "hedera-threshold")
   * @param threshold - Minimum number of signers required (m)
   * @param totalKeys - Total number of key holders (n)
   * @returns MultisigKey instance
   * @throws KeyError NEURON-KEY-001 if threshold < 1 or threshold > totalKeys
   */
  static fromConfig(protocol: string, threshold: number, totalKeys: number): MultisigKey {
    if (threshold < 1) {
      throw invalidFormat(
        `Threshold must be >= 1, got ${threshold.toString()}`,
      );
    }
    if (threshold > totalKeys) {
      throw invalidFormat(
        `Threshold (${threshold.toString()}) must be <= totalKeys (${totalKeys.toString()})`,
      );
    }

    return new MultisigKey(protocol, threshold, totalKeys);
  }

  /**
   * Return the protocol identifier for this multisig configuration.
   *
   * FR-024: e.g., "secp256k1-aggregated" or "hedera-threshold".
   *
   * @returns Protocol identifier string
   */
  protocol(): string {
    return this._protocol;
  }

  /**
   * Return the minimum number of signers required (m).
   *
   * FR-024: The threshold value in the m-of-n scheme.
   *
   * @returns Threshold count
   */
  threshold(): number {
    return this._threshold;
  }

  /**
   * Return the total number of key holders (n).
   *
   * FR-024: The total key count in the m-of-n scheme.
   *
   * @returns Total keys count
   */
  totalKeys(): number {
    return this._totalKeys;
  }

  /**
   * EVM address derivation is not supported for multisig keys.
   *
   * GAP-005: Aggregated key derivation is not yet available for any protocol.
   * All MultisigKey protocols throw NEURON-KEY-002.
   *
   * @throws KeyError NEURON-KEY-002 always
   */
  evmAddress(): never {
    throw unsupportedKeyType(
      `EVM address derivation is not supported for multisig protocol "${this._protocol}" (GAP-005)`,
    );
  }

  /**
   * PeerID derivation is not supported for multisig keys.
   *
   * GAP-005: Aggregated key derivation is not yet available for any protocol.
   * All MultisigKey protocols throw NEURON-KEY-002.
   *
   * @throws KeyError NEURON-KEY-002 always
   */
  peerId(): never {
    throw unsupportedKeyType(
      `PeerID derivation is not supported for multisig protocol "${this._protocol}" (GAP-005)`,
    );
  }
}
