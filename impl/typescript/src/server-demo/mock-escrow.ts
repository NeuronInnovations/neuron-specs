// Spec 012 — seller-only in-memory mock escrow.
// Phase 1 scope: v1 exercises only `proposed → released`. Full state-machine
// fidelity is deferred to H1 (Go-seller interop + on-chain escrow).
//
// Traces: FR-B18, research.md R0.6.

export type EscrowState = 'proposed' | 'funded' | 'released' | 'refunded'

interface Entry {
  state: EscrowState
  priceAtto: bigint
  invoiceSha256Hex: string
}

export class MockEscrow {
  private readonly entries = new Map<string, Entry>()

  propose(agreementHash: string, priceAtto: bigint, invoiceSha256Hex: string): void {
    if (this.entries.has(agreementHash)) {
      throw new Error(`escrow already proposed for agreementHash ${agreementHash}`)
    }
    this.entries.set(agreementHash, { state: 'proposed', priceAtto, invoiceSha256Hex })
  }

  state(agreementHash: string): EscrowState | undefined {
    return this.entries.get(agreementHash)?.state
  }

  release(agreementHash: string): void {
    const e = this.entries.get(agreementHash)
    if (!e) throw new Error(`no such escrow: ${agreementHash}`)
    if (e.state === 'released') return // idempotent
    if (e.state !== 'proposed') {
      throw new Error(`cannot release from state ${e.state}`)
    }
    e.state = 'released'
  }

  refund(agreementHash: string): void {
    const e = this.entries.get(agreementHash)
    if (!e) throw new Error(`no such escrow: ${agreementHash}`)
    if (e.state === 'refunded') return
    e.state = 'refunded'
  }
}
