# T001 — Handshake Smoke Test

**Purpose**: Prove the cheapest thing first — a browser can complete a libp2p Noise XX + yamux + identify handshake against a Node.js libp2p host over `ws://` and round-trip one echo message. No Neuron envelope, no bootstrap JSON, no Spec 008 flow. Nothing else in spec 012 starts until this passes.

**Traces**: plan.md Phase 1A (T001); research.md R0.2 / R0.3; Risk R3.

## Run it

In two terminals, from `impl/typescript/`:

```bash
# Terminal 1 — start the fixture server on :8081
pnpm run smoke:server
```

You should see:

```
fixture-server: peer id 12D3KooW…
fixture-server: listening /ip4/127.0.0.1/tcp/8081/ws/p2p/12D3KooW…
fixture-server: wrote …/tests/smoke/public/smoke-addr.txt
fixture-server: echo protocol /test/echo/1.0.0
fixture-server: ready — press ctrl-c to stop
```

```bash
# Terminal 2 — start vite dev server on :5174 and open the browser
pnpm run smoke:browser
```

The browser opens `http://127.0.0.1:5174/smoke.html`. Click **Run smoke test**.

## Expected outcome

The log on the page should end with a green line:

```
[ok] HANDSHAKE OK — round-trip = Nms — received "ping-<timestamp>"
```

## Pass criteria (T001 Done-when)

1. `pnpm run smoke:server` starts without errors and prints the listen multiaddr.
2. `pnpm run smoke:browser` opens the page at `http://127.0.0.1:5174/smoke.html`.
3. Clicking **Run** produces the green `HANDSHAKE OK` line.
4. Both **Chromium ≥ 120** and **Firefox ≥ 115** produce the same result (manual two-browser check).
5. If the resolved js-libp2p version differs from `research.md` R0.2's proposed default, record the tested pair in `research.md`.

## Failure modes (common)

| Browser shows | Likely cause | Fix |
|---|---|---|
| `smoke-addr.txt fetch status 404` | Server hasn't started yet, or started on a different port | Start server first; if port changed, restart both |
| `dial failed: connection refused` | Server not running on `127.0.0.1:8081` | Check Terminal 1, ensure no crash |
| `WebSocket connection failed` | Corporate firewall or browser blocks loopback WS | Try a different browser; check OS firewall |
| `noise handshake failed` | js-libp2p / @chainsafe/libp2p-noise major-version mismatch | Check `pnpm install` completed; review `package.json` pins |
| `DialDeniedError: connection gater denied all addresses` | Default browser gater rejects loopback + insecure `/ws`. | Phase 1 overrides `connectionGater.denyDialMultiaddr` to permit loopback — already wired in `smoke-client.ts`. Phase 2 H2 (wss:// + public host) removes the override. |
| Browser: `EncryptionFailedError: Unexpected EOF … 0/1 bytes` (server silently exits during Noise) | Server Node.js lacks `Promise.withResolvers` (needs Node ≥ 21.12 / ≥ 20.12 / ≥ 22). libp2p@3 uses it in its upgrader; throwing kills the stream mid-Noise, so the client sees EOF. | `fixture-server.ts` polyfills `Promise.withResolvers` at the top. Remove once the project's minimum Node version is bumped to ≥ 22 in `package.json` engines. |

## Files

- `fixture-server.ts` — Node host, echo handler, writes `public/smoke-addr.txt`.
- `smoke-client.ts` — Browser dialer, reads `smoke-addr.txt`, opens echo stream.
- `smoke.html` — Page shell.
- `vite.config.ts` — Serves `smoke.html` + `public/` on `:5174`.
- `public/smoke-addr.txt` — Generated at server startup (gitignored).

## When to delete this

Keep through Phase 1 implementation — it's a fast smoke gate if anything breaks in the libp2p stack. Phase 2 H3 (automated Playwright tamper tests) may eventually fold this into CI, at which point this folder can be removed.
