export const PROVIDER_DISPLAY = {
  openrouter: { title: 'OpenRouter', color: 'purple', icon: 'cloud' },
  groq: { title: 'Groq', color: 'orange', icon: 'lightning' },
  google: { title: 'Google AI', color: 'blue', icon: 'google' },
  kilo: { title: 'Kilo', color: 'teal', icon: 'bolt' },
  pollinations: { title: 'Pollinations', color: 'green', icon: 'leaf' },
  ovh: { title: 'OVH Cloud', color: 'blue', icon: 'server' },
  nvidia: { title: 'NVIDIA', color: 'green', icon: 'microchip' },
  cohere: { title: 'Cohere', color: 'teal', icon: 'comment alternate' },
  huggingface: { title: 'Hugging Face', color: 'yellow', icon: 'smile' },
  llm7: { title: 'LLM7', color: 'violet', icon: 'bolt' },
  opencode: { title: 'OpenCode', color: 'grey', icon: 'code' },
  aihorde: { title: 'AI Horde', color: 'olive', icon: 'users' },
  routeway: { title: 'Routeway', color: 'blue', icon: 'road' },
  bazaarlink: { title: 'Bazaarlink', color: 'pink', icon: 'linkify' },
  ainative: { title: 'AINative', color: 'purple', icon: 'brain' },
  agnes: { title: 'Agnes', color: 'red', icon: 'magic' },
  reka: { title: 'Reka', color: 'orange', icon: 'star' },
  siliconflow: { title: 'SiliconFlow', color: 'violet', icon: 'microchip' },
  zhipu: { title: 'Zhipu AI', color: 'red', icon: 'brain' },
  mistral: { title: 'Mistral', color: 'yellow', icon: 'wind' },
  togetherai: { title: 'Together AI', color: 'pink', icon: 'users' },
  novita: { title: 'Novita', color: 'olive', icon: 'rocket' },
  cloudflare: { title: 'Cloudflare', color: 'orange', icon: 'shield' },
  cerebras: { title: 'Cerebras', color: 'blue', icon: 'microchip' },
  sambanova: { title: 'SambaNova', color: 'purple', icon: 'server' },
  github: { title: 'GitHub Models', color: 'grey', icon: 'github' },
  chutes: { title: 'Chutes', color: 'green', icon: 'bolt' },
  fireworks: { title: 'Fireworks', color: 'red', icon: 'fire' },
  nebius: { title: 'Nebius', color: 'teal', icon: 'cloud' },
  lambdalabs: { title: 'Lambda Labs', color: 'violet', icon: 'lambda' },
};

export const LIMIT_FIELDS = ['rpm_limit', 'rpd_limit', 'tpm_limit', 'tpd_limit'];

export const formatNumber = (value) => {
  const n = Number(value);
  if (!Number.isFinite(n)) return '-';
  return new Intl.NumberFormat('zh-CN').format(n);
};

export const formatLimit = (value) => {
  const n = Number(value);
  if (!Number.isFinite(n)) return '-';
  return n === 0 ? 'unlimited' : formatNumber(n);
};

export const validateLimits = (limits) => {
  if (!limits) return true;
  return LIMIT_FIELDS.every((field) => {
    const value = limits[field];
    return value === undefined
      || value === ''
      || value === null
      || (Number.isFinite(Number(value)) && Number(value) >= 0);
  });
};
