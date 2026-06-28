import { useCallback, useState } from 'react';
import { API } from '../../helpers';
import { isSeparatorKey } from '../utils/deploymentMeta';

/**
 * useGatewayConfig — owns the gateway config fetch + the deploymentMode
 * UI state that is initialised from that config.
 *
 * Returns:
 *   config, loading, error          — fetch state
 *   allDeployments                  — full deployment map (editor config, includes free pool)
 *   loadConfig: () => Promise<void> — re-fetch and re-derive modes
 *   deploymentMode, setDeploymentMode — mode map + setter (mutated by UI)
 */
export const useGatewayConfig = () => {
  const [config, setConfig] = useState(null);
  const [allDeployments, setAllDeployments] = useState(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [deploymentMode, setDeploymentMode] = useState({});

  const loadConfig = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      // ponytail: parallel fetch, both fail independently
      const [manualRes, editorRes] = await Promise.all([
        API.get('/api/fallback/manual-config'),
        API.get('/api/fallback/editor/config').catch(() => null),
      ]);
      const { success, data, message } = manualRes.data || {};
      if (success && data) {
        setConfig(data);
        setDeploymentMode({});
      } else {
        setError(message || '加载网关配置失败');
      }
      // Extract full deployments from editor config (includes free pool)
      const editorData = editorRes?.data?.data;
      if (editorData?.deployments) {
        const full = {};
        editorData.deployments.forEach((dep) => {
          if (dep?.id && !isSeparatorKey(dep.id)) {
            full[dep.id] = dep;
          }
        });
        setAllDeployments(full);
      }
    } catch (e) {
      setError(e.message || '加载网关配置失败');
    } finally {
      setLoading(false);
    }
  }, []);

  return { config, allDeployments, loading, error, loadConfig, deploymentMode, setDeploymentMode };
};
