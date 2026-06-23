export const AUTO_FREE_DEPLOYMENT_RE = /^free:(openrouter|groq)-(\d+|[a-f0-9]{8})$/i;

export const isAutoFreeDeploymentId = (id) =>
  AUTO_FREE_DEPLOYMENT_RE.test(String(id || ''));

export const providerFromDeploymentId = (id) => {
  const match = String(id || '').match(AUTO_FREE_DEPLOYMENT_RE);
  return match ? match[1].toLowerCase() : '-';
};

// 注意：这是"自动免费池体系"专用判定——只认 free:openrouter-x / free:groq-x
// 这种 recognized provider 的 id。跟 deploymentMeta.js 的通用 isFreeDeployment
// 是不同概念：后者接受任何 free:* 前缀和 pool==='free'。别混用。
export const isAutoFreeDeployment = (id, dep) =>
  dep?.pool === 'free' || isAutoFreeDeploymentId(id);
