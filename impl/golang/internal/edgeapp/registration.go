package edgeapp

import (
	"context"
	"errors"
	"fmt"
	"math/big"
)

// EnsureRegistered is the idempotent edge-side registration helper.
//
// Behavior:
//
//   - Adapter is nil ⇒ EnsureRegistration is disabled; returns
//     (nil, false, nil) so the seller startup can no-op cleanly.
//   - Existing registration found for ownerEVM:
//     - If AgentURIByTokenID matches the desired agentURI ⇒ no-op.
//       Returns (tokenID, false, nil).
//     - If it differs and update=true ⇒ UpdateAgentURI. Returns
//       (tokenID, false, nil) on success.
//     - If it differs and update=false ⇒ returns
//       (tokenID, false, ErrAgentURIMismatch).
//   - No existing registration ⇒ Register. Returns (newTokenID, true, nil).
//
// The "fresh" return flag is true exactly when this call minted a new
// registration. Callers that record the tokenID elsewhere (e.g. on-disk
// state, journal log) can branch on it.
func EnsureRegistered(
	ctx context.Context,
	adapter RegistryAdapter,
	ownerEVM string,
	agentURI string,
	update bool,
) (tokenID *big.Int, fresh bool, err error) {
	if adapter == nil {
		return nil, false, nil
	}
	if ownerEVM == "" {
		return nil, false, errors.New("ensure-registered: ownerEVM required")
	}
	if agentURI == "" {
		return nil, false, errors.New("ensure-registered: agentURI required")
	}

	if existing, found, lookupErr := adapter.LookupTokenID(ctx, ownerEVM); lookupErr == nil && found {
		current, uriErr := adapter.AgentURIByTokenID(ctx, existing)
		if uriErr != nil {
			return existing, false, fmt.Errorf("ensure-registered: read agentURI: %w", uriErr)
		}
		if current == agentURI {
			return existing, false, nil
		}
		if !update {
			return existing, false, ErrAgentURIMismatch
		}
		if _, updErr := adapter.UpdateAgentURI(ctx, ownerEVM, existing, agentURI); updErr != nil {
			return existing, false, fmt.Errorf("ensure-registered: update agentURI: %w", updErr)
		}
		return existing, false, nil
	} else if lookupErr != nil {
		return nil, false, fmt.Errorf("ensure-registered: lookup: %w", lookupErr)
	}

	tokenID, _, regErr := adapter.Register(ctx, ownerEVM, agentURI)
	if regErr != nil {
		// ErrAlreadyRegistered is a benign race the LookupTokenID guard
		// missed (or a memory-registry quirk where the entry was inserted
		// between Lookup and Register). Re-lookup and treat as
		// already-registered.
		if errors.Is(regErr, ErrAlreadyRegistered) {
			t, found, _ := adapter.LookupTokenID(ctx, ownerEVM)
			if found {
				return t, false, nil
			}
		}
		return nil, false, fmt.Errorf("ensure-registered: register: %w", regErr)
	}
	return tokenID, true, nil
}

// ErrAgentURIMismatch is returned by EnsureRegistered when an existing
// registration's agentURI differs from the requested one and the caller
// did not opt into updating it. The caller's options are: pass update=true,
// or treat the existing registration as authoritative and proceed without
// re-registering.
var ErrAgentURIMismatch = errors.New("ensure-registered: existing registration's agentURI differs (call with update=true to overwrite)")
