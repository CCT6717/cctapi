import { sanitizeHtml } from './sanitize';

describe('sanitizeHtml', () => {
  it('removes executable markup while preserving safe html', () => {
    const dirty = '<p>Hello</p><img src=x onerror="alert(1)"><script>alert(2)</script>';

    const clean = sanitizeHtml(dirty);

    expect(clean).toContain('<p>Hello</p>');
    expect(clean).not.toContain('onerror');
    expect(clean).not.toContain('<script>');
  });
});
