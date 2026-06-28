import { useCallback, useState } from 'react';
import { getManualConfig, saveManualConfig } from '../fallback-gateway/gatewayConfigApi';

/**
 * useFallbackSave — 统一 GET→mutate→PUT→刷新 流程
 *
 * Usage:
 *   const { execute, saving, saveMessage, setSaveMessage } = useFallbackSave({
 *     loadConfig, loadDeploymentStatuses
 *   });
 *
 *   await execute(
 *     (fresh) => { const p = clone(fresh); return p; },
 *     { successMsg: '保存成功', onSaved: () => setDraft({}) }
 *   );
 *
 * mutator 返回 null 表示静默中止（预检查已弹错误消息）。
 */
export const useFallbackSave = ({ loadConfig, loadDeploymentStatuses }) => {
  const [saving, setSaving] = useState(false);
  const [saveMessage, setSaveMessage] = useState(null);

  const execute = useCallback(async (mutator, { successMsg, onSaved } = {}) => {
    setSaving(true);
    setSaveMessage(null);
    const scrollY = window.scrollY;
    try {
      const { data: res } = await getManualConfig();
      const fresh = res?.data;
      if (!fresh) {
        setSaveMessage({ type: 'error', text: '无法获取最新配置，操作中止' });
        return false;
      }

      const mutatorResult = mutator(fresh);
      if (mutatorResult === null) return false; // mutator 已弹错误
      // mutator returns { payload, skippedFreeCount } (buildSavePayload) or a raw payload (legacy callers)
      const { payload, skippedFreeCount = 0 } =
        mutatorResult && typeof mutatorResult === 'object' && mutatorResult.payload
          ? mutatorResult
          : { payload: mutatorResult };

      let saveRes;
      try {
        saveRes = await saveManualConfig(payload);
      } catch (saveErr) {
        // Axios throws on non-2xx; extract server error message
        const serverMsg = saveErr?.response?.data?.message || saveErr?.message || '保存请求失败';
        setSaveMessage({ type: 'error', text: serverMsg });
        return false;
      }
      const { success, message } = saveRes?.data || {};
      if (!success) {
        setSaveMessage({ type: 'error', text: message || '操作失败' });
        return false;
      }

      // Warn if free deployment edits were silently dropped
      if (skippedFreeCount > 0) {
        setSaveMessage({
          type: 'warning',
          text: `保存成功，但 ${skippedFreeCount} 个免费部署的改动未保存（请在「免费模型池」面板编辑）`,
        });
      } else {
        setSaveMessage({ type: 'success', text: successMsg || '操作成功' });
      }
      onSaved?.();
      await Promise.all([loadConfig(), loadDeploymentStatuses()]);
      requestAnimationFrame(() => window.scrollTo(0, scrollY));
      return true;
    } catch (e) {
      setSaveMessage({ type: 'error', text: e.message || '操作异常' });
      return false;
    } finally {
      setSaving(false);
    }
  }, [loadConfig, loadDeploymentStatuses]);

  return { execute, saving, saveMessage, setSaveMessage };
};
