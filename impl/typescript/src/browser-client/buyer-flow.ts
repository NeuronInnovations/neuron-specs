/// <reference lib="dom" />
// Spec 012 — buyer-side state machine + control-plane message handler.
//
// Traces: FR-B14, FR-B15, FR-B16, FR-B28.

import type { Libp2p } from 'libp2p'
import type { Stream } from '@libp2p/interface'
import { TopicMessage } from '../topic/message.js'
import type { BrowserSession } from './session.js'
import type { BootstrapJSON } from './bootstrap-schema.js'
import { FramedSender, FramedReceiver, type LibP2PStreamLike } from './in-stream-channel.js'
import { eciesDecrypt } from './ecies-decrypt.js'
import { verifyInboundEnvelope } from './envelope-verify.js'
import { receiveFile, type ReceivedFile, type ReceiveStream } from './file-receive.js'
import { makeNeuronError, NeuronBrowserCode } from './errors.js'

export interface LedgerEntry {
  direction: 'inbound' | 'outbound'
  messageType: string
  senderAddress: string
  timestampNs: bigint
  signatureStatus: 'verified' | 'self-signed' | 'failed'
  payloadHashHex: string
}

export interface BuyerFlowDeps {
  readonly session: BrowserSession
  readonly bootstrap: BootstrapJSON
  readonly libp2p: Libp2p
  readonly controlStream: Stream
  readonly onLedger: (entry: LedgerEntry) => void
}

function utf8(s: string): Uint8Array { return new TextEncoder().encode(s) }

async function keccakHex(bytes: Uint8Array): Promise<string> {
  // dynamic import keeps the initial bundle small
  const { keccak_256 } = await import('@noble/hashes/sha3')
  return Array.from(keccak_256(bytes), (b) => b.toString(16).padStart(2, '0')).join('')
}

/**
 * Execute the 4-message flow. Resolves with the received file on success.
 */
export async function runBuyerFlow(deps: BuyerFlowDeps): Promise<ReceivedFile> {
  const { session, bootstrap, libp2p, controlStream, onLedger } = deps

  const sender = new FramedSender(controlStream as unknown as LibP2PStreamLike)
  let nextSeqNum = 1n
  let prevSellerSeq: bigint | null = null

  const sendEnvelope = async (payloadObj: Record<string, unknown>, type: string): Promise<void> => {
    const payload = utf8(JSON.stringify(payloadObj))
    const env = TopicMessage.create(
      session.createPrivateKey(),
      BigInt(Date.now()) * 1_000_000n,
      nextSeqNum++,
      payload,
    )
    sender.sendEnvelope(utf8(env.toCanonicalJson()))
    onLedger({
      direction: 'outbound',
      messageType: type,
      senderAddress: env.senderAddress,
      timestampNs: env.timestamp,
      signatureStatus: 'self-signed',
      payloadHashHex: await keccakHex(env.payload),
    })
  }

  // Shared promise that resolves when we see `connectionSetup` and have the
  // decrypted multiaddr for the data stream.
  let resolveDataAddr: (addr: string) => void = () => { /* */ }
  let rejectFlow: (err: Error) => void = () => { /* */ }
  const dataAddrPromise = new Promise<string>((r, j) => {
    resolveDataAddr = r
    rejectFlow = j
  })

  const _receiver = new FramedReceiver(
    controlStream as unknown as LibP2PStreamLike,
    (envelopeBytes): void => {
      void (async (): Promise<void> => {
        try {
          const env = parseEnvelopeFromJson(envelopeBytes)
          verifyInboundEnvelope(env, {
            expectedSenderAddress: bootstrap.sellerEVMAddress,
            prevSenderSeqNum: prevSellerSeq,
            nowMs: Date.now(),
          })
          prevSellerSeq = env.sequenceNumber
          const payload = JSON.parse(new TextDecoder().decode(env.payload)) as Record<string, unknown>
          const type = String(payload.type)
          onLedger({
            direction: 'inbound',
            messageType: type,
            senderAddress: env.senderAddress,
            timestampNs: env.timestamp,
            signatureStatus: 'verified',
            payloadHashHex: await keccakHex(env.payload),
          })
          if (type === 'connectionSetup') {
            const encryptedMultiaddrs = String(payload.encryptedMultiaddrs)
            const plain = eciesDecrypt(encryptedMultiaddrs, session.exportPrivateKeyForECIES())
            const addrs = JSON.parse(new TextDecoder().decode(plain)) as string[]
            const chosen = addrs[0] ?? bootstrap.sellerWSSMultiaddr
            resolveDataAddr(chosen)
          }
        } catch (err) {
          rejectFlow(err instanceof Error ? err : new Error(String(err)))
        }
      })()
    },
    (err) => rejectFlow(err),
  )

  // Register the data-stream handler BEFORE sending serviceRequest.
  // The seller opens the data stream back to us via multistream-select right
  // after emitting connectionSetup, so our handler must be in place first or
  // the seller's dial is rejected.
  let resolveDataStream: (s: Stream) => void = () => { /* */ }
  let rejectDataStream: (e: Error) => void = () => { /* */ }
  const dataStreamPromise = new Promise<Stream>((resolve, reject) => {
    resolveDataStream = resolve
    rejectDataStream = reject
  })
  try {
    await libp2p.handle(bootstrap.dataStreamProtocolID, (stream) => {
      console.log('[neuron-012] data stream opened by seller')
      resolveDataStream(stream)
    })
    console.log('[neuron-012] registered data-stream handler for', bootstrap.dataStreamProtocolID)
  } catch (err) {
    throw makeNeuronError(
      NeuronBrowserCode.TRANSPORT_CONNECT_REFUSED,
      `failed to register data-stream handler: ${(err as Error).message}`,
      err,
    )
  }

  // 1. buyer → seller: serviceRequest
  console.log('[neuron-012] sending serviceRequest')
  await sendEnvelope(
    {
      type: 'serviceRequest',
      service: 'jpeg-demo',
      buyerAddress: session.identity.evmAddress,
      buyerPubKeyHex: session.identity.compressedPublicKeyHex,
    },
    'serviceRequest',
  )

  // Wait for seller's paymentDetails + connectionSetup and produce the data-addr.
  console.log('[neuron-012] awaiting connectionSetup from seller')
  const dataMultiaddr = await dataAddrPromise
  if (!dataMultiaddr) {
    throw makeNeuronError(NeuronBrowserCode.ENVELOPE_MALFORMED, 'no multiaddr decrypted from connectionSetup')
  }
  console.log('[neuron-012] connectionSetup received; decrypted multiaddr =', dataMultiaddr)

  // Now start the 15s timer SOLELY for the seller's dial-back (not the full flow).
  console.log('[neuron-012] waiting for seller to open data stream (15s budget)')
  const dialTimer = setTimeout(() => {
    rejectDataStream(
      makeNeuronError(
        NeuronBrowserCode.READ_IDLE_TIMEOUT,
        'seller did not open data stream within 15s after connectionSetup',
      ),
    )
  }, 15_000)
  let dataStream: Stream
  try {
    dataStream = await dataStreamPromise
  } finally {
    clearTimeout(dialTimer)
  }

  // Receive file
  const filePromise = receiveFile(dataStream as unknown as ReceiveStream)
  const file = await filePromise

  // 4. buyer → seller: invoiceAck
  await sendEnvelope(
    {
      type: 'invoiceAck',
      receivedSha256Hex: file.metadata.sha256Hex,
    },
    'invoiceAck',
  )

  return file
}

function parseEnvelopeFromJson(envelopeBytes: Uint8Array): TopicMessage {
  const doc = JSON.parse(new TextDecoder().decode(envelopeBytes)) as {
    senderAddress: string
    signature: string
    timestamp: string
    sequenceNumber: string
    payload: string
  }
  return TopicMessage.fromFields(
    doc.senderAddress,
    base64Decode(doc.signature),
    BigInt(doc.timestamp),
    BigInt(doc.sequenceNumber),
    base64Decode(doc.payload),
  )
}

function base64Decode(b64: string): Uint8Array {
  const binary = atob(b64)
  const out = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) out[i] = binary.charCodeAt(i)
  return out
}
