package remoteid

// Spec-anchored conformance wrappers for the Remote ID feed sources
// (spec 017, FR-R05 / FR-R12 surfaces).
//
// Per the MF-01 acceptance criterion, these wrappers expose the
// spec-clause → test linkage in the test names themselves. Existing
// top-level test names continue to work unchanged (Option B wrapper
// pattern).
//
// The feed-source layer (replay / synth / ds400) supplies DecodedFrame
// values that are normalized into RemoteIdFrame canonical JSON per
// FR-R05 by the dapp/remoteid layer; the DS-400 source contract
// foreshadows FR-R12 active-service persistence by exposing the decoder
// registry + source-tag stamping the persisted catalog state will rely on.

import "testing"

// Test_FR_R05_ReplaySourceProducesCanonicalShape verifies the replay
// feed source emits DecodedFrames whose field shape matches FR-R05.
// (The canonical-JSON encoding of these frames lives in dapp/remoteid;
// here we cover the upstream half of the pathway.)
func Test_FR_R05_ReplaySourceProducesCanonicalShape(t *testing.T) {
	t.Run("emits-all-fixture-entries", TestRunReplay_EmitsAllFixtureEntries)
	t.Run("respects-pacing", TestRunReplay_RespectsPacing)
	t.Run("loop-restarts-from-beginning", TestRunReplay_LoopRestartsFromBeginning)
	t.Run("errors-on-missing-file", TestRunReplay_ErrorsOnMissingFile)
	t.Run("errors-on-empty-path", TestRunReplay_ErrorsOnEmptyPath)
	t.Run("synthesizes-observed-at-when-absent", TestRunReplay_ObservedAtSynthesizedWhenAbsent)
}

// Test_FR_R05_SynthSourceProducesCanonicalShape verifies the synthetic-
// orbit feed source emits DecodedFrames suitable for FR-R05
// normalization. Deterministic synth output is also the fixture-free
// driver for SC-R01 smoke tests.
func Test_FR_R05_SynthSourceProducesCanonicalShape(t *testing.T) {
	t.Run("emits-expected-shape", TestRunSynth_EmitsExpectedShape)
	t.Run("rejects-zero-fps", TestRunSynth_RejectsZeroFPS)
	t.Run("rejects-zero-drone-count", TestRunSynth_RejectsZeroDroneCount)
	t.Run("alternates-drones-in-round-robin", TestRunSynth_AlternatesDronesInRoundRobin)
}

// Test_FR_R12_DS400SourceContract verifies the DS-400 source adapter
// surface that future FR-R12 persistence will rely on: transport
// selection, error semantics, and source-tag stamping. The network
// read-loop itself is intentionally stubbed (see ds400_source.go);
// these tests cover the stub contract so the swap-in is mechanical
// once DS-400 device access lands.
//
// Two decoder-mutating tests are deliberately excluded from this
// wrapper — TestRegisterFrameDecoder_ReturnsPrevious and
// TestRunDS400_ReturnsUnavailableEvenWithDecoder both write to the
// package-level frameDecoders map, and the Option B wrapper pattern
// would cause concurrent invocation (subtest + top-level test) on
// the same map, panicking with "concurrent map writes". They remain
// invokable by their canonical top-level names; CONFORMANCE.md lists
// them under FR-R12 for the spec-clause linkage.
//
// Note: FR-R12 in the spec ("persisted active-service entry includes
// the active stream catalog state") is not yet implemented; the
// "FR-R12 contract" anchor used here documents the upstream-source
// side of that future persistence layer.
func Test_FR_R12_DS400SourceContract(t *testing.T) {
	t.Run("returns-unavailable-when-no-decoder", TestRunDS400_ReturnsUnavailableWhenNoDecoder)
	t.Run("rejects-missing-transport", TestRunDS400_RejectsMissingTransport)
	t.Run("rejects-missing-address", TestRunDS400_RejectsMissingAddress)
	t.Run("rejects-unknown-transport", TestRunDS400_RejectsUnknownTransport)
	t.Run("config-source-tag-defaults-to-vendor", TestDS400Config_SourceTagDefaultsToVendor)
	t.Run("feed-source-shape", TestRunDS400_FeedSourceShape)
	t.Run("error-points-to-checklist", TestDS400ErrorPointsToCheckList)
}

// Test_FR_R05_DroneScoutJSONProducesCanonicalShape exercises the
// DroneScout MQTT JSON parser + normalizer pair (Stage B). Per spec
// 017 FR-R05, the data plane carries canonical-JSON RemoteIdFrame
// payloads; this wrapper covers the upstream half — parse a
// DroneScout MQTT JSON envelope, normalize it into a DecodedFrame
// whose field shape will round-trip through internal/dapp/remoteid
// FromDecoded → MarshalJSON as canonical FR-R05 JSON. UASdata is
// preserved as base64 with DroneIDType="uasdata-base64" until Stage C
// decodes the byte layout (research-report §6.2).
//
// Fixtures live under testdata/dronescout/; see that directory's
// README for provenance (synthetic-shape, not captured from a real
// sensor).
func Test_FR_R05_DroneScoutJSONProducesCanonicalShape(t *testing.T) {
	t.Run("data-bt5-single-parses", TestParseSensorPayload_DataBT5Single)
	t.Run("data-bt5-aggregated-splits-two-objects", TestParseSensorPayload_DataBT5Aggregated)
	t.Run("status-parses", TestParseSensorPayload_Status)
	t.Run("location-parses", TestParseSensorPayload_Location)
	t.Run("empty-payload-returns-empty", TestParseSensorPayload_EmptyPayload)
	t.Run("whitespace-only-payload-returns-empty", TestParseSensorPayload_WhitespaceOnlyPayload)
	t.Run("trims-trailing-newline-and-null", TestParseSensorPayload_TrimsTrailingNewlineAndNull)
	t.Run("empty-compression-defaults-to-none", TestParseSensorPayload_EmptyCompressionDefaultsToNone)
	t.Run("rejects-lzma-stage-b", TestParseSensorPayload_RejectsLZMA)
	t.Run("rejects-unknown-compression", TestParseSensorPayload_RejectsUnknownCompression)
	t.Run("rejects-malformed-json", TestParseSensorPayload_RejectsMalformedJSON)
	t.Run("skips-unknown-kind-forward-compat", TestParseSensorPayload_SkipsUnknownKind)
	t.Run("recognises-aircraft-kind", TestParseSensorPayload_RecognisesAircraftKind)
	t.Run("brace-boundary-inside-string-does-not-split", TestParseSensorPayload_StringContainingBraceBoundaryDoesNotSplit)
	t.Run("normalize-data-kind-produces-frame", TestNormalizeToDecodedFrame_DataKindProducesFrame)
	t.Run("observed-at-from-timestamp-ms-to-ns", TestNormalizeToDecodedFrame_ObservedAtFromTimestamp)
	t.Run("defaults-source-tag-to-ds240", TestNormalizeToDecodedFrame_DefaultsSourceTag)
	t.Run("rejects-non-data-kind", TestNormalizeToDecodedFrame_RejectsNonDataKind)
	t.Run("rejects-missing-uasdata", TestNormalizeToDecodedFrame_RejectsMissingUASdata)
	t.Run("rejects-nil-data-pointer", TestNormalizeToDecodedFrame_RejectsNilDataPointer)
	t.Run("fixtures-produce-canonical-decoded-frames", TestDroneScoutFixture_DataMessagesProduceCanonicalDecodedFrames)
}

// Test_FR_R12_DroneScoutMQTTSourceContract exercises the DroneScout
// MQTT source-adapter surface that future FR-R12 persistence will
// rely on: configuration validation, connect-error semantics, and
// the end-to-end subscribe → parse → normalize → emit pipeline. The
// network read-loop itself is exercised against an in-process
// mochi-mqtt broker (Stage C-lite, 2026-05-13); a real broker /
// real DroneScout sensor lands in Stage C-full alongside the
// `cmd/remoteid-seller --source=dronescout-mqtt` CLI wiring.
//
// **Live-vs-fixture classification is operator-side.** The
// integration tests in this wrapper run against an in-process
// broker fed by synthetic-shape fixtures — they are NOT live
// evidence. A seller using RunDroneScoutMQTT against a real broker
// fed by a real ds240 advertises `feedSource = "live"` via the
// heartbeat layer; this wrapper does not exercise that path.
//
// Note: FR-R12 in the spec ("persisted active-service entry
// includes the active stream catalog state") is not yet
// implemented; this wrapper documents the upstream-source side of
// that future persistence layer.
func Test_FR_R12_DroneScoutMQTTSourceContract(t *testing.T) {
	t.Run("rejects-empty-url", TestRunDroneScoutMQTT_RejectsEmptyURL)
	t.Run("rejects-unknown-compression", TestRunDroneScoutMQTT_RejectsUnknownCompression)
	t.Run("returns-connect-error-when-broker-unreachable", TestRunDroneScoutMQTT_ReturnsConnectErrorWhenBrokerUnreachable)
	t.Run("config-validate-normalises-compression", TestDroneScoutMQTTConfig_ValidateNormalisesCompression)
	t.Run("config-validate-normalises-topic", TestDroneScoutMQTTConfig_ValidateNormalisesTopic)
	t.Run("config-validate-normalises-timeouts", TestDroneScoutMQTTConfig_ValidateNormalisesTimeouts)
	t.Run("config-source-tag-default", TestDroneScoutMQTTConfig_SourceTagDefault)
	t.Run("config-source-tag-override", TestDroneScoutMQTTConfig_SourceTagOverride)
	t.Run("config-client-id-default-is-random", TestDroneScoutMQTTConfig_ClientIDDefaultIsRandom)
	t.Run("config-client-id-override", TestDroneScoutMQTTConfig_ClientIDOverride)
	t.Run("integration-end-to-end-data-message", TestRunDroneScoutMQTT_IntegrationEndToEnd_DataMessage)
	t.Run("integration-end-to-end-aggregated-message", TestRunDroneScoutMQTT_IntegrationEndToEnd_AggregatedMessage)
	t.Run("integration-skips-non-data-kinds", TestRunDroneScoutMQTT_IntegrationEndToEnd_SkipsNonDataKinds)
	t.Run("integration-respects-sensor-id-allow", TestRunDroneScoutMQTT_IntegrationEndToEnd_RespectsSensorIDAllow)
	t.Run("integration-ctx-cancel-returns", TestRunDroneScoutMQTT_IntegrationEndToEnd_CtxCancelReturns)
	t.Run("integration-unknown-kind-silently-skipped", TestRunDroneScoutMQTT_IntegrationEndToEnd_UnknownKindIsSilentlySkipped)
	t.Run("integration-malformed-payload-drops-silently", TestRunDroneScoutMQTT_IntegrationEndToEnd_MalformedPayloadDropsSilently)
}
