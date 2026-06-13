/// <reference lib="dom" />
// 2a-wt — fetch wrapper around validateBootstrapWt.
// Same-origin fetch of /bootstrap-wt.json; parallel to bootstrap.ts.

import { validateBootstrapWt, type BootstrapWtJSON } from './bootstrap-wt-schema.js'
import { makeNeuronError, NeuronBrowserCode } from '../browser-client/errors.js'

export async function loadBootstrapWt(pageOrigin: string): Promise<BootstrapWtJSON> {
  const url = new URL('/bootstrap-wt.json', pageOrigin).toString()
  console.log('[neuron-012-wt] fetching bootstrap:', url)

  let resp: Response
  try {
    resp = await fetch(url, {
      mode: 'same-origin',
      cache: 'no-store',
      credentials: 'omit',
    })
  } catch (err) {
    const detail = err instanceof Error ? `${err.name}: ${err.message}` : String(err)
    console.error('[neuron-012-wt] bootstrap fetch threw', { url, err })
    throw makeNeuronError(
      NeuronBrowserCode.BOOTSTRAP_FETCH_FAILED,
      `bootstrap-wt.json fetch threw at ${url} — ${detail}`,
      err,
    )
  }
  if (!resp.ok) {
    throw makeNeuronError(
      NeuronBrowserCode.BOOTSTRAP_FETCH_FAILED,
      `bootstrap-wt.json returned HTTP ${resp.status}`,
    )
  }
  const ct = resp.headers.get('content-type') ?? ''
  if (!ct.toLowerCase().startsWith('application/json')) {
    throw makeNeuronError(
      NeuronBrowserCode.BOOTSTRAP_WRONG_CONTENT_TYPE,
      `bootstrap-wt.json has unexpected content-type ${ct}`,
    )
  }
  let doc: unknown
  try {
    doc = await resp.json()
  } catch (err) {
    throw makeNeuronError(
      NeuronBrowserCode.BOOTSTRAP_PARSE_FAILURE,
      'bootstrap-wt.json parse failed',
      err,
    )
  }
  return validateBootstrapWt(doc)
}
