# DroneScout MQTT fixtures

These fixtures are **synthetic-shape** test vectors — not captured from a
real DroneScout sensor. The JSON envelope structure mirrors the
documented BlueMark MQTT message format (per the BlueMark
`RemoteID-MQTT-subscriber` reference Python implementation and the
DroneScout 230/240-series manual). When a real captured payload becomes
available from a sensor deployment, **replace these files with the captured bytes** — the parser
in
[`../../dronescout_json.go`](../../dronescout_json.go) does not change.

The `UASdata` field in the `data-*.json` fixtures contains an
**obviously-synthetic placeholder** base64 string (e.g.,
`"U1lOVEgtVUFTREFUQS1ESDIzNDAxMjMtMDAwMQ=="` decodes to
`"SYNTH-UASDATA-DH234012?-0001"`). Stage B preserves the base64 payload
verbatim in `DecodedFrame.DroneID` with
`DroneIDType = "uasdata-base64"`; the OpenDroneID byte-level decode
lands in Stage C.

## Files

| Filename                      | Purpose                                                                                                       |
| ----------------------------- | ------------------------------------------------------------------------------------------------------------- |
| `data-bt5-single.json`        | One Remote ID detection over BT5; primary parser golden vector for the single-object happy path               |
| `data-bt5-aggregated.json`    | `transmit_mode = 2` shape: two detections concatenated with the `}{` boundary in one MQTT message             |
| `status.json`                 | Sensor `status` message; firmware-version + model provenance for evidence packs                               |
| `location.json`               | Sensor self-location message (LTE add-on only); NOT a drone position                                          |

The filename convention is `data-<transmission-type>-<scenario>.json`.
Allowed transmission types: `bt4` / `bt5` / `wlan-beacon` / `wlan-nan`.

## Feed-source classification

A seller reading these fixtures from disk MUST advertise
`feedSource = "replay"` per spec 017 FR-R15. They are **not** live
evidence — the underlying bytes were not produced by an active sensor
in real time. Live classification only applies to seller runs that
subscribe to a real MQTT broker (Stage C work).

## Sanitisation policy

If/when a real captured payload replaces these fixtures, the redaction
policy in
[`docs/tevv/dronescout-mqtt-capture-checklist.md`](../../../../../docs/tevv/dronescout-mqtt-capture-checklist.md)
§4 applies: no plaintext operator IDs, no real-world drone serial
numbers we don't have permission to publish, no broker hostnames, no
sensor IDs that identify a specific deployment. The capture checklist
includes a Python sanitiser stub the operator can adapt.

## Why synthetic for Stage B

The parser surface (JSON envelope + kind discrimination + UASdata
preservation) is well-defined regardless of fixture source. Building
the parser against the documented JSON shape unblocks Stage B today;
swapping in a real packet later is a one-file fixture replacement, not
a parser change. See `docs/tevv/dronescout-mqtt-live-feed-plan.md` §8.
