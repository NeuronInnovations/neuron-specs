package registry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegistryUnavailable(t *testing.T) {
	tests := []struct {
		name     string
		err      RegistryUnavailable
		wantMsg  string
		wantImpl bool
	}{
		{
			name:     "rpc error",
			err:      RegistryUnavailable{Detail: "connection refused on 127.0.0.1:8545"},
			wantMsg:  "registry unavailable: connection refused on 127.0.0.1:8545",
			wantImpl: true,
		},
		{
			name:     "contract address",
			err:      RegistryUnavailable{Detail: "0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28"},
			wantMsg:  "registry unavailable: 0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28",
			wantImpl: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var iface error = tc.err
			assert.Equal(t, tc.wantMsg, iface.Error())
			assert.Equal(t, tc.err.Detail, tc.err.Detail)
		})
	}
}

func TestRegistrationNotFound(t *testing.T) {
	err := RegistrationNotFound{Detail: "0xabc123"}
	var iface error = err
	assert.Equal(t, "registration not found: 0xabc123", iface.Error())
	assert.Equal(t, "0xabc123", err.Detail)
}

func TestIncompleteRegistration(t *testing.T) {
	err := IncompleteRegistration{Detail: "missing stdIn topic service"}
	var iface error = err
	assert.Equal(t, "incomplete registration: missing stdIn topic service", iface.Error())
	assert.Equal(t, "missing stdIn topic service", err.Detail)
}

func TestProofOfControlFailed(t *testing.T) {
	err := ProofOfControlFailed{
		Expected: "0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		Actual:   "0xBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
	}
	var iface error = err
	assert.Contains(t, iface.Error(), "proof of control failed")
	assert.Contains(t, iface.Error(), "0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	assert.Contains(t, iface.Error(), "0xBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB")
	assert.Equal(t, "0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", err.Expected)
	assert.Equal(t, "0xBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB", err.Actual)
}

func TestAdmissionDenied(t *testing.T) {
	err := AdmissionDenied{Detail: "platform policy requires KYC"}
	var iface error = err
	assert.Equal(t, "admission denied: platform policy requires KYC", iface.Error())
	assert.Equal(t, "platform policy requires KYC", err.Detail)
}

func TestDuplicateRegistration(t *testing.T) {
	err := DuplicateRegistration{
		ChildAddress:    "0x1111111111111111111111111111111111111111",
		RegistryAddress: "0x2222222222222222222222222222222222222222",
	}
	var iface error = err
	assert.Contains(t, iface.Error(), "duplicate registration")
	assert.Contains(t, iface.Error(), "0x1111111111111111111111111111111111111111")
	assert.Contains(t, iface.Error(), "0x2222222222222222222222222222222222222222")
	assert.Equal(t, "0x1111111111111111111111111111111111111111", err.ChildAddress)
	assert.Equal(t, "0x2222222222222222222222222222222222222222", err.RegistryAddress)
}

func TestInvalidDIDService(t *testing.T) {
	err := InvalidDIDService{Detail: "endpoint URL is empty"}
	var iface error = err
	assert.Equal(t, "invalid DID service: endpoint URL is empty", iface.Error())
	assert.Equal(t, "endpoint URL is empty", err.Detail)
}

func TestBrokenTopicRef(t *testing.T) {
	err := BrokenTopicRef{
		TopicRef:       "nonexistent-topic",
		AvailableNames: []string{"stdIn", "stdOut", "stdErr"},
	}
	var iface error = err
	assert.Contains(t, iface.Error(), "broken topic ref")
	assert.Contains(t, iface.Error(), "nonexistent-topic")
	assert.Contains(t, iface.Error(), "stdIn")
	assert.Equal(t, "nonexistent-topic", err.TopicRef)
	assert.Equal(t, []string{"stdIn", "stdOut", "stdErr"}, err.AvailableNames)
}

func TestBrokenTopicRef_EmptyNames(t *testing.T) {
	err := BrokenTopicRef{
		TopicRef:       "missing",
		AvailableNames: []string{},
	}
	assert.Contains(t, err.Error(), "missing")
	assert.Empty(t, err.AvailableNames)
}

func TestInvalidServiceSchema(t *testing.T) {
	err := InvalidServiceSchema{
		ServiceType: "neuron-topic",
		Field:       "transport",
		Detail:      "must be one of [hcs, libp2p]",
	}
	var iface error = err
	assert.Contains(t, iface.Error(), "invalid service schema")
	assert.Contains(t, iface.Error(), "neuron-topic")
	assert.Contains(t, iface.Error(), "transport")
	assert.Contains(t, iface.Error(), "must be one of [hcs, libp2p]")
	assert.Equal(t, "neuron-topic", err.ServiceType)
	assert.Equal(t, "transport", err.Field)
	assert.Equal(t, "must be one of [hcs, libp2p]", err.Detail)
}

func TestUnauthorizedOperation(t *testing.T) {
	err := UnauthorizedOperation{
		CallerRole: "REGISTERED_AGENT",
		Operation:  "burn",
	}
	var iface error = err
	assert.Contains(t, iface.Error(), "unauthorized operation")
	assert.Contains(t, iface.Error(), "REGISTERED_AGENT")
	assert.Contains(t, iface.Error(), "burn")
	assert.Equal(t, "REGISTERED_AGENT", err.CallerRole)
	assert.Equal(t, "burn", err.Operation)
}

func TestAllowlistRejection(t *testing.T) {
	did := "did:key:zQ3shunBKsXmLGEBU3JcUY5JjFCkEXvMAFYVPYMBaDqMTAnWz"
	err := AllowlistRejection{ParentDID: did}
	var iface error = err
	assert.Contains(t, iface.Error(), "allowlist rejection")
	assert.Contains(t, iface.Error(), did)
	assert.Equal(t, did, err.ParentDID)
}

// TestAllErrorsImplementErrorInterface verifies every error type satisfies
// the error interface via a compile-time type assertion.
func TestAllErrorsImplementErrorInterface(t *testing.T) {
	var _ error = RegistryUnavailable{}
	var _ error = RegistrationNotFound{}
	var _ error = IncompleteRegistration{}
	var _ error = ProofOfControlFailed{}
	var _ error = AdmissionDenied{}
	var _ error = DuplicateRegistration{}
	var _ error = InvalidDIDService{}
	var _ error = BrokenTopicRef{}
	var _ error = InvalidServiceSchema{}
	var _ error = UnauthorizedOperation{}
	var _ error = AllowlistRejection{}
}
