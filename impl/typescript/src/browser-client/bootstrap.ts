/// <reference lib="dom" />
// Spec 012 — bootstrap fetcher.
// Wraps validateBootstrap() with a same-origin fetch per contracts/bootstrap-json.md.
//
// Traces: FR-B23, FR-B24.

import { validateBootstrap, type BootstrapJSON } from './bootstrap-schema.js'
import { makeNeuronError, NeuronBrowserCode } from './errors.js'

export async function loadBootstrap(pageOrigin: string): Promise<BootstrapJSON> {
  let resp: Response
  const url = new URL('/bootstrap.json', pageOrigin).toString()
  console.log('[neuron-012] fetching bootstrap:', url)
  try {
    resp = await fetch(url, {
      mode: 'same-origin',
      cache: 'no-store',
      credentials: 'omit',
    })
  } catch (err) {
    const detail = err instanceof Error ? `${err.name}: ${err.message}` : String(err)
    console.error('[neuron-012] bootstrap fetch threw', { url, err })
    throw makeNeuronError(
      NeuronBrowserCode.BOOTSTRAP_FETCH_FAILED,
      `bootstrap.json fetch threw at ${url} — ${detail}`,
      err,
    )
  }
  if (!resp.ok) {
    throw makeNeuronError(
      NeuronBrowserCode.BOOTSTRAP_FETCH_FAILED,
      `bootstrap.json returned HTTP ${resp.status}`,
    )
  }
  const ct = resp.headers.get('content-type') ?? ''
  if (!ct.toLowerCase().startsWith('application/json')) {
    throw makeNeuronError(
      NeuronBrowserCode.BOOTSTRAP_WRONG_CONTENT_TYPE,
      `bootstrap.json has unexpected content-type ${ct}`,
    )
  }
  let doc: unknown
  try {
    doc = await resp.json()
  } catch (err) {
    throw makeNeuronError(
      NeuronBrowserCode.BOOTSTRAP_PARSE_FAILURE,
      'bootstrap.json parse failed',
      err,
    )
  }
  return validateBootstrap(doc, pageOrigin)
}
