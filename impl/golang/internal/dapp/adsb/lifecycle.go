package adsb

import (
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
)

// Lifecycle is a per-request bookkeeping wrapper around
// payment.AgreementStateMachine. Mirrors internal/dapp/remoteid.Lifecycle
// with the same state-machine semantics (IDLE → REQUESTED → AGREED →
// FUNDED → ACTIVE → INVOICED → ACTIVE → COMPLETED in full mode).
type Lifecycle struct {
	mu       sync.Mutex
	machine  *payment.AgreementStateMachine
	mode     string
	logger   *log.Logger
	logLabel string
}

// LifecycleOptions configures NewLifecycle.
type LifecycleOptions struct {
	RequestID string
	Mode      string
	Logger    *log.Logger
}

// NewLifecycle returns a Lifecycle in IDLE state.
func NewLifecycle(opts LifecycleOptions) (*Lifecycle, error) {
	if opts.RequestID == "" {
		return nil, errors.New("adsb.NewLifecycle: RequestID required")
	}
	mode := opts.Mode
	if mode == "" {
		mode = CommerceModeFull
	}
	switch mode {
	case CommerceModeFull, CommerceModeRegistrationOnly, CommerceModeDataOnly:
	default:
		return nil, fmt.Errorf("adsb.NewLifecycle: unknown mode %q", mode)
	}
	return &Lifecycle{
		machine: payment.NewAgreementStateMachine(opts.RequestID),
		mode:    mode,
		logger:  opts.Logger,
	}, nil
}

func (l *Lifecycle) State() payment.AgreementState {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.machine.State()
}

func (l *Lifecycle) RequestID() string { return l.machine.RequestID() }
func (l *Lifecycle) Mode() string      { return l.mode }

func (l *Lifecycle) Receive(_ payment.ServiceRequest) error {
	return l.fire(payment.EventServiceRequest, "serviceRequest received")
}

func (l *Lifecycle) Accept() error {
	return l.fire(payment.EventAccept, "serviceResponse(accept) published")
}

func (l *Lifecycle) Reject() error {
	return l.fire(payment.EventReject, "serviceResponse(reject) published")
}

func (l *Lifecycle) Funded() error {
	if l.mode != CommerceModeFull {
		return fmt.Errorf("adsb.Lifecycle.Funded: mode=%s skips funded transition", l.mode)
	}
	return l.fire(payment.EventEscrowCreated, "escrowCreated received")
}

func (l *Lifecycle) StartDelivery() error {
	if l.mode != CommerceModeFull {
		return fmt.Errorf("adsb.Lifecycle.StartDelivery: mode=%s skips active-state transition", l.mode)
	}
	return l.fire(payment.EventDeliveryStarted, "connectionSetup published; libp2p stream active")
}

func (l *Lifecycle) BeginInvoice() error {
	if l.mode != CommerceModeFull {
		return fmt.Errorf("adsb.Lifecycle.BeginInvoice: mode=%s skips invoice transition", l.mode)
	}
	return l.fire(payment.EventInvoice, "serviceStop received; invoice published")
}

func (l *Lifecycle) ApproveInvoice() error {
	if l.mode != CommerceModeFull {
		return fmt.Errorf("adsb.Lifecycle.ApproveInvoice: mode=%s skips invoice-approved transition", l.mode)
	}
	return l.fire(payment.EventInvoiceApproved, "invoiceAck(approved) received")
}

func (l *Lifecycle) RefuseInvoice() error {
	if l.mode != CommerceModeFull {
		return fmt.Errorf("adsb.Lifecycle.RefuseInvoice: mode=%s skips invoice-refused transition", l.mode)
	}
	return l.fire(payment.EventInvoiceRefused, "invoiceAck(refused) received")
}

func (l *Lifecycle) Complete() error {
	if l.mode != CommerceModeFull {
		return fmt.Errorf("adsb.Lifecycle.Complete: mode=%s never reaches COMPLETED", l.mode)
	}
	return l.fire(payment.EventComplete, "lifecycle complete")
}

func (l *Lifecycle) Stop() error {
	return l.fire(payment.EventStop, "serviceStop received (abort, no settlement)")
}

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
