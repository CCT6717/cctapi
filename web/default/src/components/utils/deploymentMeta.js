/**
 * deploymentMeta.js — 通用 deployment 元数据 helper。
 * 单一真相源：所有 deployment 相关的纯判定/格式化逻辑集中于此。
 *
 * 注意：本模块的 isFreeDeployment 是"通用 free 判定"——
 *   dep?.pool === 'free'  或  id 以 'free:' 开头（任何 free: 服务）
 * 它跟 freePoolUtils.js 的 isAutoFreeDeployment 是不同概念：
 *   后者只认 free:openrouter-x / free:groq-x 这种"自动免费池体系"的 id。
 */

export const isSeparatorKey = (id) => String(id || '').startsWith('---');

/**
 * 通用 free 部署判定。
 * - 有 dep 时：pool === 'free' 或 id 以 'free:' 开头
 * - 无 dep 时：退化为 id 以 'free:' 开头（零回归）
 */
export const isFreeDeployment = (id, dep) =>
  dep?.pool === 'free' || String(id || '').startsWith('free:');

export const slugModelName = (name) =>
  String(name || '').toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/-+/g, '-').replace(/^-|-$/g, '');

/**
 * computeInitialMode — 从原始 config 推断一个部署的 mode。
 * - daily_limit_tokens > 0   → 'quota'
 * - pool 以 _fixed_ 开头       → 'fixed'
 * - 否则                       → 'error'
 */
export const computeInitialMode = (data, depId) => {
  const dep = data?.deployments?.[depId];
  if (!dep) return 'error';
  if (dep.daily_limit_tokens > 0) return 'quota';
  if (dep.pool && dep.pool.startsWith('_fixed_')) return 'fixed';
  return 'error';
};

export const formatStatusTime = (value) => {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return String(value);
  return date.toLocaleString('zh-CN', { hour12: false });
};

/**
 * getDeploymentStatusMeta — 把运行时状态转成 { label, color, detail }。
 */
export const getDeploymentStatusMeta = (status) => {
  const alertType = status?.alert_type || '';
  if (alertType === 'cooldown') {
    return {
      label: '冷却中',
      color: 'orange',
      detail: `冷却至 ${formatStatusTime(status.cooldown_until)}`,
    };
  }
  if (alertType === 'exhausted') {
    return {
      label: '已耗尽',
      color: 'red',
      detail: `耗尽至 ${formatStatusTime(status.exhausted_until)}`,
    };
  }
  if (alertType === 'hard_limit') {
    return {
      label: '硬限额',
      color: 'red',
      detail: `用量 ${status?.usage_percent || '-'}`,
    };
  }
  if (alertType === 'soft_limit') {
    return {
      label: '软限额',
      color: 'yellow',
      detail: `用量 ${status?.usage_percent || '-'}`,
    };
  }
  return {
    label: '可用',
    color: 'green',
    detail: status ? `用量 ${status.usage_percent || '-'}` : '暂无状态数据',
  };
};

/**
 * getDeploymentOwnerNames — 哪些虚拟模型拥有此部署。
 * projectedVMs: 已投影的 VM 数组（含 fallback_order）
 */
export const getDeploymentOwnerNames = (projectedVMs, deploymentId) =>
  (projectedVMs || [])
    .filter((vm) => (vm.fallback_order || []).includes(deploymentId))
    .map((vm) => vm.name || '未命名虚拟模型');
