// Browser-only shim for the subset of `node:crypto` used by keylib in the
// browser-client import graph. Aliased via vite.config.ts `resolve.alias`.
//
// keylib reaches `node:crypto` through:
//   - src/keylib/matching.ts    → timingSafeEqual
//   - src/keylib/signature.ts   → timingSafeEqual
// (encrypted-key.ts uses createCipheriv/Decipheriv + randomBytes, but is not
//  in the browser-client import chain — see src/browser-client/session.ts.)
//
// Polyfill is constant-time on equal-length inputs; returns false on length
// mismatch without leaking any timing information about content.

export function timingSafeEqual(a: Uint8Array, b: Uint8Array): boolean {
  if (a.length !== b.length) return false
  let diff = 0
  for (let i = 0; i < a.length; i++) {
    // Non-null asserts: both arrays have index i by the length check above.
    diff |= (a[i] as number) ^ (b[i] as number)
  }
  return diff === 0
}

// Keep the other `node:crypto` exports defined-but-throwing so if any future
// import reaches them we get a clear error rather than `undefined is not a
// function` at call-site.
function notInBrowser(name: string): (...args: unknown[]) => never {
  return () => {
    throw new Error(
      `node:crypto.${name} is not available in the browser shim. ` +
        `If this is reached, update examples/browser-demo/node-crypto-shim.ts.`,
    )
  }
}
export const randomBytes = notInBrowser('randomBytes')
export const createCipheriv = notInBrowser('createCipheriv')
export const createDecipheriv = notInBrowser('createDecipheriv')
