/**
 * Channel validation -- ChannelRole parsing and validation.
 *
 * Spec reference: 004 spec.md
 *   - FR-T07: Reserved channel roles (stdIn, stdOut, stdErr).
 *   - FR-T08: Custom channels use custom:<name> namespace prefix.
 *   - FR-T11: ReservedChannelName error for misuse of reserved names.
 *
 * Spec reference: 004 data-model.md ChannelRole enum.
 */

import type { ChannelRole } from './types.js';
import { isReservedChannel, isValidChannelRole } from './types.js';
import { invalidTopicRef } from './errors.js';

/**
 * Parse and validate a channel role string.
 *
 * FR-T07: Standard roles (stdIn, stdOut, stdErr) are reserved.
 * FR-T08: Custom channels MUST use the custom:<name> prefix where <name>
 * is non-empty and consists of alphanumeric characters, hyphens, and underscores.
 *
 * @param s - String to parse as a ChannelRole
 * @returns Validated ChannelRole value
 * @throws TopicError NEURON-TOPIC-001 if the string is not a valid channel role
 */
export function channelRoleFromString(s: string): ChannelRole {
  if (isValidChannelRole(s)) {
    return s;
  }

  // Provide a specific error message for common mistakes
  if (s.length === 0) {
    throw invalidTopicRef('Channel role must be non-empty');
  }

  if (!s.startsWith('custom:') && !isReservedChannel(s)) {
    throw invalidTopicRef(
      `Invalid channel role: "${s}". Must be 'stdIn', 'stdOut', 'stdErr', or 'custom:<name>'`,
    );
  }

  // custom: prefix but invalid name portion
  throw invalidTopicRef(
    `Invalid custom channel name: "${s}". The name portion after 'custom:' must be non-empty and contain only alphanumeric characters, hyphens, and underscores`,
  );
}

/**
 * Validate that a channel name is not a reserved name when used for custom channels.
 *
 * FR-T07, FR-T11: Attempting to create a custom channel with a reserved name
 * MUST be rejected.
 *
 * @param name - Channel name to validate
 * @throws TopicError NEURON-TOPIC-001 if the name is a reserved channel role
 */
export function assertNotReservedChannel(name: string): void {
  if (isReservedChannel(name)) {
    throw invalidTopicRef(
      `Channel name "${name}" is a reserved channel role. Use 'custom:' prefix for custom channels`,
    );
  }
}
