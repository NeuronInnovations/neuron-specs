/// <reference lib="dom" />
// Spec 012 — browser-side file receiver.
// Feeds a libp2p data stream through the shared frame decoder:
// - frame 0 parsed as metadata
// - frames 1..N concatenated into the payload buffer
// - running total-bytes check + SHA-256 + read-idle timeout.
//
// Traces: FR-B17, FR-B19, FR-B20, FR-B21, FR-B22, FR-B34.

import { sha256 } from '@noble/hashes/sha256'
import { FrameDecoder } from '../wire/frame.js'
import { MAX_TOTAL_BYTES, READ_IDLE_MS } from './constants.js'
import { parseFrameZero, type FileMetadata } from './file-metadata.js'
import { makeNeuronError, NeuronBrowserCode, NeuronBrowserError } from './errors.js'

export interface ReceivedFile {
  readonly metadata: FileMetadata
  readonly bytes: Uint8Array
}

export interface ReceiveStream {
  addEventListener(
    type: 'message',
    handler: (evt: { data: Uint8Array | { subarray: () => Uint8Array } }) => void,
  ): void
  addEventListener(type: 'close', handler: (evt?: unknown) => void): void
}

export function receiveFile(stream: ReceiveStream): Promise<ReceivedFile> {
  return new Promise((resolve, reject) => {
    let metadata: FileMetadata | null = null
    const chunks: Uint8Array[] = []
    let total = 0
    let idleTimer: ReturnType<typeof setTimeout> | null = null
    let settled = false

    const finish = (outcome: ReceivedFile | Error): void => {
      if (settled) return
      settled = true
      if (idleTimer) clearTimeout(idleTimer)
      if (outcome instanceof Error) reject(outcome)
      else resolve(outcome)
    }

    const resetIdle = (): void => {
      if (idleTimer) clearTimeout(idleTimer)
      idleTimer = setTimeout(() => {
        finish(
          makeNeuronError(
            NeuronBrowserCode.READ_IDLE_TIMEOUT,
            `no data for ${READ_IDLE_MS}ms`,
          ),
        )
      }, READ_IDLE_MS)
    }

    const decoder = new FrameDecoder((payload) => {
      try {
        if (payload.length === 0) return // keep-alive
        if (metadata === null) {
          metadata = parseFrameZero(payload)
          return
        }
        if (total + payload.length > metadata.sizeBytes) {
          throw makeNeuronError(
            NeuronBrowserCode.CHUNK_TOTAL_MISMATCH,
            `payload exceeds declared sizeBytes ${metadata.sizeBytes}`,
          )
        }
        chunks.push(payload)
        total += payload.length
        if (total === metadata.sizeBytes) {
          // Finalise + verify.
          const full = new Uint8Array(total)
          let offset = 0
          for (const c of chunks) {
            full.set(c, offset)
            offset += c.length
          }
          const computedHash = Array.from(sha256(full), (b) => b.toString(16).padStart(2, '0')).join('')
          if (computedHash !== metadata.sha256Hex) {
            throw makeNeuronError(
              NeuronBrowserCode.FILE_SHA256_MISMATCH,
              `sha256 mismatch: computed ${computedHash}, metadata ${metadata.sha256Hex}`,
            )
          }
          finish({ metadata, bytes: full })
        }
      } catch (err) {
        if (err instanceof NeuronBrowserError) finish(err)
        else finish(makeNeuronError(NeuronBrowserCode.ENVELOPE_MALFORMED, String(err), err))
      }
    })

    stream.addEventListener('message', (evt) => {
      if (settled) return
      resetIdle()
      const raw = evt.data
      const bytes =
        raw && typeof (raw as { subarray?: () => Uint8Array }).subarray === 'function'
          ? (raw as { subarray: () => Uint8Array }).subarray()
          : (raw as Uint8Array)
      try {
        decoder.push(bytes)
      } catch (err) {
        finish(
          err instanceof NeuronBrowserError
            ? err
            : makeNeuronError(NeuronBrowserCode.FRAME_SIZE_EXCEEDED, String(err), err),
        )
      }
    })
    stream.addEventListener('close', () => {
      if (settled) return
      finish(
        makeNeuronError(
          NeuronBrowserCode.TRANSPORT_STREAM_CLOSED_UNEXPECTEDLY,
          `data stream closed before file fully received (${total}/${metadata?.sizeBytes ?? '?'} bytes)`,
        ),
      )
    })

    resetIdle()
  })
}

// Re-export the constant for consumers who want to range-check at the caller.
export { MAX_TOTAL_BYTES }
