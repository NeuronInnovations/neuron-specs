package registry

import (
	"testing"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Helper: build a fully valid AgentURI for a given public key ---

func validAgentURIForKey(t *testing.T, pub keylib.NeuronPublicKey) AgentURI {
	t.Helper()
	peerID, err := pub.PeerID()
	require.NoError(t, err)

	stdIn, err := NewNeuronTopicService("stdIn", "1.0.0", "stdIn", "hcs", "hedera-mainnet",
		map[string]any{"topicId": "0.0.1"})
	require.NoError(t, err)

	stdOut, err := NewNeuronTopicService("stdOut", "1.0.0", "stdOut", "hcs", "hedera-mainnet",
		map[string]any{"topicId": "0.0.2"})
	require.NoError(t, err)

	stdErr, err := NewNeuronTopicService("stdErr", "1.0.0", "stdErr", "hcs", "hedera-mainnet",
		map[string]any{"topicId": "0.0.3"})
	require.NoError(t, err)

	p2p, err := NewNeuronP2PExchangeService("p2p", "1.0.0", peerID.String(),
		"/neuron/multiaddr-exchange/1.0.0", "stdIn")
	require.NoError(t, err)

	didSvc, err := NewDIDService(pub)
	require.NoError(t, err)

	return AgentURI{
		topicServices: []NeuronTopicService{stdIn, stdOut, stdErr},
		p2pServices:   []NeuronP2PExchangeService{p2p},
		didServices:   []DIDService{didSvc},
	}
}

// --- ValidateRegistrationCompleteness Tests (T028-T029, T038) ---

func TestValidateCompleteness_Valid(t *testing.T) {
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	pub := childKey.PublicKey()
	uri := validAgentURIForKey(t, pub)

	valid, errors := ValidateRegistrationCompleteness(uri, pub)
	assert.True(t, valid)
	assert.Nil(t, errors)
}

func TestValidateCompleteness_MissingTopicServices(t *testing.T) {
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	pub := childKey.PublicKey()
	peerID, err := pub.PeerID()
	require.NoError(t, err)

	p2p, err := NewNeuronP2PExchangeService("p2p", "1.0.0", peerID.String(),
		"/neuron/multiaddr-exchange/1.0.0", "stdIn")
	require.NoError(t, err)

	// Only 1 topic service instead of 3.
	stdIn, err := NewNeuronTopicService("stdIn", "1.0.0", "stdIn", "hcs", "hedera-mainnet",
		map[string]any{"topicId": "0.0.1"})
	require.NoError(t, err)

	uri := AgentURI{
		topicServices: []NeuronTopicService{stdIn},
		p2pServices:   []NeuronP2PExchangeService{p2p},
	}

	valid, errs := ValidateRegistrationCompleteness(uri, pub)
	assert.False(t, valid)

	hasRule := false
	for _, e := range errs {
		if e.Rule == "V-REG-01" {
			hasRule = true
		}
	}
	assert.True(t, hasRule, "expected V-REG-01 violation")
}

func TestValidateCompleteness_MissingP2PService(t *testing.T) {
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	pub := childKey.PublicKey()

	stdIn, _ := NewNeuronTopicService("stdIn", "1.0.0", "stdIn", "hcs", "hedera", map[string]any{"t": "1"})
	stdOut, _ := NewNeuronTopicService("stdOut", "1.0.0", "stdOut", "hcs", "hedera", map[string]any{"t": "2"})
	stdErr, _ := NewNeuronTopicService("stdErr", "1.0.0", "stdErr", "hcs", "hedera", map[string]any{"t": "3"})

	uri := AgentURI{
		topicServices: []NeuronTopicService{stdIn, stdOut, stdErr},
		p2pServices:   nil, // No P2P services.
	}

	valid, errs := ValidateRegistrationCompleteness(uri, pub)
	assert.False(t, valid)

	hasRule := false
	for _, e := range errs {
		if e.Rule == "V-REG-02" {
			hasRule = true
		}
	}
	assert.True(t, hasRule, "expected V-REG-02 violation")
}

func TestValidateCompleteness_TopicMissingFields(t *testing.T) {
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	pub := childKey.PublicKey()
	peerID, err := pub.PeerID()
	require.NoError(t, err)

	// Topic with empty transport field.
	badTopic := NeuronTopicService{
		Type:    ServiceTypeNeuronTopic,
		Name:    "stdIn",
		Version: "1.0.0",
		Channel: "stdIn",
		// Transport is missing.
		Anchor: "hedera",
		Config: map[string]any{"t": "1"},
	}
	stdOut, _ := NewNeuronTopicService("stdOut", "1.0.0", "stdOut", "hcs", "hedera", map[string]any{"t": "2"})
	stdErr, _ := NewNeuronTopicService("stdErr", "1.0.0", "stdErr", "hcs", "hedera", map[string]any{"t": "3"})
	p2p, _ := NewNeuronP2PExchangeService("p2p", "1.0.0", peerID.String(), "/proto", "stdIn")

	uri := AgentURI{
		topicServices: []NeuronTopicService{badTopic, stdOut, stdErr},
		p2pServices:   []NeuronP2PExchangeService{p2p},
	}

	valid, errs := ValidateRegistrationCompleteness(uri, pub)
	assert.False(t, valid)

	hasV03 := false
	for _, e := range errs {
		if e.Rule == "V-REG-03" {
			hasV03 = true
		}
	}
	assert.True(t, hasV03, "expected V-REG-03 violation for missing transport")
}

func TestValidateCompleteness_P2PMissingFields(t *testing.T) {
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	pub := childKey.PublicKey()

	stdIn, _ := NewNeuronTopicService("stdIn", "1.0.0", "stdIn", "hcs", "hedera", map[string]any{"t": "1"})
	stdOut, _ := NewNeuronTopicService("stdOut", "1.0.0", "stdOut", "hcs", "hedera", map[string]any{"t": "2"})
	stdErr, _ := NewNeuronTopicService("stdErr", "1.0.0", "stdErr", "hcs", "hedera", map[string]any{"t": "3"})

	// P2P with empty peerID.
	badP2P := NeuronP2PExchangeService{
		Type:     ServiceTypeNeuronP2PExchange,
		Name:     "p2p",
		Version:  "1.0.0",
		PeerID:   "", // Missing.
		Protocol: "/proto",
		TopicRef: "stdIn",
	}

	uri := AgentURI{
		topicServices: []NeuronTopicService{stdIn, stdOut, stdErr},
		p2pServices:   []NeuronP2PExchangeService{badP2P},
	}

	valid, errs := ValidateRegistrationCompleteness(uri, pub)
	assert.False(t, valid)

	hasV04 := false
	for _, e := range errs {
		if e.Rule == "V-REG-04" {
			hasV04 = true
		}
	}
	assert.True(t, hasV04, "expected V-REG-04 violation for missing peerID")
}

func TestValidateCompleteness_BrokenTopicRef(t *testing.T) {
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	pub := childKey.PublicKey()
	peerID, err := pub.PeerID()
	require.NoError(t, err)

	stdIn, _ := NewNeuronTopicService("stdIn", "1.0.0", "stdIn", "hcs", "hedera", map[string]any{"t": "1"})
	stdOut, _ := NewNeuronTopicService("stdOut", "1.0.0", "stdOut", "hcs", "hedera", map[string]any{"t": "2"})
	stdErr, _ := NewNeuronTopicService("stdErr", "1.0.0", "stdErr", "hcs", "hedera", map[string]any{"t": "3"})

	// P2P with a topicRef that doesn't match any topic name.
	p2p := NeuronP2PExchangeService{
		Type:     ServiceTypeNeuronP2PExchange,
		Name:     "p2p",
		Version:  "1.0.0",
		PeerID:   peerID.String(),
		Protocol: "/proto",
		TopicRef: "nonExistent", // Does not match any topic service name.
	}

	uri := AgentURI{
		topicServices: []NeuronTopicService{stdIn, stdOut, stdErr},
		p2pServices:   []NeuronP2PExchangeService{p2p},
	}

	valid, errs := ValidateRegistrationCompleteness(uri, pub)
	assert.False(t, valid)

	hasV05 := false
	for _, e := range errs {
		if e.Rule == "V-REG-05" {
			hasV05 = true
		}
	}
	assert.True(t, hasV05, "expected V-REG-05 violation for broken topicRef")
}

func TestValidateCompleteness_InvalidDID(t *testing.T) {
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	pub := childKey.PublicKey()
	peerID, err := pub.PeerID()
	require.NoError(t, err)

	stdIn, _ := NewNeuronTopicService("stdIn", "1.0.0", "stdIn", "hcs", "hedera", map[string]any{"t": "1"})
	stdOut, _ := NewNeuronTopicService("stdOut", "1.0.0", "stdOut", "hcs", "hedera", map[string]any{"t": "2"})
	stdErr, _ := NewNeuronTopicService("stdErr", "1.0.0", "stdErr", "hcs", "hedera", map[string]any{"t": "3"})
	p2p, _ := NewNeuronP2PExchangeService("p2p", "1.0.0", peerID.String(), "/proto", "stdIn")

	// DID with wrong endpoint (doesn't match childPublicKey.DIDKey()).
	wrongDID := DIDService{
		Type:     ServiceTypeDID,
		Name:     "DID",
		Endpoint: "did:key:zQ3sWRONG",
		Version:  "v1",
	}

	uri := AgentURI{
		topicServices: []NeuronTopicService{stdIn, stdOut, stdErr},
		p2pServices:   []NeuronP2PExchangeService{p2p},
		didServices:   []DIDService{wrongDID},
	}

	valid, errs := ValidateRegistrationCompleteness(uri, pub)
	assert.False(t, valid)

	hasV06 := false
	for _, e := range errs {
		if e.Rule == "V-REG-06" {
			hasV06 = true
		}
	}
	assert.True(t, hasV06, "expected V-REG-06 violation for invalid DID")
}

func TestValidateCompleteness_MultipleDIDServices(t *testing.T) {
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	pub := childKey.PublicKey()
	peerID, err := pub.PeerID()
	require.NoError(t, err)

	stdIn, _ := NewNeuronTopicService("stdIn", "1.0.0", "stdIn", "hcs", "hedera", map[string]any{"t": "1"})
	stdOut, _ := NewNeuronTopicService("stdOut", "1.0.0", "stdOut", "hcs", "hedera", map[string]any{"t": "2"})
	stdErr, _ := NewNeuronTopicService("stdErr", "1.0.0", "stdErr", "hcs", "hedera", map[string]any{"t": "3"})
	p2p, _ := NewNeuronP2PExchangeService("p2p", "1.0.0", peerID.String(), "/proto", "stdIn")

	didSvc, _ := NewDIDService(pub)

	uri := AgentURI{
		topicServices: []NeuronTopicService{stdIn, stdOut, stdErr},
		p2pServices:   []NeuronP2PExchangeService{p2p},
		didServices:   []DIDService{didSvc, didSvc}, // Two DID services.
	}

	valid, errs := ValidateRegistrationCompleteness(uri, pub)
	assert.False(t, valid)

	hasV07 := false
	for _, e := range errs {
		if e.Rule == "V-REG-07" {
			hasV07 = true
		}
	}
	assert.True(t, hasV07, "expected V-REG-07 violation for multiple DID services")
}

func TestValidateCompleteness_DuplicateChannel(t *testing.T) {
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	pub := childKey.PublicKey()
	peerID, err := pub.PeerID()
	require.NoError(t, err)

	// Two stdIn, one stdOut, missing stdErr.
	stdIn1, _ := NewNeuronTopicService("stdIn-1", "1.0.0", "stdIn", "hcs", "hedera", map[string]any{"t": "1"})
	stdIn2, _ := NewNeuronTopicService("stdIn-2", "1.0.0", "stdIn", "hcs", "hedera", map[string]any{"t": "2"})
	stdOut, _ := NewNeuronTopicService("stdOut", "1.0.0", "stdOut", "hcs", "hedera", map[string]any{"t": "3"})
	p2p, _ := NewNeuronP2PExchangeService("p2p", "1.0.0", peerID.String(), "/proto", "stdIn-1")

	uri := AgentURI{
		topicServices: []NeuronTopicService{stdIn1, stdIn2, stdOut},
		p2pServices:   []NeuronP2PExchangeService{p2p},
	}

	valid, errs := ValidateRegistrationCompleteness(uri, pub)
	assert.False(t, valid)

	hasV11 := false
	for _, e := range errs {
		if e.Rule == "V-REG-11" {
			hasV11 = true
		}
	}
	assert.True(t, hasV11, "expected V-REG-11 violation for duplicate/missing channels")
}

func TestValidateCompleteness_PeerIDMismatch(t *testing.T) {
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	pub := childKey.PublicKey()

	stdIn, _ := NewNeuronTopicService("stdIn", "1.0.0", "stdIn", "hcs", "hedera", map[string]any{"t": "1"})
	stdOut, _ := NewNeuronTopicService("stdOut", "1.0.0", "stdOut", "hcs", "hedera", map[string]any{"t": "2"})
	stdErr, _ := NewNeuronTopicService("stdErr", "1.0.0", "stdErr", "hcs", "hedera", map[string]any{"t": "3"})

	// P2P with a wrong peerID.
	p2p := NeuronP2PExchangeService{
		Type:     ServiceTypeNeuronP2PExchange,
		Name:     "p2p",
		Version:  "1.0.0",
		PeerID:   "12D3KooWWRONG",
		Protocol: "/proto",
		TopicRef: "stdIn",
	}

	uri := AgentURI{
		topicServices: []NeuronTopicService{stdIn, stdOut, stdErr},
		p2pServices:   []NeuronP2PExchangeService{p2p},
	}

	valid, errs := ValidateRegistrationCompleteness(uri, pub)
	assert.False(t, valid)

	hasV12 := false
	for _, e := range errs {
		if e.Rule == "V-REG-12" {
			hasV12 = true
		}
	}
	assert.True(t, hasV12, "expected V-REG-12 violation for peerID mismatch")
}

func TestValidateCompleteness_MixedTransportsValid(t *testing.T) {
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	pub := childKey.PublicKey()
	peerID, err := pub.PeerID()
	require.NoError(t, err)

	// Mix of transports: hcs, kafka, libp2p.
	stdIn, _ := NewNeuronTopicService("stdIn", "1.0.0", "stdIn", "hcs", "hedera-mainnet", map[string]any{"topicId": "0.0.1"})
	stdOut, _ := NewNeuronTopicService("stdOut", "1.0.0", "stdOut", "kafka", "kafka-cluster", map[string]any{"broker": "kafka:9092"})
	stdErr, _ := NewNeuronTopicService("stdErr", "1.0.0", "stdErr", "libp2p", "libp2p-net", map[string]any{"relay": true})
	p2p, _ := NewNeuronP2PExchangeService("p2p", "1.0.0", peerID.String(), "/neuron/multiaddr-exchange/1.0.0", "stdIn")
	didSvc, _ := NewDIDService(pub)

	uri := AgentURI{
		topicServices: []NeuronTopicService{stdIn, stdOut, stdErr},
		p2pServices:   []NeuronP2PExchangeService{p2p},
		didServices:   []DIDService{didSvc},
	}

	valid, errs := ValidateRegistrationCompleteness(uri, pub)
	assert.True(t, valid, "mixed transports should be valid, got errors: %v", errs)
	assert.Nil(t, errs)
}

// --- ValidateRoleBoundary Tests (T045-T046) ---

func TestValidateRoleBoundary_AdminCannotRegister(t *testing.T) {
	tests := []struct {
		operation string
		wantErr   bool
	}{
		{"register", true},
		{"updateAgentURI", true},
		{"transfer", true},
		{"burn", false},
		{"configure", false},
	}

	for _, tc := range tests {
		t.Run(tc.operation, func(t *testing.T) {
			err := ValidateRoleBoundary(RegistryAdmin, tc.operation)
			if tc.wantErr {
				require.Error(t, err)
				var unauth UnauthorizedOperation
				assert.ErrorAs(t, err, &unauth)
				assert.Equal(t, "REGISTRY_ADMIN", unauth.CallerRole)
				assert.Equal(t, tc.operation, unauth.Operation)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateRoleBoundary_AgentCanUpdateOwn(t *testing.T) {
	// Agent CAN updateAgentURI and burn.
	assert.NoError(t, ValidateRoleBoundary(RegisteredAgent, "updateAgentURI"))
	assert.NoError(t, ValidateRoleBoundary(RegisteredAgent, "burn"))

	// Agent MUST NOT register.
	err := ValidateRoleBoundary(RegisteredAgent, "register")
	require.Error(t, err)
	var unauth UnauthorizedOperation
	assert.ErrorAs(t, err, &unauth)
}

func TestValidateRoleBoundary_OperatorCanUpdate(t *testing.T) {
	// Operator CAN updateAgentURI.
	assert.NoError(t, ValidateRoleBoundary(DelegatedOperator, "updateAgentURI"))

	// Operator MUST NOT register.
	err := ValidateRoleBoundary(DelegatedOperator, "register")
	require.Error(t, err)
	var unauth UnauthorizedOperation
	assert.ErrorAs(t, err, &unauth)
}

func TestValidateRoleBoundary_ParentCannotRegister(t *testing.T) {
	// Parent MUST NOT register or updateAgentURI.
	err := ValidateRoleBoundary(ParentRole, "register")
	require.Error(t, err)
	var unauth UnauthorizedOperation
	assert.ErrorAs(t, err, &unauth)

	err = ValidateRoleBoundary(ParentRole, "updateAgentURI")
	require.Error(t, err)
	assert.ErrorAs(t, err, &unauth)

	// Parent CAN burn (not explicitly forbidden).
	assert.NoError(t, ValidateRoleBoundary(ParentRole, "burn"))
}

// --- Admission Policy Tests ---

func TestPermissionlessPolicy_AlwaysAdmits(t *testing.T) {
	policy := PermissionlessPolicy{}

	admitted, err := policy.IsAdmitted("0xchild", "did:key:zQ3sParent")
	assert.NoError(t, err)
	assert.True(t, admitted)

	// Even with empty values.
	admitted, err = policy.IsAdmitted("", "")
	assert.NoError(t, err)
	assert.True(t, admitted)
}

func TestAllowlistPolicy_AdmitsListed(t *testing.T) {
	policy := NewAllowlistPolicy()
	policy.AddParentDID("did:key:zQ3sAllowed")

	admitted, err := policy.IsAdmitted("0xchild", "did:key:zQ3sAllowed")
	assert.NoError(t, err)
	assert.True(t, admitted)
}

func TestAllowlistPolicy_RejectsUnlisted(t *testing.T) {
	policy := NewAllowlistPolicy()
	policy.AddParentDID("did:key:zQ3sAllowed")

	admitted, err := policy.IsAdmitted("0xchild", "did:key:zQ3sRejected")
	assert.Error(t, err)
	assert.False(t, admitted)

	var rejection AllowlistRejection
	assert.ErrorAs(t, err, &rejection)
	assert.Equal(t, "did:key:zQ3sRejected", rejection.ParentDID)
}

func TestAllowlistPolicy_AddRemove(t *testing.T) {
	policy := NewAllowlistPolicy()

	did := "did:key:zQ3sTest"

	// Initially not on the list.
	assert.False(t, policy.Contains(did))

	// Add.
	policy.AddParentDID(did)
	assert.True(t, policy.Contains(did))

	// Admitted now.
	admitted, err := policy.IsAdmitted("0xchild", did)
	assert.NoError(t, err)
	assert.True(t, admitted)

	// Remove.
	policy.RemoveParentDID(did)
	assert.False(t, policy.Contains(did))

	// No longer admitted.
	admitted, err = policy.IsAdmitted("0xchild", did)
	assert.Error(t, err)
	assert.False(t, admitted)
}

// --- Validation collects ALL violations (never short-circuits) ---

func TestValidateCompleteness_CollectsAllViolations(t *testing.T) {
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	pub := childKey.PublicKey()

	// Completely empty AgentURI should trigger multiple rules.
	uri := AgentURI{}

	valid, errs := ValidateRegistrationCompleteness(uri, pub)
	assert.False(t, valid)

	// Should have at least V-REG-01 (missing topics), V-REG-02 (missing p2p),
	// and V-REG-11 (missing channels).
	rules := make(map[string]bool)
	for _, e := range errs {
		rules[e.Rule] = true
	}
	assert.True(t, rules["V-REG-01"], "expected V-REG-01")
	assert.True(t, rules["V-REG-02"], "expected V-REG-02")
	assert.True(t, rules["V-REG-11"], "expected V-REG-11")
	assert.True(t, len(errs) >= 3, "expected at least 3 violations, got %d", len(errs))
}
