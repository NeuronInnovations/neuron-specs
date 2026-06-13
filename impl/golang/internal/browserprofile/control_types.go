package browserprofile

// Tier B control-stream payload types.
//
// Each struct maps 1:1 to the JSON payloads that flow on the
// /neuron/browser-profile/control/1.0.0 stream:
//
//	serviceRequest   (buyer -> seller)
//	paymentDetails   (seller -> buyer)
//	connectionSetup  (seller -> buyer, encryptedMultiaddrs via delivery/ECIES)
//	invoiceAck       (buyer -> seller)
//
// Go's encoding/json emits struct fields in declaration order, which matches
// the TS `JSON.stringify(payloadObj)` output for the object-literal shapes in
// impl/typescript/src/server-demo/seller-flow.ts and
// impl/typescript/src/browser-client/buyer-flow.ts.
//
// Note: priceAtto is a JSON string on the wire (TS: `priceAtto: '1'`), so the
// Go field is `string`, not an integer.

const (
	// ControlProtocolID mirrors impl/typescript/src/browser-client/constants.ts:17.
	ControlProtocolID = "/neuron/browser-profile/control/1.0.0"
	// DataProtocolID mirrors impl/typescript/src/browser-client/constants.ts:20.
	DataProtocolID = "/neuron/browser-profile/data/1.0.0"
	// ServiceName matches the Tier 1 demo (`service: 'jpeg-demo'`).
	ServiceName = "jpeg-demo"
)

// Payload type discriminators.
const (
	TypeServiceRequest   = "serviceRequest"
	TypePaymentDetails   = "paymentDetails"
	TypeConnectionSetup  = "connectionSetup"
	TypeInvoiceAck       = "invoiceAck"
)

// ServiceRequestPayload is the buyer -> seller opening message.
// Field order mirrors the TS literal at buyer-flow.ts:144-149.
type ServiceRequestPayload struct {
	Type           string `json:"type"`
	Service        string `json:"service"`
	BuyerAddress   string `json:"buyerAddress"`
	BuyerPubKeyHex string `json:"buyerPubKeyHex"`
}

// PaymentDetailsPayload is the seller -> buyer payment-pledge message.
// Field order mirrors the TS literal at seller-flow.ts:99-104.
// PriceAtto is a JSON string (TS emits `'1'`, not 1).
type PaymentDetailsPayload struct {
	Type             string `json:"type"`
	AgreementHash    string `json:"agreementHash"`
	PriceAtto        string `json:"priceAtto"`
	InvoiceSha256Hex string `json:"invoiceSha256Hex"`
}

// ConnectionSetupPayload carries the seller's ECIES-encrypted multiaddrs.
// Field order mirrors the TS literal at seller-flow.ts:111-116.
type ConnectionSetupPayload struct {
	Type                string `json:"type"`
	RecipientEVMAddress string `json:"recipientEVMAddress"`
	EncryptedMultiaddrs string `json:"encryptedMultiaddrs"`
	StreamProtocol      string `json:"streamProtocol"`
}

// InvoiceAckPayload is the buyer's terminal message after SHA-256 verification.
// Field order mirrors the TS literal at buyer-flow.ts:184-188.
type InvoiceAckPayload struct {
	Type              string `json:"type"`
	ReceivedSha256Hex string `json:"receivedSha256Hex"`
}

// FileMetadata is the frame-0 JSON object sent on the data stream.
// Field order mirrors the TS FileMetadata interface (file-metadata.ts:17-22).
// SizeBytes is a JSON number (not string): Go emits int -> 50282, matching TS.
type FileMetadata struct {
	Filename    string `json:"filename"`
	SizeBytes   int    `json:"sizeBytes"`
	ContentType string `json:"contentType"`
	Sha256Hex   string `json:"sha256Hex"`
}

