package payment

import (
	"github.com/ethereum/go-ethereum/crypto"
)

// AgreementState represents the lifecycle state of a buyer-seller agreement.
// FR-P13: 10 states.
type AgreementState string

const (
	StateIdle        AgreementState = "IDLE"
	StateRequested   AgreementState = "REQUESTED"
	StateNegotiating AgreementState = "NEGOTIATING"
	StateAgreed      AgreementState = "AGREED"
	StateFunded      AgreementState = "FUNDED"
	StateActive      AgreementState = "ACTIVE"
	StateInvoiced    AgreementState = "INVOICED"
	StateCompleted   AgreementState = "COMPLETED"
	StateTerminated  AgreementState = "TERMINATED"
	StateRejected    AgreementState = "REJECTED"
)

// AgreementEvent triggers a state transition. FR-P14.
type AgreementEvent string

const (
	EventServiceRequest  AgreementEvent = "SERVICE_REQUEST"
	EventAccept          AgreementEvent = "ACCEPT"
	EventReject          AgreementEvent = "REJECT"
	EventCounter         AgreementEvent = "COUNTER"
	EventDeadlineExpired AgreementEvent = "DEADLINE_EXPIRED"
	EventWithdraw        AgreementEvent = "WITHDRAW"
	EventEscrowCreated   AgreementEvent = "ESCROW_CREATED"
	EventDeliveryStarted AgreementEvent = "DELIVERY_STARTED"
	EventInvoice         AgreementEvent = "INVOICE"
	EventInvoiceApproved AgreementEvent = "INVOICE_APPROVED"
	EventInvoiceRefused  AgreementEvent = "INVOICE_REFUSED"
	EventTerminate       AgreementEvent = "TERMINATE"
	EventComplete        AgreementEvent = "COMPLETE"
	EventTimeout         AgreementEvent = "TIMEOUT"
	// Lifecycle events added 2026-05-08 per 008 FR-P36/P37/P38.
	// EventStop is triggered by receipt of a serviceStop payload (FR-P36) and
	// transitions ACTIVE → TERMINATED. Distinct from EventTerminate (the
	// generic local-SDK terminate) because EventStop has a wire-level cause
	// and observers can correlate the transition to the on-topic message.
	EventStop AgreementEvent = "STOP"
	// EventCancel is triggered by receipt of a serviceCancel payload (FR-P37)
	// and transitions any of {REQUESTED, NEGOTIATING, AGREED, FUNDED, ACTIVE,
	// INVOICED} → TERMINATED. Pre-AGREED transitions via cancel are observer-
	// equivalent to REJECTED, but the wire signal is serviceCancel rather
	// than serviceResponse(action=reject).
	EventCancel AgreementEvent = "CANCEL"
)

// transition defines a valid state transition.
type transition struct {
	from  AgreementState
	event AgreementEvent
	to    AgreementState
}

// transitions defines all valid state transitions per FR-P14.
var transitions = []transition{
	{StateIdle, EventServiceRequest, StateRequested},
	{StateRequested, EventCounter, StateNegotiating},
	{StateRequested, EventAccept, StateAgreed},
	{StateRequested, EventReject, StateRejected},
	{StateRequested, EventDeadlineExpired, StateRejected},
	{StateNegotiating, EventAccept, StateAgreed},
	{StateNegotiating, EventReject, StateRejected},
	{StateNegotiating, EventWithdraw, StateRejected},
	{StateNegotiating, EventDeadlineExpired, StateRejected},
	{StateAgreed, EventEscrowCreated, StateFunded},
	{StateFunded, EventDeliveryStarted, StateActive},
	{StateActive, EventInvoice, StateInvoiced},
	{StateInvoiced, EventInvoiceApproved, StateActive},
	{StateInvoiced, EventInvoiceRefused, StateTerminated},
	{StateInvoiced, EventTimeout, StateTerminated},
	{StateActive, EventTerminate, StateTerminated},
	{StateActive, EventComplete, StateCompleted},
	// Lifecycle wire-signal transitions added 2026-05-08 per 008 FR-P36/P37/P38.
	{StateActive, EventStop, StateTerminated},
	{StateRequested, EventCancel, StateTerminated},
	{StateNegotiating, EventCancel, StateTerminated},
	{StateAgreed, EventCancel, StateTerminated},
	{StateFunded, EventCancel, StateTerminated},
	{StateActive, EventCancel, StateTerminated},
	{StateInvoiced, EventCancel, StateTerminated},
}

// transitionMap builds a lookup for fast transition resolution.
var transitionMap map[AgreementState]map[AgreementEvent]AgreementState

func init() {
	transitionMap = make(map[AgreementState]map[AgreementEvent]AgreementState)
	for _, t := range transitions {
		if transitionMap[t.from] == nil {
			transitionMap[t.from] = make(map[AgreementEvent]AgreementState)
		}
		transitionMap[t.from][t.event] = t.to
	}
}

// AgreementStateMachine tracks agreement state per requestId.
// FR-P13, FR-P13a: tracked per requestId, not per buyer-seller pair.
type AgreementStateMachine struct {
	requestId            string
	state                AgreementState
	negotiationDeadline  uint64
}

// NewAgreementStateMachine creates a new state machine in IDLE state.
func NewAgreementStateMachine(requestId string) *AgreementStateMachine {
	return &AgreementStateMachine{
		requestId: requestId,
		state:     StateIdle,
	}
}

// State returns the current agreement state.
func (m *AgreementStateMachine) State() AgreementState {
	return m.state
}

// RequestID returns the agreement's requestId.
func (m *AgreementStateMachine) RequestID() string {
	return m.requestId
}

// SetNegotiationDeadline sets the deadline for negotiation. FR-P07a.
func (m *AgreementStateMachine) SetNegotiationDeadline(deadline uint64) {
	m.negotiationDeadline = deadline
}

// Transition attempts a state transition given an event.
// Returns the new state or an error if the transition is invalid.
// FR-P14: State transitions follow defined rules.
func (m *AgreementStateMachine) Transition(event AgreementEvent) (AgreementState, error) {
	const op = "Transition"

	events, ok := transitionMap[m.state]
	if !ok {
		return m.state, NewPaymentError(ErrNegotiationFailed, op,
			"no transitions available from state "+string(m.state))
	}

	next, ok := events[event]
	if !ok {
		return m.state, NewPaymentError(ErrNegotiationFailed, op,
			"invalid transition: "+string(m.state)+" + "+string(event))
	}

	m.state = next
	return m.state, nil
}

// CheckDeadline checks if the negotiation deadline has expired.
// If expired, auto-transitions to REJECTED. FR-P07a.
func (m *AgreementStateMachine) CheckDeadline(now uint64) (bool, error) {
	if m.negotiationDeadline == 0 {
		return false, nil
	}
	if now <= m.negotiationDeadline {
		return false, nil
	}

	// Only applies in REQUESTED or NEGOTIATING states.
	if m.state == StateRequested || m.state == StateNegotiating {
		m.state = StateRejected
		return true, nil
	}
	return false, nil
}

// ComputeAgreementHash computes keccak256(canonicalJSON(acceptedServiceResponse)).
// FR-P17, SC-P07: agreementHash links escrow to off-chain negotiation terms.
func ComputeAgreementHash(canonicalJSON []byte) [32]byte {
	hash := crypto.Keccak256(canonicalJSON)
	var result [32]byte
	copy(result[:], hash)
	return result
}
