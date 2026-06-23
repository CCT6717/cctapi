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
    try {
      const { data: res } = await getManualConfig();
      const fresh = res?.data;
      if (!fresh) {
        setSaveMessage({ type: 'error', text: '无法获取最新配置，操作中止' });
        return false;
      }

      const payload = mutator(fresh);
      if (payload === null) return false; // mutator 已弹错误

      const { success, message } = (await saveManualConfig(payload)).data || {};
      if (!success) {
        setSaveMessage({ type: 'error', text: message || '操作失败' });
        return false;
      }

      setSaveMessage({ type: 'success', text: successMsg || '操作成功' });
      onSaved?.();
      await Promise.all([loadConfig(), loadDeploymentStatuses()]);
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
