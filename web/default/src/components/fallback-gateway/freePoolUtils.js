export const AUTO_FREE_DEPLOYMENT_RE = /^free:(openrouter|groq)-(\d+|[a-f0-9]{8})$/i;

export const isAutoFreeDeploymentId = (id) =>
  AUTO_FREE_DEPLOYMENT_RE.test(String(id || ''));

export const providerFromDeploymentId = (id) => {
  const match = String(id || '').match(AUTO_FREE_DEPLOYMENT_RE);
  return match ? match[1].toLowerCase() : '-';
};

export const isFreeDeployment = (id, dep) =>
  dep?.pool === 'free' || isAutoFreeDeploymentId(id);
