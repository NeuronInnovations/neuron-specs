package account

import (
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
)

// ---------------------------------------------------------------------------
// Intermediate JSON structs
// ---------------------------------------------------------------------------

// jsonAccount is the intermediate representation used for JSON encoding and
// decoding of NeuronAccount. Public keys are stored as compressed hex strings,
// EVM addresses as EIP-55 checksummed hex, and balances as decimal strings.
type jsonAccount struct {
	AccountType     string            `json:"accountType"`
	PublicKey       string            `json:"publicKey,omitempty"`
	EVMAddress      string            `json:"evmAddress,omitempty"`
	PeerID          string            `json:"peerID,omitempty"`
	DID             *jsonDID          `json:"did,omitempty"`
	ParentPubKey    string            `json:"parentPubKey,omitempty"`
	MultisigKey     *jsonMultisigKey  `json:"multisigKey,omitempty"`
	CurrencySymbol  string            `json:"currencySymbol"`
	CreditBalance   *string           `json:"creditBalance,omitempty"`
	BalanceAllocat  *string           `json:"balanceAllocation,omitempty"`
	Balance         *string           `json:"balance,omitempty"`
	LedgerAttach    *jsonLedgerAttach `json:"ledgerAttachment,omitempty"`
	RegistryBinding *RegistryBinding  `json:"registryBinding,omitempty"`
	FeePayer        *string           `json:"feePayer,omitempty"`
	P2PHost         string            `json:"p2pHost,omitempty"`
}

// jsonDID is the JSON representation of a NeuronDID.
type jsonDID struct {
	Identifier string `json:"identifier"`
}

// jsonMultisigKey is the JSON representation of a MultisigKey.
// Only metadata (protocol, threshold, totalKeys) is serialized; key material
// is never included in JSON output.
type jsonMultisigKey struct {
	Protocol  string `json:"protocol"`
	Threshold uint   `json:"threshold"`
	TotalKeys uint   `json:"totalKeys"`
}

// jsonLedgerAttach is the JSON representation of a LedgerAttachment.
type jsonLedgerAttach struct {
	LedgerIdentifier   string  `json:"ledgerIdentifier"`
	AttachedAddress    string  `json:"attachedAddress"`
	State              string  `json:"state"`
	VerificationStatus string  `json:"verificationStatus"`
	LastSyncedAt       *string `json:"lastSyncedAt,omitempty"`
}

// ---------------------------------------------------------------------------
// MarshalJSON
// ---------------------------------------------------------------------------

// MarshalJSON implements json.Marshaler for NeuronAccount. It converts the
// account to the intermediate jsonAccount representation before marshalling.
//
// Public keys are encoded as compressed hex with "0x" prefix.
// EVM addresses are encoded with EIP-55 checksumming.
// Balance fields are encoded as decimal strings.
// Timestamps are encoded in RFC 3339 format.
func (a *NeuronAccount) MarshalJSON() ([]byte, error) {
	ja := jsonAccount{
		AccountType:    a.accountType.String(),
		CurrencySymbol: a.currencySymbol,
	}

	// Public key (compressed hex).
	if a.publicKey != nil {
		ja.PublicKey = a.publicKey.Hex()
	}

	// EVM address (EIP-55 checksummed).
	if a.evmAddress != nil {
		ja.EVMAddress = a.evmAddress.Hex()
	}

	// PeerID.
	if a.peerID != nil {
		ja.PeerID = a.peerID.String()
	}

	// DID.
	if a.did != nil {
		ja.DID = &jsonDID{Identifier: a.did.identifier}
	}

	// Parent public key (compressed hex).
	if a.parentPubKey != nil {
		ja.ParentPubKey = a.parentPubKey.Hex()
	}

	// MultisigKey metadata.
	if a.multisigKey != nil {
		ja.MultisigKey = &jsonMultisigKey{
			Protocol:  a.multisigKey.Protocol(),
			Threshold: a.multisigKey.Threshold(),
			TotalKeys: a.multisigKey.TotalKeys(),
		}
	}

	// Balance fields as decimal strings.
	if a.creditBalance != nil {
		s := a.creditBalance.String()
		ja.CreditBalance = &s
	}
	if a.balanceAllocat != nil {
		s := a.balanceAllocat.String()
		ja.BalanceAllocat = &s
	}
	if a.balance != nil {
		s := a.balance.String()
		ja.Balance = &s
	}

	// Ledger attachment.
	if a.ledgerAttach != nil {
		jla := &jsonLedgerAttach{
			LedgerIdentifier:   a.ledgerAttach.ledgerIdentifier,
			AttachedAddress:    a.ledgerAttach.attachedAddress.Hex(),
			State:              a.ledgerAttach.state.String(),
			VerificationStatus: a.ledgerAttach.verificationStatus.String(),
		}
		if a.ledgerAttach.lastSyncedAt != nil {
			ts := a.ledgerAttach.lastSyncedAt.Format(time.RFC3339)
			jla.LastSyncedAt = &ts
		}
		ja.LedgerAttach = jla
	}

	// Registry binding (struct marshals directly).
	if a.registryBinding != nil {
		ja.RegistryBinding = a.registryBinding
	}

	// Fee payer.
	if a.feePayer != nil {
		s := string(*a.feePayer)
		ja.FeePayer = &s
	}

	// P2P host.
	if a.p2pHost != nil {
		ja.P2PHost = a.p2pHost.String()
	}

	return json.Marshal(ja)
}

// ---------------------------------------------------------------------------
// UnmarshalJSON
// ---------------------------------------------------------------------------

// UnmarshalJSON implements json.Unmarshaler for NeuronAccount. It reconstructs
// the account from the intermediate jsonAccount representation.
//
// For PublicKey and ParentPubKey fields, it uses keylib.NeuronPublicKeyFromHex
// and re-derives EVMAddress, PeerID, and P2PHost from the public key.
//
// MultisigKey is reconstructed with metadata only (protocol, threshold,
// totalKeys); key material is not recoverable from JSON.
func (a *NeuronAccount) UnmarshalJSON(data []byte) error {
	var ja jsonAccount
	if err := json.Unmarshal(data, &ja); err != nil {
		return fmt.Errorf("account unmarshal: %w", err)
	}

	// Parse account type.
	acctType, err := parseAccountType(ja.AccountType)
	if err != nil {
		return fmt.Errorf("account unmarshal: %w", err)
	}
	a.accountType = acctType

	// Parse public key and derive evmAddress, peerID.
	if ja.PublicKey != "" {
		pk, err := keylib.NeuronPublicKeyFromHex(ja.PublicKey)
		if err != nil {
			return fmt.Errorf("account unmarshal publicKey: %w", err)
		}
		a.publicKey = &pk

		evmAddr := pk.EVMAddress()
		a.evmAddress = &evmAddr

		peerID, err := pk.PeerID()
		if err != nil {
			return fmt.Errorf("account unmarshal peerID derivation: %w", err)
		}
		a.peerID = &peerID

		// For Child accounts, p2pHost = peerID.
		if acctType == Child {
			p2pHost := peerID
			a.p2pHost = &p2pHost
		}
	}

	// Parse DID.
	if ja.DID != nil {
		a.did = &NeuronDID{
			identifier: ja.DID.Identifier,
			document:   DIDDocument{},
		}
	}

	// Parse parent public key.
	if ja.ParentPubKey != "" {
		ppk, err := keylib.NeuronPublicKeyFromHex(ja.ParentPubKey)
		if err != nil {
			return fmt.Errorf("account unmarshal parentPubKey: %w", err)
		}
		a.parentPubKey = &ppk
	}

	// Parse multisig key metadata (no key material reconstruction).
	if ja.MultisigKey != nil {
		mk := keylib.MultisigKeyFromMetadata(
			ja.MultisigKey.Protocol,
			ja.MultisigKey.Threshold,
			ja.MultisigKey.TotalKeys,
		)
		a.multisigKey = &mk
	}

	// Currency symbol.
	a.currencySymbol = ja.CurrencySymbol

	// Parse balance fields.
	if ja.CreditBalance != nil {
		bi, ok := new(big.Int).SetString(*ja.CreditBalance, 10)
		if !ok {
			return fmt.Errorf("account unmarshal: invalid creditBalance %q", *ja.CreditBalance)
		}
		a.creditBalance = bi
	}
	if ja.BalanceAllocat != nil {
		bi, ok := new(big.Int).SetString(*ja.BalanceAllocat, 10)
		if !ok {
			return fmt.Errorf("account unmarshal: invalid balanceAllocation %q", *ja.BalanceAllocat)
		}
		a.balanceAllocat = bi
	}
	if ja.Balance != nil {
		bi, ok := new(big.Int).SetString(*ja.Balance, 10)
		if !ok {
			return fmt.Errorf("account unmarshal: invalid balance %q", *ja.Balance)
		}
		a.balance = bi
	}

	// Parse ledger attachment.
	if ja.LedgerAttach != nil {
		la, err := parseLedgerAttachment(ja.LedgerAttach)
		if err != nil {
			return fmt.Errorf("account unmarshal ledgerAttachment: %w", err)
		}
		a.ledgerAttach = la
	}

	// Registry binding.
	if ja.RegistryBinding != nil {
		rb := *ja.RegistryBinding
		a.registryBinding = &rb
	}

	// Fee payer.
	if ja.FeePayer != nil {
		fp := LedgerAccountId(*ja.FeePayer)
		a.feePayer = &fp
	}

	return nil
}

// ---------------------------------------------------------------------------
// Convenience functions
// ---------------------------------------------------------------------------

// FR-015: JSON serialization/deserialization
// Serialize marshals a NeuronAccount to JSON bytes.
// Returns an error if the account cannot be serialized.
func Serialize(a *NeuronAccount) ([]byte, error) {
	return json.Marshal(a)
}

// Deserialize unmarshals JSON bytes into a new NeuronAccount.
// Returns an error if the JSON is invalid or the account cannot be reconstructed.
func Deserialize(data []byte) (*NeuronAccount, error) {
	var a NeuronAccount
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, err
	}
	if violations := a.Validate(); len(violations) > 0 {
		return nil, fmt.Errorf("account validation failed: %s", violations[0].Error())
	}
	return &a, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// parseAccountType converts a string to an AccountType.
func parseAccountType(s string) (AccountType, error) {
	switch s {
	case "Parent":
		return Parent, nil
	case "Child":
		return Child, nil
	case "Shared":
		return Shared, nil
	default:
		return Unspecified, fmt.Errorf("unknown account type %q", s)
	}
}

// parseAttachmentState converts a string to an AttachmentState.
func parseAttachmentState(s string) (AttachmentState, error) {
	switch s {
	case "Attached":
		return Attached, nil
	case "Detached":
		return Detached, nil
	default:
		return Detached, fmt.Errorf("unknown attachment state %q", s)
	}
}

// parseVerificationStatus converts a string to a VerificationStatus.
func parseVerificationStatus(s string) (VerificationStatus, error) {
	switch s {
	case "Unverified":
		return Unverified, nil
	case "Verified":
		return Verified, nil
	case "Failed":
		return Failed, nil
	default:
		return Unverified, fmt.Errorf("unknown verification status %q", s)
	}
}

// parseLedgerAttachment converts a jsonLedgerAttach to a LedgerAttachment.
func parseLedgerAttachment(jla *jsonLedgerAttach) (*LedgerAttachment, error) {
	addr, err := keylib.EVMAddressFromHex(jla.AttachedAddress)
	if err != nil {
		return nil, fmt.Errorf("invalid attachedAddress: %w", err)
	}

	state, err := parseAttachmentState(jla.State)
	if err != nil {
		return nil, err
	}

	vs, err := parseVerificationStatus(jla.VerificationStatus)
	if err != nil {
		return nil, err
	}

	la := &LedgerAttachment{
		ledgerIdentifier:   jla.LedgerIdentifier,
		attachedAddress:    addr,
		state:              state,
		verificationStatus: vs,
	}

	if jla.LastSyncedAt != nil {
		ts, err := time.Parse(time.RFC3339, *jla.LastSyncedAt)
		if err != nil {
			return nil, fmt.Errorf("invalid lastSyncedAt: %w", err)
		}
		la.lastSyncedAt = &ts
	}

	return la, nil
}

