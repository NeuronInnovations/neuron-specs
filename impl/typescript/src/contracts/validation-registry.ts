/**
 * IValidationRegistry -- Validation Registry interface for third-party verification.
 *
 * Spec reference: 007 spec.md
 *   - FR-C-27: validationRequest() creates a request addressed to a validator.
 *              Only the agent owner (owner of agentId NFT) may call.
 *   - FR-C-28: validationResponse() callable only by the addressed validator.
 *   - FR-C-29: Response codes: 0=pending, 1=pass, 2=fail.
 *   - FR-C-30: getValidationStatus() returns the complete validation record.
 *   - FR-C-31: getSummary() returns aggregated counts filtered by validators and tag.
 *   - FR-C-32: Events: ValidationRequested, ValidationResponded.
 *   - FR-C-33: agentId must exist in the linked Identity Registry.
 *
 * Cross-registry identity: the agentId (tokenId from Identity Registry) is the
 * key that links validation records to a specific registered agent (DD-04).
 */

import type { Uint256, Address, Bytes32, ValidationRecord, ValidationSummary } from './types.js';
import type { ValidationResponse } from './types.js';

// ---------------------------------------------------------------------------
// Interface
// ---------------------------------------------------------------------------

/**
 * Validation Registry -- third-party verification of agent capabilities.
 *
 * Agents request validation from specific validators. Validators respond
 * with pass/fail/pending status. Summary queries aggregate results by
 * validator and tag filters.
 */
export interface IValidationRegistry {
  /**
   * Create a validation request addressed to a specific validator.
   *
   * FR-C-27: Only the owner of the agentId NFT in the Identity Registry
   *          may call this function. Reverts with NotAgentOwner if caller
   *          is not the owner.
   * FR-C-32: Emits ValidationRequested(requestHash, agentId, validatorAddress).
   * FR-C-33: Reverts if agentId does not exist in Identity Registry.
   *
   * @param agentId - The agent's tokenId from the Identity Registry.
   * @param validator - The address of the validator to receive the request.
   * @param requestURI - Off-chain URI describing the validation request.
   * @param tag - Categorical tag for filtering (bytes32). bytes32(0) for no tag.
   * @returns The requestHash (unique identifier for this validation request).
   */
  validationRequest(
    agentId: Uint256,
    validator: Address,
    requestURI: string,
    tag: Bytes32,
  ): Promise<Bytes32>;

  /**
   * Respond to a validation request.
   *
   * FR-C-28: Callable only by the validator address specified in the original
   *          request. Reverts with NotAddressedValidator if caller does not match.
   * FR-C-29: response must be 0 (pending), 1 (pass), or 2 (fail).
   * FR-C-32: Emits ValidationResponded(requestHash, response, tag).
   *
   * @param requestHash - The unique identifier of the validation request.
   * @param response - The validation outcome (ValidationResponse enum).
   * @param responseURI - Off-chain URI with the validator's response details.
   * @param responseHash - Keccak256 hash of the responseURI content.
   * @param tag - Categorical tag (bytes32). May differ from the request tag.
   */
  validationResponse(
    requestHash: Bytes32,
    response: ValidationResponse,
    responseURI: string,
    responseHash: Bytes32,
    tag: Bytes32,
  ): Promise<void>;

  /**
   * Read the complete validation record for a request.
   *
   * FR-C-30: Returns validator, agentId, response, responseHash, tag, lastUpdate.
   *
   * @param requestHash - The unique identifier of the validation request.
   * @returns The complete ValidationRecord.
   */
  getValidationStatus(requestHash: Bytes32): Promise<ValidationRecord>;

  /**
   * Query aggregated validation summary for an agent.
   *
   * FR-C-31: Returns count, passCount, failCount filtered by validators and tag.
   *          Empty validatorAddresses includes all validators.
   *          bytes32(0) tag is a wildcard.
   *
   * @param agentId - The agent's tokenId.
   * @param validatorAddresses - Filter by specific validators. Empty = all.
   * @param tag - Filter by tag. bytes32(0) = wildcard.
   * @returns Aggregated validation summary.
   */
  getSummary(
    agentId: Uint256,
    validatorAddresses: ReadonlyArray<Address>,
    tag: Bytes32,
  ): Promise<ValidationSummary>;
}

// ---------------------------------------------------------------------------
// Events
// ---------------------------------------------------------------------------

/**
 * Emitted when a validation request is created.
 *
 * FR-C-32: ValidationRequested(bytes32 indexed requestHash, uint256 indexed agentId,
 *          address indexed validatorAddress).
 */
export interface ValidationRequestedEvent {
  /** Unique identifier for the validation request. FR-C-27 */
  readonly requestHash: Bytes32;

  /** The agent's tokenId from Identity Registry. FR-C-27 */
  readonly agentId: Uint256;

  /** The address of the validator. FR-C-27 */
  readonly validatorAddress: Address;
}

/**
 * Emitted when a validator responds to a validation request.
 *
 * FR-C-32: ValidationResponded(bytes32 indexed requestHash, uint8 response, bytes32 tag).
 */
export interface ValidationRespondedEvent {
  /** Unique identifier of the validation request. FR-C-28 */
  readonly requestHash: Bytes32;

  /** The validation outcome (0=pending, 1=pass, 2=fail). FR-C-29 */
  readonly response: ValidationResponse;

  /** Categorical tag associated with the response. FR-C-28 */
  readonly tag: Bytes32;
}
