/**
 * Named constants for the Key Library.
 *
 * Spec reference: 006 algorithm-reference.md §1, §6, §11, §13
 * All magic values are defined here as named constants.
 */

// --- Key lengths (006 algorithm-reference.md §1, §2) ---

/** secp256k1 private key: 32 bytes. FR-001 */
export const PRIVATE_KEY_LENGTH = 32;

/** Compressed public key: 33 bytes (0x02/0x03 prefix + 32-byte X). FR-001, FR-A02 */
export const COMPRESSED_PUBLIC_KEY_LENGTH = 33;

/** Uncompressed public key: 65 bytes (0x04 prefix + 32-byte X + 32-byte Y). FR-001, FR-A02 */
export const UNCOMPRESSED_PUBLIC_KEY_LENGTH = 65;

/** ECDSA signature: 65 bytes (R||S||V). FR-014, FR-A10 */
export const SIGNATURE_LENGTH = 65;

/** EVM address: 20 bytes. FR-005, FR-A03 */
export const EVM_ADDRESS_LENGTH = 20;

// --- secp256k1 curve parameters (006 algorithm-reference.md §1) ---

/** secp256k1 group order n. FR-A01 */
export const SECP256K1_ORDER =
  0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141n;

// --- Multicodec (006 algorithm-reference.md §6) ---

/** secp256k1-pub multicodec varint bytes (value 231 = 0xe7). FR-006a, FR-A06 */
export const MULTICODEC_SECP256K1_PUB = new Uint8Array([0xe7, 0x01]);

/** DID:key prefix. FR-006a, FR-A06 */
export const DID_KEY_PREFIX = 'did:key:z';

// --- BIP-44 (006 algorithm-reference.md §13) ---

/** Default BIP-44 derivation path (Ethereum). FR-013, FR-A13 */
export const BIP44_DEFAULT_PATH = "m/44'/60'/0'/0/0";

// --- Argon2id v1 defaults (006 algorithm-reference.md §11) ---

/** Argon2id time iterations for v1. FR-015, FR-A11 */
export const ARGON2_V1_TIME = 1;

/** Argon2id memory in KiB for v1 (64 MiB). FR-015, FR-A11 */
export const ARGON2_V1_MEMORY = 65536;

/** Argon2id parallelism threads for v1. FR-015, FR-A11 */
export const ARGON2_V1_THREADS = 4;

// --- Encryption field sizes (006 algorithm-reference.md §11) ---

/** Salt length for Argon2id: 16 bytes. FR-015, FR-A11 */
export const SALT_LENGTH = 16;

/** Nonce length for AES-256-GCM: 12 bytes. FR-015, FR-A11 */
export const NONCE_LENGTH = 12;

/** Ciphertext length: 48 bytes (32 key + 16 GCM tag). FR-015, FR-A11 */
export const CIPHERTEXT_LENGTH = 48;

// --- PeerID protobuf header (006 algorithm-reference.md §5) ---

/** Protobuf header for secp256k1 public key: field1=Secp256k1(2), field2=length(33). FR-006, FR-A05 */
export const PEER_ID_PROTOBUF_HEADER = new Uint8Array([0x08, 0x02, 0x12, 0x21]);

/** Identity multihash header: identity(0x00), length(0x25 = 37). FR-006, FR-A05 */
export const IDENTITY_MULTIHASH_HEADER = new Uint8Array([0x00, 0x25]);
