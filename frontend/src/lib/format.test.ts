import { describe, expect, it } from 'vitest';
import { formatSize } from './format';

describe('formatSize', () => {
  it('mirrors core.FormatSize exactly (base-1024, 0 decimals for bytes, 1 above)', () => {
    expect(formatSize(0)).toBe('0 B');
    expect(formatSize(512)).toBe('512 B');
    expect(formatSize(1023)).toBe('1023 B');
    expect(formatSize(1024)).toBe('1.0 KB');
    expect(formatSize(1536)).toBe('1.5 KB');
    expect(formatSize(524288000)).toBe('500.0 MB');
    expect(formatSize(1073741824)).toBe('1.0 GB');
    expect(formatSize(1099511627776)).toBe('1.0 TB');
  });

  it('clamps garbage inputs to "0 B"', () => {
    expect(formatSize(-42)).toBe('0 B');
    expect(formatSize(Number.NaN)).toBe('0 B');
  });
});
