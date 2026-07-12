import en from '../../locales/en/translation.json';
import zh from '../../locales/zh/translation.json';

describe('Chat translations', () => {
  it('defines the missing-link message at the key used by the page', () => {
    expect(zh.chat.no_link_configured).toBeTruthy();
    expect(en.chat.no_link_configured).toBeTruthy();
  });
});
