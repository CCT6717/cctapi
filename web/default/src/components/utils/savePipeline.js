import { isSeparatorKey, isFreeDeployment, slugModelName } from './deploymentMeta';

/**
 * applyDraftEdits overlays user-editable draft fields onto payload deployments.
 * Skips separator keys and free deployments.
 * Never touches strategy/pools or other carry-over fields.
 */
function applyDraftEdits(payload, draftDeployments) {
  if (!payload.deployments) return;
  Object.keys(payload.deployments).forEach((id) => {
    if (isSeparatorKey(id)) return;
    const dep = payload.deployments[id];
    if (isFreeDeployment(id, dep)) return;
    const draft = draftDeployments[id];
    if (!draft) return;

    if (draft.enabled !== undefined) dep.enabled = draft.enabled;
    if (draft.priority !== undefined) dep.priority = Number(draft.priority);
    if (draft.weight !== undefined) dep.weight = Number(draft.weight);
    if (draft.quota_mode !== undefined) dep.quota_mode = draft.quota_mode;
    if (draft.daily_limit_tokens !== undefined) dep.daily_limit_tokens = Number(draft.daily_limit_tokens) || 0;

    const s = Number(draft.soft_limit_ratio);
    dep.soft_limit_ratio = Number.isFinite(s) ? s : 0;

    const h = Number(draft.hard_limit_ratio);
    dep.hard_limit_ratio = Number.isFinite(h) ? h : 0;

    payload.deployments[id] = dep;
  });
}

/**
 * applyDeploymentModes maps deployment mode changes to VM-level fields.
 * fixed mode → single-deployment pool + disable degrade
 * restoring from fixed → reset to default pool
 */
function applyDeploymentModes(payload, deploymentMode, deploymentOwnerVm) {
  if (!payload.virtual_models || !payload.deployments) return;
  Object.entries(deploymentMode).forEach(([depId, mode]) => {
    if (isSeparatorKey(depId)) return;
    const dep = payload.deployments[depId];
    if (!dep) return;
    const vmKey = deploymentOwnerVm[depId];
    if (!vmKey || !payload.virtual_models[vmKey]) return;
    const vm = payload.virtual_models[vmKey];

    if (mode === 'fixed') {
      const fixedPool = `_fixed_${slugModelName(vmKey)}_${depId}`;
      vm.pools = [fixedPool];
      vm.allow_degrade_to_low = false;
      vm.allow_degrade_to_free = false;
      dep.pool = fixedPool;
    } else if (mode === 'error' && dep.pool && dep.pool.startsWith('_fixed_')) {
      dep.pool = 'default';
      vm.pools = Array.from(new Set([...(vm.pools || []).filter((p) => !p.startsWith('_fixed_')), 'default']));
      vm.allow_degrade_to_low = true;
      vm.allow_degrade_to_free = true;
    }
  });
}

/**
 * applyRoutingStrategy overlays draft strategy changes onto VM config.
 */
function applyRoutingStrategy(payload, draftRoutingVm) {
  if (!payload.virtual_models) return;
  Object.keys(draftRoutingVm).forEach((vmKey) => {
    if (!payload.virtual_models[vmKey]) return;
    const target = draftRoutingVm[vmKey];
    if (target) payload.virtual_models[vmKey].strategy = target;
  });
}

/**
 * buildSavePayload builds the PUT payload from fresh config + all draft state.
 * Pure function — does not mutate input, always returns a new object.
 */
export function buildSavePayload(fresh, { draftDeployments, draftRoutingVm, deploymentMode, deploymentOwnerVm }) {
  const payload = JSON.parse(JSON.stringify(fresh));
  applyDraftEdits(payload, draftDeployments);
  applyDeploymentModes(payload, deploymentMode, deploymentOwnerVm);
  applyRoutingStrategy(payload, draftRoutingVm);
  return payload;
}
