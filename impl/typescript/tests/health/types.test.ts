/**
 * Tests for health module type definitions.
 *
 * Spec reference: 005 data-model.md
 * Verifies enum values, valid role sets, and type structure.
 */

import { describe, it, expect } from 'vitest';
import {
  VALID_ROLES,
  VALID_NAT_TYPES,
  VALID_GPS_FIX_QUALITIES,
  LivenessState,
} from '../../src/health/types.js';
import type {
  NodeRole,
  NATType,
  GPSFixQuality,
  Capabilities,
  Location,
  LivenessRecord,
} from '../../src/health/types.js';

describe('Health Types', () => {
  // ---------------------------------------------------------------------------
  // NodeRole — FR-H02, FR-H05
  // ---------------------------------------------------------------------------
  describe('NodeRole', () => {
    it('VALID_ROLES contains buyer, seller, relay, validator', () => {
      expect(VALID_ROLES).toContain('buyer');
      expect(VALID_ROLES).toContain('seller');
      expect(VALID_ROLES).toContain('relay');
      expect(VALID_ROLES).toContain('validator');
    });

    it('VALID_ROLES has exactly 4 entries', () => {
      expect(VALID_ROLES).toHaveLength(4);
    });

    it('VALID_ROLES is frozen (readonly)', () => {
      expect(Object.isFrozen(VALID_ROLES)).toBe(true);
    });

    it('NodeRole type accepts valid values', () => {
      const roles: NodeRole[] = ['buyer', 'seller', 'relay', 'validator'];
      expect(roles).toHaveLength(4);
    });
  });

  // ---------------------------------------------------------------------------
  // NATType — FR-H03
  // ---------------------------------------------------------------------------
  describe('NATType', () => {
    it('VALID_NAT_TYPES contains all 4 types', () => {
      expect(VALID_NAT_TYPES).toContain('no-nat');
      expect(VALID_NAT_TYPES).toContain('endpoint-independent');
      expect(VALID_NAT_TYPES).toContain('address-dependent');
      expect(VALID_NAT_TYPES).toContain('address-and-port-dependent');
    });

    it('VALID_NAT_TYPES has exactly 4 entries', () => {
      expect(VALID_NAT_TYPES).toHaveLength(4);
    });

    it('NATType type accepts valid values', () => {
      const types: NATType[] = ['no-nat', 'endpoint-independent', 'address-dependent', 'address-and-port-dependent'];
      expect(types).toHaveLength(4);
    });
  });

  // ---------------------------------------------------------------------------
  // GPSFixQuality — FR-H03
  // ---------------------------------------------------------------------------
  describe('GPSFixQuality', () => {
    it('VALID_GPS_FIX_QUALITIES contains none, 2D, 3D', () => {
      expect(VALID_GPS_FIX_QUALITIES).toContain('none');
      expect(VALID_GPS_FIX_QUALITIES).toContain('2D');
      expect(VALID_GPS_FIX_QUALITIES).toContain('3D');
    });

    it('VALID_GPS_FIX_QUALITIES has exactly 3 entries', () => {
      expect(VALID_GPS_FIX_QUALITIES).toHaveLength(3);
    });

    it('GPSFixQuality type accepts valid values', () => {
      const fixes: GPSFixQuality[] = ['none', '2D', '3D'];
      expect(fixes).toHaveLength(3);
    });
  });

  // ---------------------------------------------------------------------------
  // LivenessState — FR-H18, FR-H19
  // ---------------------------------------------------------------------------
  describe('LivenessState', () => {
    it('has all 5 states', () => {
      expect(LivenessState.UNKNOWN).toBe('UNKNOWN');
      expect(LivenessState.ALIVE).toBe('ALIVE');
      expect(LivenessState.SUSPECT).toBe('SUSPECT');
      expect(LivenessState.DEAD).toBe('DEAD');
      expect(LivenessState.OFFLINE).toBe('OFFLINE');
    });

    it('enum has exactly 5 members', () => {
      const values = Object.values(LivenessState);
      expect(values).toHaveLength(5);
    });
  });

  // ---------------------------------------------------------------------------
  // Capabilities interface — FR-H03
  // ---------------------------------------------------------------------------
  describe('Capabilities', () => {
    it('accepts minimal capabilities (natReachability only)', () => {
      const caps: Capabilities = { natReachability: true };
      expect(caps.natReachability).toBe(true);
      expect(caps.natType).toBeUndefined();
      expect(caps.protocols).toBeUndefined();
    });

    it('accepts full capabilities', () => {
      const caps: Capabilities = {
        natReachability: false,
        natType: 'endpoint-independent',
        protocols: ['/adsb/v1', '/mesh/v2'],
      };
      expect(caps.natReachability).toBe(false);
      expect(caps.natType).toBe('endpoint-independent');
      expect(caps.protocols).toEqual(['/adsb/v1', '/mesh/v2']);
    });
  });

  // ---------------------------------------------------------------------------
  // Location interface — FR-H03
  // ---------------------------------------------------------------------------
  describe('Location', () => {
    it('accepts minimal location (lat + lon)', () => {
      const loc: Location = { lat: 37.7749, lon: -122.4194 };
      expect(loc.lat).toBe(37.7749);
      expect(loc.lon).toBe(-122.4194);
      expect(loc.alt).toBeUndefined();
      expect(loc.fix).toBeUndefined();
    });

    it('accepts full location', () => {
      const loc: Location = { lat: 37.7749, lon: -122.4194, alt: 15.0, fix: '3D' };
      expect(loc.alt).toBe(15.0);
      expect(loc.fix).toBe('3D');
    });
  });

  // ---------------------------------------------------------------------------
  // LivenessRecord interface — FR-H17, FR-H18
  // ---------------------------------------------------------------------------
  describe('LivenessRecord', () => {
    it('accepts a valid record', () => {
      const record: LivenessRecord = {
        senderAddress: '0x7E5F4552091A69125d5DfCb7b8C2659029395Bdf',
        currentState: LivenessState.ALIVE,
        lastDeadline: 1700000060000000000n,
        lastSequence: 1n,
        lastConsensusTimestamp: 1700000000000000000n,
      };
      expect(record.currentState).toBe(LivenessState.ALIVE);
      expect(record.lastDeadline).toBe(1700000060000000000n);
    });
  });
});
