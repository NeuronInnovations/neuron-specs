/**
 * Base error class for the Neuron SDK.
 *
 * Spec reference: 006 error-taxonomy.md
 * Format: NEURON-{DOMAIN}-{NUMBER}
 *
 * Every error across the Neuron SDK MUST conform to this structure:
 * - code: string (e.g., "NEURON-KEY-001")
 * - name: string (e.g., "InvalidFormat")
 * - message: string (human-readable description)
 * - cause: Error | undefined (wrapped underlying error)
 */
export class NeuronError extends Error {
  /** Error code in NEURON-{DOMAIN}-{NNN} format. FR-X05 */
  public readonly code: string;

  /** Error name (e.g., "InvalidFormat"). FR-X05 */
  public override readonly name: string;

  /** Wrapped underlying error for debugging. FR-X05 MAY */
  public readonly cause: Error | undefined;

  constructor(code: string, name: string, message: string, cause?: Error) {
    super(message);
    this.code = code;
    this.name = name;
    this.cause = cause;
  }
}

// --- KEY Domain (Spec 002) ---
// Source: 006 error-taxonomy.md, KEY Domain

/** NEURON-KEY-001: Input format not recognized */
export const KEY_INVALID_FORMAT = 'NEURON-KEY-001';
/** NEURON-KEY-002: Ed25519 or non-secp256k1 key detected (FR-A14) */
export const KEY_UNSUPPORTED_KEY_TYPE = 'NEURON-KEY-002';
/** NEURON-KEY-003: Input has wrong byte length */
export const KEY_INVALID_LENGTH = 'NEURON-KEY-003';
/** NEURON-KEY-004: Non-hex characters in hex string input */
export const KEY_INVALID_HEX = 'NEURON-KEY-004';
/** NEURON-KEY-005: Key bytes fail secp256k1 curve validation */
export const KEY_INVALID_KEY = 'NEURON-KEY-005';
/** NEURON-KEY-006: All-zero key material provided */
export const KEY_ZERO_VALUE = 'NEURON-KEY-006';
/** NEURON-KEY-007: Key relationship verification failed */
export const KEY_KEY_MISMATCH = 'NEURON-KEY-007';
/** NEURON-KEY-008: Argon2id or AES-GCM encryption failed */
export const KEY_ENCRYPTION_FAILED = 'NEURON-KEY-008';
/** NEURON-KEY-009: AES-GCM decryption failed */
export const KEY_DECRYPTION_FAILED = 'NEURON-KEY-009';
/** NEURON-KEY-010: Bad mnemonic */
export const KEY_INVALID_MNEMONIC = 'NEURON-KEY-010';
/** NEURON-KEY-011: BIP-44 HD key derivation failed */
export const KEY_DERIVATION_FAILED = 'NEURON-KEY-011';
/** NEURON-KEY-012: Wrapped underlying blockchain SDK error */
export const KEY_SDK_ERROR = 'NEURON-KEY-012';
/** NEURON-KEY-013: ECDSA signing operation failed */
export const KEY_SIGNING_FAILED = 'NEURON-KEY-013';
/** NEURON-KEY-014: Signature verification failed */
export const KEY_VERIFICATION_FAILED = 'NEURON-KEY-014';

// --- ACCT Domain (Spec 001) ---

export const ACCT_INVALID_ACCOUNT_TYPE = 'NEURON-ACCT-001';
export const ACCT_MISSING_REQUIRED_FIELD = 'NEURON-ACCT-002';
export const ACCT_FORBIDDEN_FIELD = 'NEURON-ACCT-003';
export const ACCT_INVALID_DID = 'NEURON-ACCT-004';
export const ACCT_PARENT_KEY_MISMATCH = 'NEURON-ACCT-005';
export const ACCT_INVALID_LEDGER_ATTACHMENT = 'NEURON-ACCT-006';
export const ACCT_ACCOUNT_INCOMPLETE = 'NEURON-ACCT-007';
export const ACCT_INVALID_CURRENCY_SYMBOL = 'NEURON-ACCT-008';

// --- REG Domain (Spec 003) ---

export const REG_REGISTRATION_FAILED = 'NEURON-REG-001';
export const REG_LOOKUP_FAILED = 'NEURON-REG-002';
export const REG_UPDATE_FAILED = 'NEURON-REG-003';
export const REG_REVOCATION_FAILED = 'NEURON-REG-004';
export const REG_INVALID_AGENT_URI = 'NEURON-REG-005';
export const REG_UNAUTHORIZED_CALLER = 'NEURON-REG-006';

// --- TOPIC Domain (Spec 004) ---

export const TOPIC_INVALID_TOPIC_REF = 'NEURON-TOPIC-001';
export const TOPIC_UNSUPPORTED_OPERATION = 'NEURON-TOPIC-002';
export const TOPIC_INVALID_SIGNATURE = 'NEURON-TOPIC-003';
export const TOPIC_SEQUENCE_VIOLATION = 'NEURON-TOPIC-004';
export const TOPIC_PAYLOAD_TOO_LARGE = 'NEURON-TOPIC-005';
export const TOPIC_ADAPTER_NOT_REGISTERED = 'NEURON-TOPIC-006';
export const TOPIC_PUBLISH_FAILED = 'NEURON-TOPIC-007';
export const TOPIC_SUBSCRIBE_FAILED = 'NEURON-TOPIC-008';
export const TOPIC_RESOLVE_FAILED = 'NEURON-TOPIC-009';
export const TOPIC_INVALID_TIMESTAMP = 'NEURON-TOPIC-010';

// --- HEALTH Domain (Spec 005) ---

export const HEALTH_INVALID_PAYLOAD_TYPE = 'NEURON-HEALTH-001';
export const HEALTH_INVALID_VERSION = 'NEURON-HEALTH-002';
export const HEALTH_INVALID_DEADLINE = 'NEURON-HEALTH-003';
export const HEALTH_INVALID_ROLE = 'NEURON-HEALTH-004';
export const HEALTH_PAYLOAD_TOO_LARGE = 'NEURON-HEALTH-005';
export const HEALTH_INVALID_LOCATION = 'NEURON-HEALTH-006';
export const HEALTH_INVALID_CAPABILITIES = 'NEURON-HEALTH-007';

// --- WIRE Domain (Spec 006) ---

export const WIRE_INVALID_FIELD_ORDER = 'NEURON-WIRE-001';
export const WIRE_INVALID_UINT64_ENCODING = 'NEURON-WIRE-002';
export const WIRE_INVALID_BASE64 = 'NEURON-WIRE-003';
export const WIRE_NULL_OPTIONAL_FIELD = 'NEURON-WIRE-004';
export const WIRE_INVALID_ADDRESS_ENCODING = 'NEURON-WIRE-005';
export const WIRE_INVALID_UTF8 = 'NEURON-WIRE-006';
