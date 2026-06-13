package account

import (
	"fmt"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
)

// FR-023: Parent payment address = evmAddress
// FR-024: Child payment address = parent evmAddress
// PaymentAddress resolves the payment address for this account.
//
// Resolution rules by account type:
//   - Parent: returns the account's own derived EVM address.
//   - Child: derives the Parent's EVM address from parentPubKey.
//   - Shared: returns an error because shared accounts have no single payment address.
func (a *NeuronAccount) PaymentAddress() (keylib.EVMAddress, error) {
	switch a.accountType {
	case Parent:
		if a.evmAddress == nil {
			return keylib.EVMAddress{}, fmt.Errorf("Parent account has no derived EVM address")
		}
		return *a.evmAddress, nil

	case Child:
		if a.parentPubKey == nil {
			return keylib.EVMAddress{}, fmt.Errorf("Child account has no parent public key")
		}
		parentAddr := a.parentPubKey.EVMAddress()
		return parentAddr, nil

	case Shared:
		return keylib.EVMAddress{}, fmt.Errorf("Shared accounts have no payment address")

	default:
		return keylib.EVMAddress{}, fmt.Errorf("unspecified account type has no payment address")
	}
}
