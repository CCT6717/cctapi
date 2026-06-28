import { isSeparatorKey, isFreeDeployment } from './deploymentMeta';

/**
 * applyDraftEdits overlays user-editable draft fields onto payload deployments.
 * Skips separator keys and free deployments (managed by free pool panel).
 * Never touches strategy/pools or other carry-over fields.
 * Returns count of free deployments that had draft edits (for UX warning).
 */
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
      // Free deployments: skip edits (managed by free pool panel), but count
      // so the UI can warn the user their changes weren't saved.
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

/**
 * applyDeploymentModes maps deployment mode changes to VM-level fields.
 * Handles fixed / quota / error modes and their effect on routing_mode,
 * preferred_deployment, degrade flags, and quota fields.
 * Skips free deployments (managed by free pool panel).
 */
function applyDeploymentModes(payload, deploymentMode, deploymentOwnerVm) {
  if (!payload.virtual_models || !payload.deployments) return;
  Object.entries(deploymentMode).forEach(([depId, mode]) => {
    if (isSeparatorKey(depId)) return;
    const dep = payload.deployments[depId];
    if (!dep) return;
    if (isFreeDeployment(depId, dep)) return;
    const vmKey = deploymentOwnerVm[depId];
    if (!vmKey || !payload.virtual_models[vmKey]) return;
    const vm = payload.virtual_models[vmKey];

    if (mode === 'fixed') {
      vm.routing_mode = 'fixed';
      vm.preferred_deployment = depId;
      vm.allow_degrade_to_low = false;
      vm.allow_degrade_to_free = false;
    } else if (mode === 'quota') {
      vm.routing_mode = 'fallback';
      vm.preferred_deployment = depId;
    } else if (mode === 'error') {
      vm.routing_mode = 'fallback';
      vm.preferred_deployment = depId;
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
 * Returns { payload, skippedFreeCount } so callers can warn about free edits.
 */
export function buildSavePayload(fresh, { draftDeployments, draftRoutingVm, deploymentMode, deploymentOwnerVm }) {
  const payload = JSON.parse(JSON.stringify(fresh));
  const skippedFreeCount = applyDraftEdits(payload, draftDeployments);
  applyDeploymentModes(payload, deploymentMode, deploymentOwnerVm);
  applyRoutingStrategy(payload, draftRoutingVm);
  return { payload, skippedFreeCount };
}
