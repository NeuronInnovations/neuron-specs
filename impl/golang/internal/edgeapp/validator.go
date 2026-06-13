package edgeapp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
	"github.com/neuron-sdk/neuron-go-sdk/internal/validation"
)

// ValidatorEvidencePayloadType is the topic-message payload type the
// validator publishes when emitting EvidenceEnvelopes on its stdOut topic.
const ValidatorEvidencePayloadType = "neuron-evidence/1"

// ValidatorAgreementSnapshot captures the canonical-JSON bytes the validator
// observed for one agreement's protocol messages. Validators hash these
// over the (sorted-by-arrival) sequence to produce a deterministic
// evidenceHash that downstream consumers can re-derive from the same topic
// transcript.
type ValidatorAgreementSnapshot struct {
	RequestID         string
	ServiceRequest    []byte
	ServiceResponse   []byte
	EscrowCreated     []byte
	Invoice           []byte
	InvoiceAck        []byte
	InvoiceAckAction  string // "approved" / "refused" / "" (none seen)
	StartedAt         time.Time
	SettledAt         time.Time
}

// IsComplete reports whether the snapshot captures all five canonical
// messages of a complete agreement.
func (s *ValidatorAgreementSnapshot) IsComplete() bool {
	return len(s.ServiceRequest) > 0 &&
		len(s.ServiceResponse) > 0 &&
		len(s.EscrowCreated) > 0 &&
		len(s.Invoice) > 0 &&
		len(s.InvoiceAck) > 0
}

// ValidatorConfig configures the in-process validator.
type ValidatorConfig struct {
	// Bus is the topic adapter the validator subscribes through. Required.
	Bus topic.TopicAdapter

	// Key is the validator's signing key. Used to publish EvidenceEnvelopes
	// on its own stdOut. Required.
	Key *keylib.NeuronPrivateKey

	// ValidatorStdOut is where envelopes are published. Required.
	ValidatorStdOut topic.TopicRef

	// ValidatorAgentID is the on-chain tokenID for this validator (EIP-8004).
	// Decimal-string UnsignedInt256 per spec 010. In mock mode "0" is fine.
	ValidatorAgentID string

	// SubjectAgentID is the on-chain tokenID for the seller being attested.
	// Decimal-string. Mock mode "0" is OK.
	SubjectAgentID string

	// SellerStdIn / BuyerStdIn / SellerStdOut: the topics the validator
	// subscribes to. Validator observes ServiceRequest+EscrowCreated on
	// SellerStdIn; ServiceResponse+Invoice on BuyerStdIn; InvoiceAck on
	// SellerStdIn. SellerStdOut isn't strictly needed for commerce
	// envelopes but is reserved for future delivery-evidence (spec 009).
	SellerStdIn  topic.TopicRef
	BuyerStdIn   topic.TopicRef
	SellerStdOut topic.TopicRef

	// EvidenceURIPrefix prepends the topic-message reference when the
	// envelope is built. Defaults to "memory://" — fine for mock mode.
	EvidenceURIPrefix string

	// Replay, when true, asks the bus to deliver every message on each
	// subscribed topic from sequence 0 (inclusive of all history). This
	// mirrors the natural shape for a third-party witness that joins the
	// conversation after agreement messages are already on-topic — which
	// is the standalone validator's primary use case.
	//
	// Defaults to false to preserve the SubscribeOpts contract ("Nil means
	// start from the latest message"). cmd/edge-validator sets this to
	// true; in-process commerce tests typically leave it false because the
	// validator subscribes BEFORE any publish.
	//
	// Concretely: Replay=true ⇒ Subscribe is called with
	// SubscribeOpts.FromSequence=&zero. On HCS this triggers mirror-node
	// history replay from topic creation; on MemoryBus it replays
	// in-memory log entries (which already replay-from-zero by default,
	// so the field is a no-op against MemoryBus but explicit for clarity).
	Replay bool

	Logger Logger
}

// Validator subscribes to the four relevant topics, captures protocol
// messages keyed by requestId, and builds two EvidenceEnvelopes per
// completed agreement: one against `008-payment` (settlement compliance)
// and one against `009-delivery` (placeholder — delivery telemetry is
// out of scope for iteration 2; envelope still constructed for spec 010
// pipeline shape).
//
// **Iter-7 P1.1 change:** the dedup key for emitted envelopes is the
// tuple `(requestID, agreementHash[:8])` rather than `requestID` alone.
// `agreementHash` is the sha256 of the canonical ServiceResponse bytes;
// two agreements with the same requestID but different terms (e.g. an
// older buyer that reused a static requestID across sessions) emit
// separate envelope-pairs. Idempotent against true HCS replays because
// identical bytes produce identical hashes.
//
// Validator is safe for concurrent use; Run blocks until ctx is cancelled.
type Validator struct {
	cfg          ValidatorConfig
	mu           sync.Mutex
	agreements   map[string]*ValidatorAgreementSnapshot
	emitted      map[string]bool // emitKey(requestID, agreementHash) → already-emitted set
}

// NewValidator constructs a Validator from cfg.
func NewValidator(cfg ValidatorConfig) (*Validator, error) {
	if err := validateValidatorCfg(&cfg); err != nil {
		return nil, err
	}
	return &Validator{
		cfg:        cfg,
		agreements: make(map[string]*ValidatorAgreementSnapshot),
		emitted:    make(map[string]bool),
	}, nil
}

// Run subscribes to all configured topics and blocks until ctx is cancelled.
// It captures protocol payloads by requestId and emits EvidenceEnvelopes
// the moment a snapshot becomes complete. Each completed agreement emits
// at most one envelope per spec.
func (v *Validator) Run(ctx context.Context) error {
	logger := v.cfg.Logger
	if logger == nil {
		logger = nopLogger{}
	}

	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	subOpts := func(label string) topic.SubscribeOpts {
		opts := topic.SubscribeOpts{
			ErrorHandler: func(err error) {
				logger.Printf("[validator] subscribe-side drop on %s: %v", label, err)
			},
		}
		if v.cfg.Replay {
			zero := uint64(0)
			opts.FromSequence = &zero
		}
		return opts
	}

	sellerIn, err := v.cfg.Bus.Subscribe(subCtx, v.cfg.SellerStdIn, subOpts("seller.stdIn"))
	if err != nil {
		return fmt.Errorf("validator: subscribe seller.stdIn: %w", err)
	}
	buyerIn, err := v.cfg.Bus.Subscribe(subCtx, v.cfg.BuyerStdIn, subOpts("buyer.stdIn"))
	if err != nil {
		return fmt.Errorf("validator: subscribe buyer.stdIn: %w", err)
	}
	logger.Printf("[validator] subscribed seller.stdIn=%s buyer.stdIn=%s replay=%v",
		v.cfg.SellerStdIn.Locator(), v.cfg.BuyerStdIn.Locator(), v.cfg.Replay)

	for {
		select {
		case <-ctx.Done():
			return nil
		case d, ok := <-sellerIn:
			if !ok {
				logger.Printf("[validator] seller.stdIn channel closed")
				return nil
			}
			v.handleDelivery(ctx, d, "seller.stdIn", logger)
		case d, ok := <-buyerIn:
			if !ok {
				logger.Printf("[validator] buyer.stdIn channel closed")
				return nil
			}
			v.handleDelivery(ctx, d, "buyer.stdIn", logger)
		}
	}
}

// handleDelivery routes one TopicMessage delivery into the per-agreement
// snapshot keyed by requestId, and emits envelopes on completion.
//
// Every silent-exit path emits an INFO log: signature-validate failure,
// JSON-unmarshal failure, missing requestId, and unknown protocol type.
// This was added in iter-5 after the iter-4 testnet replay produced zero
// envelopes silently — the diagnostic surface is now sufficient to
// distinguish "subscription delivered nothing" from "delivered but
// dropped at validation/parse" without code changes.
func (v *Validator) handleDelivery(ctx context.Context, d topic.MessageDelivery, source string, logger Logger) {
	if err := topic.ValidateTopicMessage(d.Message); err != nil {
		logger.Printf("[validator] drop on %s seq=%d: invalid topic message: %v",
			source, d.BackendSequence, err)
		return
	}
	payloadBytes := d.Message.Payload()
	var probe struct {
		Type      string `json:"type"`
		RequestID string `json:"requestId"`
		Action    string `json:"action"`
	}
	if err := json.Unmarshal(payloadBytes, &probe); err != nil {
		logger.Printf("[validator] drop on %s seq=%d: payload not JSON (%d bytes): %v",
			source, d.BackendSequence, len(payloadBytes), err)
		return
	}
	if probe.RequestID == "" {
		logger.Printf("[validator] drop on %s seq=%d: no requestId (type=%q)",
			source, d.BackendSequence, probe.Type)
		return
	}

	known := false
	switch probe.Type {
	case payment.PayloadServiceRequest, payment.PayloadServiceResponse,
		payment.PayloadEscrowCreated, payment.PayloadInvoice,
		payment.PayloadInvoiceAck:
		known = true
	}
	if !known {
		logger.Printf("[validator] skip on %s seq=%d: unknown payload type=%q requestId=%s",
			source, d.BackendSequence, probe.Type, probe.RequestID)
		return
	}

	v.mu.Lock()
	snap := v.agreements[probe.RequestID]
	if snap == nil {
		snap = &ValidatorAgreementSnapshot{
			RequestID: probe.RequestID,
			StartedAt: time.Now(),
		}
		v.agreements[probe.RequestID] = snap
	}
	switch probe.Type {
	case payment.PayloadServiceRequest:
		snap.ServiceRequest = payloadBytes
	case payment.PayloadServiceResponse:
		snap.ServiceResponse = payloadBytes
	case payment.PayloadEscrowCreated:
		snap.EscrowCreated = payloadBytes
	case payment.PayloadInvoice:
		snap.Invoice = payloadBytes
	case payment.PayloadInvoiceAck:
		snap.InvoiceAck = payloadBytes
		snap.InvoiceAckAction = probe.Action
		snap.SettledAt = time.Now()
	}

	complete := snap.IsComplete()
	// Iter-7 P1.1: dedup key includes the first 8 bytes of agreementHash =
	// sha256(snap.ServiceResponse). Same requestID + identical
	// ServiceResponse bytes (true HCS replay) ⇒ same key, deduped. Same
	// requestID + new ServiceResponse bytes (legacy buyer reusing rid for
	// a fresh agreement) ⇒ new key, emits independently.
	key := emitKey(snap)
	emitted := v.emitted[key]
	logger.Printf("[validator] match on %s seq=%d type=%s requestId=%s complete=%v emitted=%v",
		source, d.BackendSequence, probe.Type, probe.RequestID, complete, emitted)
	v.mu.Unlock()

	if complete && !emitted {
		logger.Printf("[validator] emit start requestId=%s ackAction=%s",
			probe.RequestID, snap.InvoiceAckAction)
		v.emitEnvelopes(ctx, snap, logger)
		v.mu.Lock()
		v.emitted[key] = true
		v.mu.Unlock()
	}
}

// emitKey returns the dedup key for the snapshot — `requestID:hash[:8]`
// where hash is sha256 of the canonical ServiceResponse bytes (or a zero
// hash when the snapshot doesn't yet have ServiceResponse, but a complete
// snapshot always does, so this is only relevant for partial snapshots
// which never call emit).
func emitKey(snap *ValidatorAgreementSnapshot) string {
	h := sha256.Sum256(snap.ServiceResponse)
	return snap.RequestID + ":" + hex.EncodeToString(h[:8])
}

// emitEnvelopes builds + publishes the payment + delivery envelopes for a
// completed snapshot. Errors are logged but don't stop the validator.
func (v *Validator) emitEnvelopes(ctx context.Context, snap *ValidatorAgreementSnapshot, logger Logger) {
	uriPrefix := v.cfg.EvidenceURIPrefix
	if uriPrefix == "" {
		uriPrefix = "memory://"
	}

	// Payment evidence (spec 008).
	paymentVerdict := validation.VerdictCompliant
	if snap.InvoiceAckAction != "approved" {
		paymentVerdict = validation.VerdictNonCompliant
	}
	paymentHash := snapshotHash(snap.ServiceRequest, snap.ServiceResponse,
		snap.EscrowCreated, snap.Invoice, snap.InvoiceAck)
	// Ensure window end > start: a sub-second mock agreement may collapse to
	// the same whole-second timestamp.
	startSec := uint64(snap.StartedAt.Unix())
	endSec := uint64(snap.SettledAt.Unix())
	if endSec <= startSec {
		endSec = startSec + 1
	}
	paymentEnv, err := validation.NewEvidenceEnvelope(
		v.cfg.ValidatorAgentID,
		v.cfg.SubjectAgentID,
		"008-payment",
		paymentVerdict,
		paymentHash,
		uriPrefix+snap.RequestID+"/payment",
		validation.WithObservationWindow(startSec, endSec),
	)
	if err != nil {
		logger.Printf("[validator] build payment envelope: %v", err)
		return
	}

	// Delivery evidence (spec 009 placeholder — iteration 2 lacks the
	// stream-content hash hook; we attest based on the same agreement
	// transcript for now). Iteration 3+ will replace this with a true
	// stream-bytes SHA-256.
	deliveryHash := snapshotHash(snap.ServiceResponse, snap.Invoice, snap.InvoiceAck)
	deliveryEnv, err := validation.NewEvidenceEnvelope(
		v.cfg.ValidatorAgentID,
		v.cfg.SubjectAgentID,
		"009-p2p-data-delivery",
		validation.VerdictInconclusive, // honest: validator can't see the data plane in iter-2
		deliveryHash,
		uriPrefix+snap.RequestID+"/delivery",
	)
	if err != nil {
		logger.Printf("[validator] build delivery envelope: %v", err)
		return
	}

	if err := v.publishEnvelope(ctx, paymentEnv); err != nil {
		logger.Printf("[validator] publish payment envelope: %v", err)
	}
	if err := v.publishEnvelope(ctx, deliveryEnv); err != nil {
		logger.Printf("[validator] publish delivery envelope: %v", err)
	}
	logger.Printf("[validator] emitted envelopes for requestID=%s payment=%s delivery=%s",
		snap.RequestID, paymentVerdict, validation.VerdictInconclusive)
}

// publishEnvelope serializes an EvidenceEnvelope to JSON and publishes it on
// the validator's stdOut topic.
func (v *Validator) publishEnvelope(ctx context.Context, env *validation.EvidenceEnvelope) error {
	_ = ctx
	data, err := json.Marshal(envelopeToJSON(env))
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}
	now := uint64(time.Now().UnixNano())
	msg, err := topic.NewTopicMessage(v.cfg.Key, now, now, data)
	if err != nil {
		return fmt.Errorf("sign envelope: %w", err)
	}
	if _, err := v.cfg.Bus.Publish(v.cfg.ValidatorStdOut, msg, topic.PublishOpts{ConfirmationMode: topic.FireAndForget}); err != nil {
		return fmt.Errorf("publish envelope: %w", err)
	}
	return nil
}

// envelopeToJSON converts an EvidenceEnvelope to a JSON-serializable map
// using its public accessors. The validation package's envelope type is
// intentionally opaque (private fields); we project to a plain map here
// so the topic-message payload is recognizable without forcing every
// observer to import internal/validation.
func envelopeToJSON(e *validation.EvidenceEnvelope) map[string]any {
	out := map[string]any{
		"type":             ValidatorEvidencePayloadType,
		"envelopeType":     e.Type(),
		"version":          e.Version(),
		"validatorAgentId": e.ValidatorAgentId(),
		"subjectAgentId":   e.SubjectAgentId(),
		"specRef":          e.SpecRef(),
		"verdict":          string(e.Verdict()),
		"evidenceHash":     e.EvidenceHash(),
		"evidenceURI":      e.EvidenceURI(),
	}
	return out
}

// snapshotHash returns 0x-prefixed lowercase-hex SHA-256 of the concatenated
// payload bytes. Stable across observers because the inputs are already
// canonical-JSON forms.
func snapshotHash(parts ...[]byte) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write(p)
	}
	return "0x" + hex.EncodeToString(h.Sum(nil))
}

func validateValidatorCfg(c *ValidatorConfig) error {
	switch {
	case c == nil:
		return errors.New("validator-cfg: nil")
	case c.Bus == nil:
		return errors.New("validator-cfg: Bus required")
	case c.Key == nil:
		return errors.New("validator-cfg: Key required")
	case c.ValidatorStdOut.Locator() == "":
		return errors.New("validator-cfg: ValidatorStdOut required")
	case c.ValidatorAgentID == "":
		return errors.New("validator-cfg: ValidatorAgentID required")
	case c.SubjectAgentID == "":
		return errors.New("validator-cfg: SubjectAgentID required")
	case c.SellerStdIn.Locator() == "":
		return errors.New("validator-cfg: SellerStdIn required")
	case c.BuyerStdIn.Locator() == "":
		return errors.New("validator-cfg: BuyerStdIn required")
	}
	return nil
}
