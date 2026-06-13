package remoteid

import (
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
)

// Lifecycle is a per-request bookkeeping wrapper around
// payment.AgreementStateMachine. It exposes one named method per Stage-2
// transition + a logger that emits one `[lifecycle] state=… requestID=…`
// line per advance, so the seller's negotiation goroutine + tests share
// one canonical trace shape.
//
// The full-commerce path drives the canonical 008 lifecycle:
//
//	IDLE → REQUESTED → AGREED → FUNDED → ACTIVE → INVOICED → ACTIVE → COMPLETED
//
// The registration-only short-circuit (D3 from the Stage-2 plan) skips
// FUNDED → ACTIVE → INVOICED → COMPLETED; the seller never touches escrow
// or settlement and the state machine stops at AGREED. See Mode().
type Lifecycle struct {
	mu       sync.Mutex
	machine  *payment.AgreementStateMachine
	mode     string
	logger   *log.Logger
	logLabel string // optional "[lifecycle] " prefix swap for tests
}

// LifecycleOptions configures NewLifecycle.
type LifecycleOptions struct {
	// RequestID identifies this per-request lifecycle. Required.
	RequestID string

	// Mode picks the transition policy:
	//   - CommerceModeFull (default): full 008 lifecycle.
	//   - CommerceModeRegistrationOnly: stops at AGREED.
	//   - CommerceModeDataOnly: never engages the state machine — this
	//     wrapper still tracks state names for evidence but Advance()
	//     calls return errors.
	Mode string

	// Logger receives one `[lifecycle] requestID=… <from>→<to> (<reason>)`
	// line per transition. Optional.
	Logger *log.Logger
}

// NewLifecycle returns a Lifecycle in IDLE state.
func NewLifecycle(opts LifecycleOptions) (*Lifecycle, error) {
	if opts.RequestID == "" {
		return nil, errors.New("remoteid.NewLifecycle: RequestID required")
	}
	mode := opts.Mode
	if mode == "" {
		mode = CommerceModeFull
	}
	switch mode {
	case CommerceModeFull, CommerceModeRegistrationOnly, CommerceModeDataOnly:
		// ok
	default:
		return nil, fmt.Errorf("remoteid.NewLifecycle: unknown mode %q", mode)
	}
	return &Lifecycle{
		machine: payment.NewAgreementStateMachine(opts.RequestID),
		mode:    mode,
		logger:  opts.Logger,
	}, nil
}

// State returns the current agreement state.
func (l *Lifecycle) State() payment.AgreementState {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.machine.State()
}

// RequestID returns the bound request id.
func (l *Lifecycle) RequestID() string { return l.machine.RequestID() }

// Mode returns the lifecycle's commerce-mode posture.
func (l *Lifecycle) Mode() string { return l.mode }

// Receive transitions IDLE → REQUESTED on observing the buyer's
// ServiceRequest (008 EventServiceRequest). Used by the seller side.
func (l *Lifecycle) Receive(_ payment.ServiceRequest) error {
	return l.fire(payment.EventServiceRequest, "serviceRequest received")
}

// Accept transitions REQUESTED → AGREED. Used by the seller after
// publishing ServiceResponse{Action:"accept"}.
func (l *Lifecycle) Accept() error {
	return l.fire(payment.EventAccept, "serviceResponse(accept) published")
}

// Reject transitions REQUESTED → REJECTED. Used by the seller when
// it cannot service the request.
func (l *Lifecycle) Reject() error {
	return l.fire(payment.EventReject, "serviceResponse(reject) published")
}

// Funded transitions AGREED → FUNDED on observing EscrowCreated. Only
// valid in CommerceModeFull.
func (l *Lifecycle) Funded() error {
	if l.mode != CommerceModeFull {
		return fmt.Errorf("remoteid.Lifecycle.Funded: mode=%s skips funded transition (R1 registration-only)", l.mode)
	}
	return l.fire(payment.EventEscrowCreated, "escrowCreated received")
}

// StartDelivery transitions FUNDED → ACTIVE. Used after the seller
// publishes ConnectionSetup + the buyer has dialed.
func (l *Lifecycle) StartDelivery() error {
	if l.mode != CommerceModeFull {
		return fmt.Errorf("remoteid.Lifecycle.StartDelivery: mode=%s skips active-state transition", l.mode)
	}
	return l.fire(payment.EventDeliveryStarted, "connectionSetup published; libp2p stream active")
}

// BeginInvoice transitions ACTIVE → INVOICED. Used when the seller
// publishes the Invoice envelope after observing the buyer's ServiceStop.
func (l *Lifecycle) BeginInvoice() error {
	if l.mode != CommerceModeFull {
		return fmt.Errorf("remoteid.Lifecycle.BeginInvoice: mode=%s skips invoice transition", l.mode)
	}
	return l.fire(payment.EventInvoice, "serviceStop received; invoice published")
}

// ApproveInvoice transitions INVOICED → ACTIVE. Used when the seller
// observes the buyer's InvoiceAck{Action:"approved"}.
func (l *Lifecycle) ApproveInvoice() error {
	if l.mode != CommerceModeFull {
		return fmt.Errorf("remoteid.Lifecycle.ApproveInvoice: mode=%s skips invoice-approved transition", l.mode)
	}
	return l.fire(payment.EventInvoiceApproved, "invoiceAck(approved) received")
}

// RefuseInvoice transitions INVOICED → TERMINATED. Used when the seller
// observes InvoiceAck{Action:"refused"}.
func (l *Lifecycle) RefuseInvoice() error {
	if l.mode != CommerceModeFull {
		return fmt.Errorf("remoteid.Lifecycle.RefuseInvoice: mode=%s skips invoice-refused transition", l.mode)
	}
	return l.fire(payment.EventInvoiceRefused, "invoiceAck(refused) received")
}

// Complete transitions ACTIVE → COMPLETED. Final happy-path transition
// for the seller.
func (l *Lifecycle) Complete() error {
	if l.mode != CommerceModeFull {
		return fmt.Errorf("remoteid.Lifecycle.Complete: mode=%s never reaches COMPLETED", l.mode)
	}
	return l.fire(payment.EventComplete, "lifecycle complete")
}

// Stop transitions ACTIVE → TERMINATED on observing ServiceStop. Used
// for abort paths where the buyer wants to end without settlement.
func (l *Lifecycle) Stop() error {
	return l.fire(payment.EventStop, "serviceStop received (abort, no settlement)")
}

// fire is the internal transition + log helper.
func (l *Lifecycle) fire(event payment.AgreementEvent, reason string) error {
	l.mu.Lock()
	from := l.machine.State()
	to, err := l.machine.Transition(event)
	l.mu.Unlock()

	if err != nil {
		if l.logger != nil {
			l.logger.Printf("[lifecycle] requestID=%s state=%s event=%s REJECTED: %v",
				l.machine.RequestID(), from, event, err)
		}
		return err
	}
	if l.logger != nil {
		l.logger.Printf("[lifecycle] requestID=%s %s→%s (%s)",
			l.machine.RequestID(), from, to, reason)
	}
	return nil
}
