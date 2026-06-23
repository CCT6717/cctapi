import { useCallback, useState } from 'react';
import { API } from '../../helpers';
import { isSeparatorKey, computeInitialMode } from '../utils/deploymentMeta';

/**
 * useGatewayConfig — owns the gateway config fetch + the deploymentMode
 * UI state that is initialised from that config.
 *
 * Returns:
 *   config, loading, error          — fetch state
 *   loadConfig: () => Promise<void> — re-fetch and re-derive modes
 *   deploymentMode, setDeploymentMode — mode map + setter (mutated by UI)
 */
export const useGatewayConfig = () => {
  const [config, setConfig] = useState(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [deploymentMode, setDeploymentMode] = useState({});

  const loadConfig = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const res = await API.get('/api/fallback/gateway/config');
      const { success, data, message } = res.data || {};
      if (success && data) {
        setConfig(data);
        const modes = {};
        if (data.deployments) {
          Object.keys(data.deployments).forEach((id) => {
            if (isSeparatorKey(id)) return;
            modes[id] = computeInitialMode(data, id);
          });
        }
        setDeploymentMode(modes);
      } else {
        setError(message || '加载网关配置失败');
      }
    } catch (e) {
      setError(e.message || '加载网关配置失败');
    } finally {
      setLoading(false);
    }
  }, []);

  return { config, loading, error, loadConfig, deploymentMode, setDeploymentMode };
};
