package validation

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
)

// PayloadTypeEvidence is the discriminator for evidence envelope payloads. FR-V01.
const PayloadTypeEvidence = "validationEvidence"

// CurrentVersion is the supported evidence envelope version. FR-V07.
const CurrentVersion = "1.0.0"

// ObservationWindow represents the time range over which evidence was collected. FR-V06.
// Timestamps are UnixTimestamp nanoseconds per 006 FR-W02a.
type ObservationWindow struct {
	Start uint64
	End   uint64
}

// EvidenceEnvelope is an immutable evidence record published by a validator. FR-V02.
// All fields are private; use accessor methods to read.
type EvidenceEnvelope struct {
	envelopeType     string  // "validationEvidence"
	version          string  // "1.0.0"
	validatorAgentId string  // UnsignedInt256 decimal string
	subjectAgentId   string  // UnsignedInt256 decimal string
	specRef          string  // e.g. "005-health", "008-payment"
	verdict          Verdict // "compliant" / "non-compliant" / "inconclusive"
	evidenceHash     string  // 0x-prefixed lowercase hex, 66 chars (32 bytes)
	evidenceURI      string  // IPFS or HTTPS URI

	// Optional fields (FR-V06)
	compositeRefs     []string           // 0x-prefixed hex hashes of related envelopes
	observationWindow *ObservationWindow // time range of observation
}

// EnvelopeOption configures optional fields on an EvidenceEnvelope.
type EnvelopeOption func(*EvidenceEnvelope) error

// WithObservationWindow sets the observation window. FR-V06.
func WithObservationWindow(start, end uint64) EnvelopeOption {
	return func(e *EvidenceEnvelope) error {
		if end <= start {
			return NewValidationError(ErrInvalidEnvelopeField,
				fmt.Sprintf("observationWindow.end (%d) must be greater than start (%d)", end, start))
		}
		e.observationWindow = &ObservationWindow{Start: start, End: end}
		return nil
	}
}

// WithCompositeRefs sets the composite reference hashes. FR-V06.
func WithCompositeRefs(refs []string) EnvelopeOption {
	return func(e *EvidenceEnvelope) error {
		for _, ref := range refs {
			if !isValidHexBytes(ref) {
				return NewValidationError(ErrInvalidEnvelopeField,
					fmt.Sprintf("compositeRef %q is not valid 0x-prefixed lowercase hex", ref))
			}
		}
		copied := make([]string, len(refs))
		copy(copied, refs)
		e.compositeRefs = copied
		return nil
	}
}

// NewEvidenceEnvelope constructs a validated, immutable EvidenceEnvelope. FR-V02.
func NewEvidenceEnvelope(
	validatorAgentId string,
	subjectAgentId string,
	specRef string,
	verdict Verdict,
	evidenceHash string,
	evidenceURI string,
	opts ...EnvelopeOption,
) (*EvidenceEnvelope, error) {
	// Validate mandatory fields. FR-V02.
	if validatorAgentId == "" {
		return nil, NewValidationError(ErrMissingRequiredField, "validatorAgentId is required")
	}
	if !isValidUnsignedInt256(validatorAgentId) {
		return nil, NewValidationError(ErrInvalidEnvelopeField,
			fmt.Sprintf("validatorAgentId %q is not a valid UnsignedInt256 decimal string", validatorAgentId))
	}
	if subjectAgentId == "" {
		return nil, NewValidationError(ErrMissingRequiredField, "subjectAgentId is required")
	}
	if !isValidUnsignedInt256(subjectAgentId) {
		return nil, NewValidationError(ErrInvalidEnvelopeField,
			fmt.Sprintf("subjectAgentId %q is not a valid UnsignedInt256 decimal string", subjectAgentId))
	}
	if specRef == "" {
		return nil, NewValidationError(ErrMissingRequiredField, "specRef is required")
	}
	if !IsValidSpecRef(specRef) {
		return nil, NewValidationError(ErrInvalidSpecRef,
			fmt.Sprintf("specRef %q is not a valid spec reference", specRef))
	}
	if !IsValidVerdict(verdict) {
		return nil, NewValidationError(ErrInvalidVerdict,
			fmt.Sprintf("verdict %q is not valid; expected compliant, non-compliant, or inconclusive", verdict))
	}
	if evidenceHash == "" {
		return nil, NewValidationError(ErrMissingRequiredField, "evidenceHash is required")
	}
	if !isValidHexBytes(evidenceHash) {
		return nil, NewValidationError(ErrInvalidEnvelopeField,
			fmt.Sprintf("evidenceHash %q is not valid 0x-prefixed lowercase hex", evidenceHash))
	}
	if evidenceURI == "" {
		return nil, NewValidationError(ErrMissingRequiredField, "evidenceURI is required")
	}

	// FR-V07: Version compatibility — accept major 1, reject major >= 2.
	if err := validateVersion(CurrentVersion); err != nil {
		return nil, err
	}

	e := &EvidenceEnvelope{
		envelopeType:     PayloadTypeEvidence,
		version:          CurrentVersion,
		validatorAgentId: validatorAgentId,
		subjectAgentId:   subjectAgentId,
		specRef:          specRef,
		verdict:          verdict,
		evidenceHash:     evidenceHash,
		evidenceURI:      evidenceURI,
	}

	for _, opt := range opts {
		if err := opt(e); err != nil {
			return nil, err
		}
	}

	return e, nil
}

// Accessor methods — all return copies to prevent mutation.

func (e *EvidenceEnvelope) Type() string              { return e.envelopeType }
func (e *EvidenceEnvelope) Version() string            { return e.version }
func (e *EvidenceEnvelope) ValidatorAgentId() string   { return e.validatorAgentId }
func (e *EvidenceEnvelope) SubjectAgentId() string     { return e.subjectAgentId }
func (e *EvidenceEnvelope) SpecRef() string            { return e.specRef }
func (e *EvidenceEnvelope) Verdict() Verdict           { return e.verdict }
func (e *EvidenceEnvelope) EvidenceHash() string       { return e.evidenceHash }
func (e *EvidenceEnvelope) EvidenceURI() string        { return e.evidenceURI }

// CompositeRefs returns a copy of the composite reference hashes. FR-V06.
func (e *EvidenceEnvelope) CompositeRefs() []string {
	if e.compositeRefs == nil {
		return nil
	}
	copied := make([]string, len(e.compositeRefs))
	copy(copied, e.compositeRefs)
	return copied
}

// ObservationWindow returns a copy of the observation window, or nil. FR-V06.
func (e *EvidenceEnvelope) ObservationWindow() *ObservationWindow {
	if e.observationWindow == nil {
		return nil
	}
	w := *e.observationWindow
	return &w
}

// MarshalJSON implements canonical JSON serialization per FR-V03.
// Field order: type→version→validatorAgentId→subjectAgentId→specRef→verdict→evidenceHash→evidenceURI
// Optional fields appear in alphabetical order: compositeRefs before observationWindow.
// ObservationWindow internal order: end before start (alphabetical per 006).
// Absent optional fields are omitted entirely (006 FR-W04).
func (e EvidenceEnvelope) MarshalJSON() ([]byte, error) {
	buf := []byte{'{'}

	// Mandatory fields in canonical order. FR-V03.
	buf = appendJSONString(buf, "type", e.envelopeType, false)
	buf = appendJSONString(buf, "version", e.version, true)
	buf = appendJSONString(buf, "validatorAgentId", e.validatorAgentId, true)
	buf = appendJSONString(buf, "subjectAgentId", e.subjectAgentId, true)
	buf = appendJSONString(buf, "specRef", e.specRef, true)
	buf = appendJSONString(buf, "verdict", string(e.verdict), true)
	buf = appendJSONString(buf, "evidenceHash", e.evidenceHash, true)
	buf = appendJSONString(buf, "evidenceURI", e.evidenceURI, true)

	// Optional fields in alphabetical order. FR-V06, 006 FR-W04.
	if len(e.compositeRefs) > 0 {
		buf = append(buf, ',')
		buf = append(buf, `"compositeRefs":[`...)
		for i, ref := range e.compositeRefs {
			if i > 0 {
				buf = append(buf, ',')
			}
			refBytes, _ := json.Marshal(ref)
			buf = append(buf, refBytes...)
		}
		buf = append(buf, ']')
	}

	if e.observationWindow != nil {
		buf = append(buf, ',')
		// Internal ordering: end before start (alphabetical). 006 FR-W05.
		// Both fields are quoted decimal strings per 006 FR-W02 / FR-W02a
		// (UnixTimestamp nanoseconds).
		buf = append(buf, `"observationWindow":{`...)
		buf = append(buf, `"end":"`...)
		buf = strconv.AppendUint(buf, e.observationWindow.End, 10)
		buf = append(buf, `","start":"`...)
		buf = strconv.AppendUint(buf, e.observationWindow.Start, 10)
		buf = append(buf, `"}`...)
	}

	buf = append(buf, '}')
	return buf, nil
}

// UnmarshalJSON deserializes an EvidenceEnvelope from JSON.
//
// observationWindow.start/end are accepted as either quoted decimal strings
// (canonical per 006 FR-W02 / FR-W02a) or as legacy JSON numbers, since
// the wire form changed in this revision. Liberal in what we accept;
// strict in what we produce.
func (e *EvidenceEnvelope) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type              string   `json:"type"`
		Version           string   `json:"version"`
		ValidatorAgentId  string   `json:"validatorAgentId"`
		SubjectAgentId    string   `json:"subjectAgentId"`
		SpecRef           string   `json:"specRef"`
		Verdict           string   `json:"verdict"`
		EvidenceHash      string   `json:"evidenceHash"`
		EvidenceURI       string   `json:"evidenceURI"`
		CompositeRefs     []string `json:"compositeRefs"`
		ObservationWindow *struct {
			Start flexibleUint64 `json:"start"`
			End   flexibleUint64 `json:"end"`
		} `json:"observationWindow"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	e.envelopeType = raw.Type
	e.version = raw.Version
	e.validatorAgentId = raw.ValidatorAgentId
	e.subjectAgentId = raw.SubjectAgentId
	e.specRef = raw.SpecRef
	e.verdict = Verdict(raw.Verdict)
	e.evidenceHash = raw.EvidenceHash
	e.evidenceURI = raw.EvidenceURI
	e.compositeRefs = raw.CompositeRefs

	if raw.ObservationWindow != nil {
		e.observationWindow = &ObservationWindow{
			Start: uint64(raw.ObservationWindow.Start),
			End:   uint64(raw.ObservationWindow.End),
		}
	}

	return nil
}

// flexibleUint64 is a uint64 that accepts both quoted-decimal-string and
// JSON-number forms during deserialization. The canonical wire form per
// 006 FR-W02 is the quoted string; the number form is accepted only for
// backward compatibility with legacy in-memory artifacts.
type flexibleUint64 uint64

// UnmarshalJSON parses either `"123"` or `123` into a uint64.
func (f *flexibleUint64) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("flexibleUint64: empty input")
	}
	s := string(data)
	if s[0] == '"' && len(s) >= 2 && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return fmt.Errorf("flexibleUint64: parse %q: %w", s, err)
	}
	*f = flexibleUint64(n)
	return nil
}

// ComputeEnvelopeHash returns keccak256(canonicalJSON(envelope)). FR-V09.
// This is the responseHash used to link on-chain verdicts to off-chain envelopes.
func ComputeEnvelopeHash(envelope *EvidenceEnvelope) ([32]byte, error) {
	data, err := json.Marshal(envelope)
	if err != nil {
		return [32]byte{}, WrapValidationError(ErrInvalidEnvelopeField,
			"failed to serialize envelope for hash computation", err)
	}
	hash := crypto.Keccak256(data)
	var result [32]byte
	copy(result[:], hash)
	return result, nil
}

// ComputeEvidenceHash returns keccak256(documentBytes). FR-V05.
// This is the evidenceHash field that links the envelope to off-chain evidence.
func ComputeEvidenceHash(documentBytes []byte) [32]byte {
	hash := crypto.Keccak256(documentBytes)
	var result [32]byte
	copy(result[:], hash)
	return result
}

// VerifyEvidenceHash checks that keccak256(documentBytes) matches envelope.EvidenceHash(). SC-V07.
func VerifyEvidenceHash(documentBytes []byte, envelope *EvidenceEnvelope) bool {
	computed := ComputeEvidenceHash(documentBytes)
	expected := fmt.Sprintf("0x%x", computed[:])
	return expected == envelope.EvidenceHash()
}

// FormatEvidenceHash formats a 32-byte hash as 0x-prefixed lowercase hex (66 chars). 006 FR-W06.
func FormatEvidenceHash(hash [32]byte) string {
	return fmt.Sprintf("0x%x", hash[:])
}

// --- Internal helpers ---

// appendJSONString appends a "key":"value" pair to buf. If comma is true, prepends a comma.
func appendJSONString(buf []byte, key, value string, comma bool) []byte {
	if comma {
		buf = append(buf, ',')
	}
	keyBytes, _ := json.Marshal(key)
	valBytes, _ := json.Marshal(value)
	buf = append(buf, keyBytes...)
	buf = append(buf, ':')
	buf = append(buf, valBytes...)
	return buf
}

// isValidHexBytes checks for 0x-prefixed lowercase hex (any even length). 006 FR-W06.
func isValidHexBytes(s string) bool {
	if len(s) < 4 || s[0] != '0' || s[1] != 'x' {
		return false
	}
	hex := s[2:]
	if len(hex)%2 != 0 {
		return false
	}
	for _, c := range hex {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// isValidUnsignedInt256 checks that a string is a non-negative decimal integer.
// Per 006 FR-W02, UnsignedInt256 values are serialized as decimal strings.
func isValidUnsignedInt256(s string) bool {
	if len(s) == 0 {
		return false
	}
	// Must not have leading zeros (except "0" itself).
	if len(s) > 1 && s[0] == '0' {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// validateVersion checks version compatibility per FR-V07.
// Accept major 1, reject major >= 2.
func validateVersion(version string) error {
	parts := strings.SplitN(version, ".", 2)
	if len(parts) == 0 {
		return NewValidationError(ErrIncompatibleVersion,
			fmt.Sprintf("invalid version format %q", version))
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return NewValidationError(ErrIncompatibleVersion,
			fmt.Sprintf("invalid version major %q", parts[0]))
	}
	if major != 1 {
		return NewValidationError(ErrIncompatibleVersion,
			fmt.Sprintf("version major %d is not supported (only major 1 accepted)", major))
	}
	return nil
}

// IsValidSpecRef validates a spec reference. FR-V24.
// Accepts "NNN-short-name" format (e.g., "005-health") or custom domain strings.
func IsValidSpecRef(ref string) bool {
	if len(ref) == 0 {
		return false
	}
	// Accept NNN-name pattern or plain non-empty string (custom domain).
	// Must contain only alphanumeric characters and hyphens.
	for _, c := range ref {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '-') {
			return false
		}
	}
	return true
}
