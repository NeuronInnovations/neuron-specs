// Browser-only shim for the subset of `node:crypto` that the keylib
// EVMAddress validator reaches via timingSafeEqual.
//
// 2a-wt — copy of the Tier 1 shim. Kept as a separate file so this demo
// is self-contained under examples/browser-demo-wt/.

export function timingSafeEqual(a: Uint8Array, b: Uint8Array): boolean {
  if (a.length !== b.length) return false
  let diff = 0
  for (let i = 0; i < a.length; i++) {
    diff |= (a[i] as number) ^ (b[i] as number)
  }
  return diff === 0
}

function notInBrowser(name: string): (...args: unknown[]) => never {
  return () => {
    throw new Error(
      `node:crypto.${name} is not available in the browser shim. ` +
        `If this is reached, update examples/browser-demo-wt/node-crypto-shim.ts.`,
    )
  }
}
export const randomBytes = notInBrowser('randomBytes')
export const createCipheriv = notInBrowser('createCipheriv')
export const createDecipheriv = notInBrowser('createDecipheriv')
