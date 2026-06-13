// Spec 012 — in-stream TopicAdapter envelope verifier.
//
// Verifies every inbound TopicMessage against the five rules in
// contracts/in-stream-adapter.md §"Signature verification rules":
//
//   1. Signature recovery     → NEURON-BROWSER-061
//   2. Sender pinning         → NEURON-BROWSER-062
//   3. Sequence check         → NEURON-BROWSER-063
//   4. Timestamp sanity       → NEURON-BROWSER-064
//   5. Payload type check     → NEURON-BROWSER-140 (caller-supplied)
//
// Throws NeuronBrowserError on the first violation.
//
// Traces: FR-B13, FR-B14.

import { TopicMessage } from '../topic/message.js'
import { Signature } from '../keylib/signature.js'
import { NeuronPublicKey } from '../keylib/public-key.js'
import { makeNeuronError, NeuronBrowserCode } from './errors.js'

export interface VerifyOptions {
  /** Expected sender EIP-55 address (bootstrap.sellerEVMAddress). */
  readonly expectedSenderAddress: string
  /** Previous inbound sequenceNumber from this sender, or null if none yet. */
  readonly prevSenderSeqNum: bigint | null
  /** Current wall-clock time in milliseconds (for timestamp-skew check). */
  readonly nowMs: number
  /**
   * Allowed absolute clock-skew between envelope timestamp and nowMs.
   * Defaults to ±2 minutes per spec.md Assumptions.
   */
  readonly clockSkewMs?: number
}

const DEFAULT_CLOCK_SKEW_MS = 2 * 60 * 1000

/**
 * Verify a received TopicMessage. Returns the same envelope unchanged if all
 * rules pass; throws on the first rule that fails.
 */
export function verifyInboundEnvelope(
  envelope: TopicMessage,
  opts: VerifyOptions,
): TopicMessage {
  const clockSkewMs = opts.clockSkewMs ?? DEFAULT_CLOCK_SKEW_MS

  // 1. Signature recovery
  let recoveredAddress: string
  try {
    const hash = TopicMessage.hashPreimage(
      envelope.timestamp,
      envelope.sequenceNumber,
      envelope.payload,
    )
    const sig = Signature.fromBytes(envelope.signatureBytes())
    const compressedPubKey = sig.recover(hash)
    recoveredAddress = NeuronPublicKey.fromCompressedBytes(compressedPubKey)
      .evmAddress()
      .toString()
  } catch (err) {
    throw makeNeuronError(
      NeuronBrowserCode.SIGNATURE_RECOVER_MISMATCH,
      `signature recovery failed for envelope from ${envelope.senderAddress}`,
      err,
    )
  }
  if (recoveredAddress !== envelope.senderAddress) {
    throw makeNeuronError(
      NeuronBrowserCode.SIGNATURE_RECOVER_MISMATCH,
      `signature does not recover to envelope.senderAddress: recovered ${recoveredAddress}, envelope claims ${envelope.senderAddress}`,
    )
  }

  // 2. Sender pinning
  if (envelope.senderAddress !== opts.expectedSenderAddress) {
    throw makeNeuronError(
      NeuronBrowserCode.SENDER_PIN_MISMATCH,
      `envelope sender ${envelope.senderAddress} does not match pinned seller ${opts.expectedSenderAddress}`,
    )
  }

  // 3. Sequence check: monotonic strictly increasing
  if (opts.prevSenderSeqNum !== null && envelope.sequenceNumber <= opts.prevSenderSeqNum) {
    throw makeNeuronError(
      NeuronBrowserCode.SEQUENCE_DECREMENT,
      `envelope sequenceNumber ${envelope.sequenceNumber} is not strictly greater than previous ${opts.prevSenderSeqNum}`,
    )
  }

  // 4. Timestamp sanity: envelope.timestamp is in NANOSECONDS (bigint), nowMs is ms.
  const envelopeMs = Number(envelope.timestamp / 1_000_000n)
  const delta = Math.abs(envelopeMs - opts.nowMs)
  if (delta > clockSkewMs) {
    throw makeNeuronError(
      NeuronBrowserCode.TIMESTAMP_SKEW,
      `envelope timestamp skew ${delta}ms exceeds allowed ${clockSkewMs}ms`,
    )
  }

  return envelope
}
