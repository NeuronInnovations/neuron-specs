/**
 * Topic System type definitions -- enums and type aliases.
 *
 * Spec reference: 004 spec.md
 *   - FR-T01: BackendKind transport identifiers.
 *   - FR-T05: Extensible custom transport via runtime adapter registration.
 *   - FR-T07: Reserved channel roles (stdIn, stdOut, stdErr).
 *   - FR-T08: Custom channel namespace prefix (custom:<name>).
 *   - FR-T23: ConfirmationMode for publish behavior.
 *
 * Spec reference: 004 data-model.md
 *   - BackendKind, ChannelRole, ConfirmationMode enum definitions.
 */

/**
 * Transport backend kind identifier.
 *
 * FR-T01: Uniquely identifies the transport backend.
 * FR-T05: Extensible via `custom:<type>` for runtime-registered adapters.
 *
 * Built-in kinds:
 * - `'hcs'` -- Hedera Consensus Service (Constitution VIII)
 * - `'erc-log'` -- ERC event logs on Ethereum/EVM chains (read-only)
 * - `'kafka'` -- Kafka with ledger anchoring
 * - `'custom:${string}'` -- Custom transport (runtime-registered)
 */
export type BackendKind = 'hcs' | 'erc-log' | 'kafka' | `custom:${string}`;

/**
 * Named role of a topic channel.
 *
 * FR-T07: Three reserved channel roles for standard communication patterns.
 * FR-T08: Custom channels use the `custom:<name>` namespace prefix.
 *
 * Reserved roles:
 * - `'stdIn'`  -- Inbound public channel (other peers publish TO this peer)
 * - `'stdOut'` -- Outbound public channel (this peer publishes its own output)
 * - `'stdErr'` -- Error/diagnostic public channel (this peer publishes errors)
 * - `'custom:${string}'` -- Custom named channel (namespace prefix required)
 */
export type ChannelRole = 'stdIn' | 'stdOut' | 'stdErr' | `custom:${string}`;

/**
 * Controls Publish() behavior regarding confirmation.
 *
 * FR-T23: Selectable per-call via PublishOpts.
 * - `'FIRE_AND_FORGET'`    -- Return after network acknowledgment (confirmed=false)
 * - `'WAIT_FOR_CONSENSUS'` -- Return after backend finalization (confirmed=true)
 */
export type ConfirmationMode = 'FIRE_AND_FORGET' | 'WAIT_FOR_CONSENSUS';

/** The three reserved channel role names. FR-T07 */
const RESERVED_CHANNELS: ReadonlySet<string> = new Set(['stdIn', 'stdOut', 'stdErr']);

/**
 * Check whether a channel name is one of the three reserved roles.
 *
 * FR-T07: stdIn, stdOut, stdErr are reserved names.
 * FR-T11: Attempting to create a custom channel with a reserved name is rejected.
 *
 * @param name - Channel name to check
 * @returns `true` if the name is a reserved channel role
 */
export function isReservedChannel(name: string): boolean {
  return RESERVED_CHANNELS.has(name);
}

/**
 * Check whether a string is a valid ChannelRole.
 *
 * FR-T07, FR-T08: A valid channel role is either a reserved name or
 * a string matching the `custom:<name>` pattern where `<name>` is
 * non-empty and consists of alphanumeric characters, hyphens, and underscores.
 *
 * @param role - String to validate
 * @returns `true` if the string is a valid ChannelRole
 */
export function isValidChannelRole(role: string): role is ChannelRole {
  if (isReservedChannel(role)) {
    return true;
  }
  // FR-T08: Custom channels MUST use the custom: prefix with a valid name portion
  if (role.startsWith('custom:')) {
    const name = role.slice(7); // Length of 'custom:'
    return name.length > 0 && /^[a-zA-Z0-9_-]+$/.test(name);
  }
  return false;
}

/**
 * Check whether a string is a valid BackendKind.
 *
 * FR-T01: Built-in kinds are 'hcs', 'erc-log', 'kafka'.
 * FR-T05: Custom kinds use the 'custom:<type>' prefix.
 *
 * @param kind - String to validate
 * @returns `true` if the string is a valid BackendKind
 */
export function isValidBackendKind(kind: string): kind is BackendKind {
  if (kind === 'hcs' || kind === 'erc-log' || kind === 'kafka') {
    return true;
  }
  if (kind.startsWith('custom:')) {
    const typeName = kind.slice(7);
    return typeName.length > 0 && /^[a-zA-Z0-9_-]+$/.test(typeName);
  }
  return false;
}

/**
 * Return the three standard channel roles.
 *
 * FR-T07: Every registered agent has three mandatory channels.
 *
 * @returns Array of the three reserved ChannelRole values
 */
export function standardChannelRoles(): readonly ChannelRole[] {
  return ['stdIn', 'stdOut', 'stdErr'] as const;
}
