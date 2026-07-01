import { isSeparatorKey, isFreeDeployment } from './deploymentMeta';

function applyDraftEdits(payload, draftDeployments) {
  let skippedFreeCount = 0;
  if (!payload.deployments) return skippedFreeCount;
  Object.keys(payload.deployments).forEach((id) => {
    if (isSeparatorKey(id)) return;
    const dep = payload.deployments[id];
    const isFree = isFreeDeployment(id, dep);
    const draft = draftDeployments[id];
    if (!draft) return;
    if (isFree) {
      skippedFreeCount += 1;
      return;
    }
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
  return skippedFreeCount;
}

function applyDeploymentModes(payload, deploymentMode, deploymentOwnerVm) {
  if (!payload.virtual_models || !payload.deployments) return;
  Object.entries(deploymentMode).forEach(([depId, mode]) => {
    if (isSeparatorKey(depId)) return;
    const dep = payload.deployments[depId];
    if (!dep) return;
    if (isFreeDeployment(depId, dep)) return;
    const vmKeys = deploymentOwnerVm[depId];
    if (!Array.isArray(vmKeys) || vmKeys.length === 0) return;
    vmKeys.forEach((vmKey) => {
      if (!payload.virtual_models[vmKey]) return;
      const vm = payload.virtual_models[vmKey];
      if (mode === 'fixed') {
        vm.routing_mode = 'fallback';
        vm.preferred_deployment = depId;
        vm.allow_degrade_to_low = false;
        vm.allow_degrade_to_free = false;
        if (payload.deployments[depId]) {
          payload.deployments[depId].daily_limit_tokens = 0;
        }
      } else if (mode === 'quota') {
        vm.routing_mode = 'fallback';
        vm.preferred_deployment = depId;
        vm.allow_degrade_to_low = false;
        vm.allow_degrade_to_free = false;
      } else if (mode === 'error') {
        vm.routing_mode = 'fallback';
        vm.preferred_deployment = depId;
        vm.allow_degrade_to_low = true;
        vm.allow_degrade_to_free = true;
      }
    });
  });
}

function applyRoutingStrategy(payload, draftRoutingVm) {
  if (!payload.virtual_models) return;
  Object.keys(draftRoutingVm).forEach((vmKey) => {
    if (!payload.virtual_models[vmKey]) return;
    const target = draftRoutingVm[vmKey];
    if (target) payload.virtual_models[vmKey].strategy = target;
  });
}

export function buildSavePayload(fresh, { draftDeployments, draftRoutingVm, deploymentMode, deploymentOwnerVm }) {
  const payload = JSON.parse(JSON.stringify(fresh));
  const skippedFreeCount = applyDraftEdits(payload, draftDeployments);
  applyDeploymentModes(payload, deploymentMode, deploymentOwnerVm);
  applyRoutingStrategy(payload, draftRoutingVm);
  return { payload, skippedFreeCount };
}
