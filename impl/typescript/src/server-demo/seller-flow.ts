// Spec 012 — seller half of the 4-message buyer flow.
//
//   1. buyer → seller  : serviceRequest
//   2. seller → buyer  : paymentDetails       (seller publishes)
//   3. seller → buyer  : connectionSetup      (ECIES-encrypted multiaddrs)
//   4. buyer → seller  : invoiceAck           (seller observes → escrow.release)
//
// After step 3, seller opens the data stream to the buyer and sends the file
// (frame 0 metadata + chunks). On step 4, escrow → released.
//
// Traces: FR-B15 (seller), FR-B16 (ECIES encrypt), FR-B18 (mock escrow).

import type { Stream } from '@libp2p/interface'
import { TopicMessage } from '../topic/message.js'
import { NeuronPrivateKey } from '../keylib/private-key.js'
import { NeuronPublicKey } from '../keylib/public-key.js'
import { FramedReceiver, FramedSender, type LibP2PStreamLike } from '../browser-client/in-stream-channel.js'
import { eciesEncrypt } from './ecies-encrypt.js'
import { MockEscrow } from './mock-escrow.js'
import type { LoadedAsset } from './file-send.js'
import { sendAsset } from './file-send.js'
import { DATA_PROTOCOL_ID } from '../browser-client/constants.js'

export interface SellerFlowDeps {
  readonly sellerPrivateKey: NeuronPrivateKey
  readonly sellerAddress: string // EIP-55
  readonly asset: LoadedAsset
  readonly escrow: MockEscrow
  readonly ownWSSMultiaddr: string
  /** Open a data stream to the peer that dialled the control stream. */
  readonly openDataStream: () => Promise<{ send: (b: Uint8Array) => boolean; close: () => Promise<void> }>
}

function utf8(s: string): Uint8Array { return new TextEncoder().encode(s) }
function hex32(): string {
  const b = crypto.getRandomValues(new Uint8Array(32))
  return Array.from(b, (x) => x.toString(16).padStart(2, '0')).join('')
}

interface ServiceRequestPayload {
  type: 'serviceRequest'
  service: string
  buyerAddress: string
  /** buyer's compressed secp256k1 pubkey, hex. Used as the ECIES recipient. */
  buyerPubKeyHex: string
}

export function handleControlStream(stream: Stream, deps: SellerFlowDeps): void {
  let nextSeqNum = 1n
  let agreementHash = ''

  const sender = new FramedSender(stream as unknown as LibP2PStreamLike)

  const emit = async (payloadObj: Record<string, unknown>): Promise<void> => {
    const now = BigInt(Date.now()) * 1_000_000n
    let env = TopicMessage.create(
      deps.sellerPrivateKey,
      now,
      nextSeqNum++,
      utf8(JSON.stringify(payloadObj)),
    )
    // TAMPER HOOK (US2 — T030). When TAMPER=sig is set and we're emitting
    // paymentDetails, flip one byte of the R component of the signature
    // before sending. The browser should reject with NEURON-BROWSER-061.
    if (process.env.TAMPER === 'sig' && payloadObj.type === 'paymentDetails') {
      const sigBytes = env.signatureBytes()
      sigBytes[0] = (sigBytes[0] ?? 0) ^ 0x01
      env = TopicMessage.fromFields(
        env.senderAddress,
        sigBytes,
        env.timestamp,
        env.sequenceNumber,
        env.payload,
      )
      console.warn('[seller-flow] TAMPER=sig: flipped paymentDetails signature byte 0')
    }
    sender.sendEnvelope(utf8(env.toCanonicalJson()))
  }

  const _receiver = new FramedReceiver(
    stream as unknown as LibP2PStreamLike,
    (envelopeBytes): void => {
      void (async (): Promise<void> => {
        const env = parseEnvelopeFromJson(envelopeBytes)
        let payload: Record<string, unknown>
        try {
          payload = JSON.parse(new TextDecoder().decode(env.payload)) as Record<string, unknown>
        } catch {
          console.warn('[seller-flow] malformed payload JSON; ignoring')
          return
        }
        const type = payload.type

        if (type === 'serviceRequest') {
          const req = payload as unknown as ServiceRequestPayload
          agreementHash = '0x' + hex32()
          deps.escrow.propose(agreementHash, 1n, deps.asset.metadata.sha256Hex)

          await emit({
            type: 'paymentDetails',
            agreementHash,
            priceAtto: '1',
            invoiceSha256Hex: deps.asset.metadata.sha256Hex,
          })

          // ECIES-encrypt our multiaddr for the buyer.
          const buyerPubCompressed = hexToBytes(req.buyerPubKeyHex)
          const multiaddrsJson = utf8(JSON.stringify([deps.ownWSSMultiaddr]))
          const encryptedMultiaddrs = eciesEncrypt(multiaddrsJson, buyerPubCompressed)

          await emit({
            type: 'connectionSetup',
            recipientEVMAddress: req.buyerAddress,
            encryptedMultiaddrs,
            streamProtocol: DATA_PROTOCOL_ID,
          })

          // Open the data stream and push the asset. The data stream is a
          // SEPARATE libp2p stream opened by the seller now that the buyer
          // has been told to expect one.
          try {
            console.log('[seller-flow] opening data stream to buyer')
            const dataStream = await deps.openDataStream()
            console.log('[seller-flow] data stream opened; sending', deps.asset.metadata.sizeBytes, 'bytes')
            sendAsset(deps.asset, dataStream)
            // Let the OS drain before we close.
            await new Promise<void>((resolve) => setTimeout(resolve, 250))
            await dataStream.close()
            console.log('[seller-flow] data stream closed gracefully')
          } catch (err) {
            console.error('[seller-flow] data stream error:', err)
          }
        } else if (type === 'invoiceAck') {
          try {
            deps.escrow.release(agreementHash)
          } catch (err) {
            console.warn('[seller-flow] escrow release failed:', err)
          }
        }
      })()
    },
    (err) => {
      // Remote abort; ok to ignore for the spike.
      console.warn('[seller-flow] control stream aborted:', err.message)
    },
  )
}

function parseEnvelopeFromJson(envelopeBytes: Uint8Array): TopicMessage {
  // The envelope is canonical JSON. We deserialize by parsing the JSON and
  // rebuilding a TopicMessage via fromFields (matches spec 004 topic-message
  // behaviour).
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

function hexToBytes(hex: string): Uint8Array {
  const clean = hex.startsWith('0x') ? hex.slice(2) : hex
  const out = new Uint8Array(clean.length / 2)
  for (let i = 0; i < out.length; i++) out[i] = parseInt(clean.slice(i * 2, i * 2 + 2), 16)
  return out
}

// Silence unused-import warnings while keeping symbols available for T018.
export type { NeuronPublicKey }
