// Package account implements the NeuronAccount module (Spec 001), providing
// a hierarchical account system with three account types:
//
//   - Parent: Root identity with DID, credit balance capability, and currency.
//   - Child: Agent/device identity referencing a Parent, with p2p host identity,
//     balance allocation, and registry binding.
//   - Shared: Multisig threshold account with MultisigKey from Spec 002.
//
// Accounts are constructed using type-safe fluent builders:
//
//	parent, err := account.NewParentAccountBuilder().
//	    WithPublicKey(pubKey).
//	    WithDID(did).
//	    WithCurrency("ETH").
//	    Build()
//
// All cryptographic types (NeuronPublicKey, EVMAddress, PeerID, MultisigKey)
// are imported from internal/keylib (Spec 002). Ledger interaction is injectable
// via the LedgerVerifier and ParentChildVerifier interfaces.
//
// Balance is never set at construction — only via ledger sync. Reachability and
// communication data are not part of the account module (see Spec 003).
package account
