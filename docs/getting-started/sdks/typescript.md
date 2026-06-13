# TypeScript SDK — Integrator Guide

← Back to [Getting Started](../README.md)

This guide is for engineers who want to **embed the Neuron TypeScript SDK in their own Node.js or browser application**. If you want to run the bundled browser demos instead, see [Step 4](../demos/4-browser-wss.md) and [Step 5](../demos/5-browser-webtransport.md).

## Install

```bash
npm install @neuron-sdk/typescript
# or
pnpm add @neuron-sdk/typescript
```

Requires Node.js 20.12+ for server-side use. Browser targets: Chromium ≥ 120 and Firefox ≥ 115 (mandatory per spec [012](../../../specs/012-browser-client-profile/spec.md) SC-05).

> The package is published as `@neuron-sdk/typescript` and tracks specs 001–005 + browser profile (012). The TypeScript SDK is **buyer-side complete** — sufficient to negotiate with and receive data from a Go seller; payment, delivery, and validation sellers run on the Go SDK.

## Package map

| Spec                                                     | Module                                                                               | Purpose                                                                                                                                                                                        |
| -------------------------------------------------------- | ------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [002](../../../specs/002-key-library/spec.md)            | `@neuron-sdk/typescript/keylib` ([source](../../../impl/typescript/src/keylib/))     | secp256k1 keys, EVM addresses, PeerIDs, DID:keys, signatures                                                                                                                                   |
| [001](../../../specs/001-neuron-account-module/spec.md)  | `@neuron-sdk/typescript/account` ([source](../../../impl/typescript/src/account/))   | Parent / Child / Shared identity                                                                                                                                                               |
| [004](../../../specs/004-topic-system/spec.md)           | `@neuron-sdk/typescript/topic` ([source](../../../impl/typescript/src/topic/))       | `TopicMessage`, adapters                                                                                                                                                                       |
| [003](../../../specs/003-peer-registry/spec.md)          | `@neuron-sdk/typescript/registry` ([source](../../../impl/typescript/src/registry/)) | EIP-8004 NFT registration                                                                                                                                                                      |
| [005](../../../specs/005-health/spec.md)                 | `@neuron-sdk/typescript/health` ([source](../../../impl/typescript/src/health/))     | `HeartbeatPayload`, liveness                                                                                                                                                                   |
| [006](../../../specs/006-protocol-determinism/spec.md)   | `@neuron-sdk/typescript/wire` ([source](../../../impl/typescript/src/wire/))         | Wire format utilities, canonical JSON                                                                                                                                                          |
| [012](../../../specs/012-browser-client-profile/spec.md) | (browser examples)                                                                   | Reference browser clients in [`examples/browser-demo/`](../../../impl/typescript/examples/browser-demo/) and [`examples/browser-demo-wt/`](../../../impl/typescript/examples/browser-demo-wt/) |

The TypeScript SDK derives **purely from specs**, never from the Go reference implementation. This is enforced by Constitution Principle VI.

## Hello, Neuron — minimal example

The smallest meaningful program: generate a key, derive identities, sign and verify a message.

```typescript
import {
  newNeuronPrivateKey,
  deriveEVMAddress,
  derivePeerID,
  deriveDIDKey,
  sign,
  verify,
} from "@neuron-sdk/typescript/keylib";

async function main() {
  // Generate a fresh secp256k1 key
  const key = await newNeuronPrivateKey();
  const pub = key.publicKey();

  // Derive all four identities from the single key
  const evm = await deriveEVMAddress(pub); // 0x... (EIP-55 checksummed)
  const peerID = await derivePeerID(pub); // 16Uiu2HAm...
  const did = await deriveDIDKey(pub); // did:key:zQ3sh...

  console.log("EVM:    ", evm);
  console.log("PeerID: ", peerID);
  console.log("DID:    ", did);

  // Sign and verify (RFC 6979 deterministic ECDSA + Keccak256)
  const message = new TextEncoder().encode("hello, neuron");
  const signature = await sign(key, message);
  const ok = await verify(message, signature, pub);
  console.log("Verified:", ok);
}

main().catch(console.error);
```

API names match the patterns documented in spec [002](../../../specs/002-key-library/spec.md). Read the source for the full surface.

## Patterns by use case

### "I want a browser tab to act as a Neuron buyer"

1. Generate an ephemeral browser key (the page reload throws it away — see spec [012](../../../specs/012-browser-client-profile/spec.md))
2. Connect to a seller via libp2p — WSS for general browsers, WebTransport for Chromium ≥ 120
3. Run the negotiation flow against the seller's `stdIn` topic
4. Reference: [`examples/browser-demo/`](../../../impl/typescript/examples/browser-demo/) (WSS) and [`examples/browser-demo-wt/`](../../../impl/typescript/examples/browser-demo-wt/) (WebTransport)
5. Walkthrough: [Step 4](../demos/4-browser-wss.md) and [Step 5](../demos/5-browser-webtransport.md)

### "I want a Node.js seller serving browser buyers"

1. Spin up a libp2p host with WSS and/or WebTransport listeners
2. Bind a request handler to the seller's `stdIn` topic
3. Reference: [`src/server-demo/`](../../../impl/typescript/src/server-demo/) — the Node seller used by the browser demos

### "I want to sign and verify messages on the server"

1. Use `@neuron-sdk/typescript/keylib` exactly as in the Hello example above
2. The same canonical-JSON serialisation and RFC 6979 signing applies on Node and in the browser — identical wire format

## Build & test

From the repo:

```bash
cd impl/typescript
pnpm install
pnpm run build              # tsc -p tsconfig.build.json
pnpm test                   # vitest run
pnpm run test:conformance   # cross-language test vectors (spec 006)
pnpm run lint               # eslint
pnpm run typecheck          # tsc --noEmit
```

The full command list lives in [`../../../CLAUDE.md`](../../../CLAUDE.md).

## Browser bundling

The SDK uses ESM (`"type": "module"`). Bundlers that handle ESM cleanly (Vite, esbuild, Webpack 5+, Rollup) work out of the box. The browser demos use Vite — see [`examples/browser-demo/vite.config.ts`](../../../impl/typescript/examples/browser-demo/) for a working configuration.

Key dependencies that need to load in the browser:

- `@noble/secp256k1` + `@noble/hashes` — pure JS, no WASM, browser-safe
- `@scure/bip32` + `@scure/bip39` — pure JS
- `libp2p` + `@libp2p/websockets` (or `@libp2p/webtransport`) — work in browser
- `hash-wasm` — Argon2id KDF, WASM module, loaded asynchronously

## Conventions

- **ESM only.** No CommonJS.
- **No `any`.** Strong types throughout. Public surface uses `Uint8Array` for byte data.
- **No re-exports of Go code.** The TypeScript SDK is independent — see Constitution Principle VI.
- **vitest** for tests; conformance tests in `tests/conformance/` consume the spec [006](../../../specs/006-protocol-determinism/spec.md) golden vectors.

## Where to read more

- Architecture overview → [architecture.md](../architecture.md)
- The twelve Constitution principles → [`../../../.specify/memory/constitution.md`](../../../.specify/memory/constitution.md)
- Repository architecture and build order → [`../../../CLAUDE.md`](../../../CLAUDE.md)
- Each spec's `spec.md` is the source of truth — read the relevant section before extending the SDK

