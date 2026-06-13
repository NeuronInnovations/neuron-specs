package remoteid

// Spec-anchored conformance wrappers for the Remote ID DApp (spec 017).
//
// Per TEVV plan §7 ("Conformance tests anchored to spec sections. Test files
// and test names reference the spec section they exercise") and
// MF-01 acceptance criterion: every relevant Remote ID
// test is reachable under a Test_FR_R<NN>_* name so that
// `go test -run Test_FR_R` exposes the spec-clause → test linkage from the
// repository alone, without a separate document.
//
// This file deliberately uses Option B (wrapper subtests) rather than
// renaming the underlying tests: existing top-level Test... functions
// continue to work unchanged, and the canonical spec-anchored names are
// added on top. The cost is that each underlying test runs twice per
// `go test ./...` invocation; the benefit is zero risk to any existing
// CI script or runbook that invokes tests by their original names.
//
// The companion file specs/017-remote-id-dapp/CONFORMANCE.md maps each
// FR-R ID to the wrapper test and the file paths that implement it.

import "testing"

// Test_FR_R02_CatalogIncludesRawAlways verifies FR-R02:
// connectionSetup MUST include
// {name:"raw", protocolID:"/ds240/raw/1.0.0", direction:"seller-initiates"}
// at a minimum, regardless of other catalog options.
func Test_FR_R02_CatalogIncludesRawAlways(t *testing.T) {
	t.Run("default-is-raw-only", TestBuildCatalog_DefaultIsRawOnly)
	t.Run("raw-always-included-with-options", TestBuildCatalog_RawAlwaysIncluded)
}

// Test_FR_R03_CatalogMayIncludeFilteredAndStatus verifies FR-R03:
// the seller MAY additionally include filtered/* (seller-initiates) and
// status/1.0.0 (buyer-initiates) entries in the catalog.
func Test_FR_R03_CatalogMayIncludeFilteredAndStatus(t *testing.T) {
	t.Run("filtered-when-requested", TestBuildCatalog_IncludeFiltered)
	t.Run("status-when-requested", TestBuildCatalog_IncludeStatus)
	t.Run("all-three-entries-when-both-requested", TestBuildCatalog_AllEntries)
}

// Test_FR_R05_RemoteIdFrameCanonicalJSON verifies FR-R05: normalized
// canonical-JSON RemoteIdFrame with required fields (type, version,
// observedAt, source, droneId, droneIdType) and optional fields
// (position, velocity, operator, regulatorVariant) — canonical ordering
// per the spec; absent optionals MUST be omitted not null.
func Test_FR_R05_RemoteIdFrameCanonicalJSON(t *testing.T) {
	t.Run("minimal-required-only", TestRemoteIdFrame_Canonical_Minimal)
	t.Run("with-position", TestRemoteIdFrame_Canonical_WithPosition)
	t.Run("full-with-all-optionals", TestRemoteIdFrame_Canonical_Full)
	t.Run("absent-optionals-omitted-not-null", TestRemoteIdFrame_OptionalFieldsOmittedWhenNil)
	t.Run("round-trip-byte-identical", TestRemoteIdFrame_RoundTrip)
	t.Run("reject-missing-mandatory-fields", TestRemoteIdFrame_MarshalRejectsMissingMandatoryFields)
	t.Run("defaults-type-and-version-when-empty", TestRemoteIdFrame_DefaultsTypeAndVersionWhenEmpty)
	t.Run("reject-malformed-input", TestRemoteIdFrame_UnmarshalMalformedRejected)
	t.Run("from-decoded-preserves-all-fields", TestFromDecoded_PreservesAllFields)
	t.Run("from-decoded-applies-defaults", TestFromDecoded_AppliesDefaultsForTypeAndVersion)
}

// Test_FR_R07_UpstreamSourceUnavailableIsInformational verifies FR-R07:
// temporary RF receiver dropout / GPS lock loss / RF noise spikes MUST
// NOT close active streams. The error taxonomy MUST distinguish this
// "informational only" condition so the data plane treats it as a log
// signal, not a kill signal.
func Test_FR_R07_UpstreamSourceUnavailableIsInformational(t *testing.T) {
	t.Run("kind-defined-and-distinct", func(t *testing.T) {
		t.Parallel()
		if ErrUpstreamSourceUnavailable == "" {
			t.Fatal("FR-R07: ErrUpstreamSourceUnavailable must be a defined error kind")
		}
		// Distinct from sibling kinds so callers can switch on it
		// without conflating with a fatal frame-malformed condition.
		distinctFrom := []ErrorKind{
			ErrInvalidFilterParameter,
			ErrFrameMalformed,
			ErrUnsupportedRegulatorVariant,
			ErrFanoutTopicUnavailable,
		}
		for _, other := range distinctFrom {
			if ErrUpstreamSourceUnavailable == other {
				t.Fatalf("FR-R07: ErrUpstreamSourceUnavailable must be distinct from %s", other)
			}
		}
	})
}

// Test_FR_R13_ErrorTaxonomyComplete verifies FR-R13: the
// NEURON-DAPP-REMOTEID-* domain MUST define at minimum:
// InvalidFilterParameter, OdidFrameMalformed (FrameMalformed),
// UnsupportedRegulatorVariant, UpstreamSourceUnavailable,
// FanoutTopicUnavailable.
func Test_FR_R13_ErrorTaxonomyComplete(t *testing.T) {
	required := []struct {
		name string
		kind ErrorKind
	}{
		{"InvalidFilterParameter", ErrInvalidFilterParameter},
		{"FrameMalformed", ErrFrameMalformed},
		{"UnsupportedRegulatorVariant", ErrUnsupportedRegulatorVariant},
		{"UpstreamSourceUnavailable", ErrUpstreamSourceUnavailable},
		{"FanoutTopicUnavailable", ErrFanoutTopicUnavailable},
	}
	for _, r := range required {
		t.Run(r.name, func(t *testing.T) {
			t.Parallel()
			if r.kind == "" {
				t.Fatalf("FR-R13: error kind %s must be defined (got empty string)", r.name)
			}
		})
	}
	t.Run("error-format-includes-kind-and-operation", func(t *testing.T) {
		t.Parallel()
		e := New(ErrFrameMalformed, "decodeFrame", "trailing bytes")
		got := e.Error()
		// Format per errors.go:58 — "remoteid.<Operation>: [<Kind>] <message>"
		wantSubstrings := []string{"remoteid.", "decodeFrame", "FrameMalformed", "trailing bytes"}
		for _, sub := range wantSubstrings {
			if !contains(got, sub) {
				t.Fatalf("FR-R13: Error() %q must contain %q", got, sub)
			}
		}
	})
}

// Test_FR_R02_SellerRegistersRawProtocolHandler verifies the seller-side
// of FR-R02: the canonical Phase 2 seller registers the raw protocol
// handler at startup and serves it correctly. (Filtered + status handler
// registration are Phase 6 work per seller.go:85-86; tracked in
// week-1-gap-report.md §3.2 as "🟡 advertised but not wired".)
func Test_FR_R02_SellerRegistersRawProtocolHandler(t *testing.T) {
	t.Run("pumps-synthetic-frames-to-buyer", TestSeller_Start_PumpsSyntheticFramesToBuyer)
	t.Run("requires-host", TestSeller_Start_RequiresHost)
	t.Run("requires-source", TestSeller_Start_RequiresSource)
	t.Run("defaults-to-raw-protocol", TestSeller_Start_DefaultsToRawProtocol)
	t.Run("honors-custom-protocol-id", TestSeller_Start_HonorsCustomProtocolID)
}

// Test_SC_R01_RawStreamVerticalSlice — composite scenario test for
// SC-R01: normalized RemoteIdFrame round-trip over a Phase 2 vertical
// slice (seller + buyer + canonical JSON, no commerce). This is the
// integration anchor for the 2026-05-29 TEVV milestone.
func Test_SC_R01_RawStreamVerticalSlice(t *testing.T) {
	t.Run("phase-2-fixture-vertical-slice", TestPhase2_FixtureVerticalSlice)
}

// contains is a small helper for the FR-R13 format check that avoids
// adding a "strings" import gate; tests pass on Go 1.25.
func contains(haystack, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
