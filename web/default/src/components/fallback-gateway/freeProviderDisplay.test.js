import { formatLimit, validateLimits } from './freeProviderDisplay';

describe('freeProviderDisplay', () => {
  test('formatLimit treats zero as unlimited', () => {
    expect(formatLimit(0)).toBe('unlimited');
  });

  test('formatLimit formats missing values as dash', () => {
    expect(formatLimit(undefined)).toBe('-');
    expect(formatLimit('not-a-number')).toBe('-');
  });

  test('validateLimits rejects negative values', () => {
    expect(validateLimits({ rpm_limit: -1 })).toBe(false);
    expect(validateLimits({ rpm_limit: 0, rpd_limit: '10' })).toBe(true);
  });
});
