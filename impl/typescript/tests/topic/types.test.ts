/**
 * Tests for Topic System type definitions.
 *
 * Spec reference: 004 spec.md
 *   - FR-T01: BackendKind validation.
 *   - FR-T07: Reserved channel roles.
 *   - FR-T08: Custom channel namespace prefix.
 */

import { describe, it, expect } from 'vitest';
import {
  isReservedChannel,
  isValidChannelRole,
  isValidBackendKind,
  standardChannelRoles,
} from '../../src/topic/types.js';

describe('isReservedChannel', () => {
  it('should return true for stdIn', () => {
    expect(isReservedChannel('stdIn')).toBe(true);
  });

  it('should return true for stdOut', () => {
    expect(isReservedChannel('stdOut')).toBe(true);
  });

  it('should return true for stdErr', () => {
    expect(isReservedChannel('stdErr')).toBe(true);
  });

  it('should return false for custom channels', () => {
    expect(isReservedChannel('custom:myChannel')).toBe(false);
  });

  it('should return false for empty string', () => {
    expect(isReservedChannel('')).toBe(false);
  });

  it('should return false for unknown names', () => {
    expect(isReservedChannel('stdin')).toBe(false); // Case-sensitive
    expect(isReservedChannel('STDOUT')).toBe(false);
  });
});

describe('isValidChannelRole', () => {
  it('should accept reserved channel roles', () => {
    expect(isValidChannelRole('stdIn')).toBe(true);
    expect(isValidChannelRole('stdOut')).toBe(true);
    expect(isValidChannelRole('stdErr')).toBe(true);
  });

  it('should accept valid custom channel roles', () => {
    expect(isValidChannelRole('custom:myChannel')).toBe(true);
    expect(isValidChannelRole('custom:my-channel')).toBe(true);
    expect(isValidChannelRole('custom:my_channel')).toBe(true);
    expect(isValidChannelRole('custom:abc123')).toBe(true);
    expect(isValidChannelRole('custom:A')).toBe(true);
  });

  it('should reject custom channels with empty name portion', () => {
    expect(isValidChannelRole('custom:')).toBe(false);
  });

  it('should reject custom channels with invalid characters', () => {
    expect(isValidChannelRole('custom:my channel')).toBe(false); // Space
    expect(isValidChannelRole('custom:my.channel')).toBe(false); // Dot
    expect(isValidChannelRole('custom:my/channel')).toBe(false); // Slash
  });

  it('should reject names without custom: prefix that are not reserved', () => {
    expect(isValidChannelRole('myChannel')).toBe(false);
    expect(isValidChannelRole('stdin')).toBe(false);
    expect(isValidChannelRole('')).toBe(false);
  });
});

describe('isValidBackendKind', () => {
  it('should accept built-in backend kinds', () => {
    expect(isValidBackendKind('hcs')).toBe(true);
    expect(isValidBackendKind('erc-log')).toBe(true);
    expect(isValidBackendKind('kafka')).toBe(true);
  });

  it('should accept valid custom backend kinds', () => {
    expect(isValidBackendKind('custom:mqtt')).toBe(true);
    expect(isValidBackendKind('custom:my-transport')).toBe(true);
    expect(isValidBackendKind('custom:my_transport')).toBe(true);
  });

  it('should reject custom backend kinds with empty type portion', () => {
    expect(isValidBackendKind('custom:')).toBe(false);
  });

  it('should reject unknown backend kinds', () => {
    expect(isValidBackendKind('mqtt')).toBe(false);
    expect(isValidBackendKind('')).toBe(false);
    expect(isValidBackendKind('HCS')).toBe(false);
  });
});

describe('standardChannelRoles', () => {
  it('should return three reserved roles', () => {
    const roles = standardChannelRoles();
    expect(roles).toHaveLength(3);
    expect(roles).toContain('stdIn');
    expect(roles).toContain('stdOut');
    expect(roles).toContain('stdErr');
  });

  it('should return a frozen array', () => {
    const roles1 = standardChannelRoles();
    const roles2 = standardChannelRoles();
    expect(roles1).toEqual(roles2);
  });
});
