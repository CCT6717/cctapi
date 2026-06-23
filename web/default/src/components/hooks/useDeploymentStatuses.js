import { useCallback, useState } from 'react';
import { API } from '../../helpers';

/**
 * useDeploymentStatuses — fetches runtime status for all deployments.
 * Optional data: failures are swallowed silently.
 *
 * Returns:
 *   deploymentStatuses, loadDeploymentStatuses
 */
export const useDeploymentStatuses = () => {
  const [deploymentStatuses, setDeploymentStatuses] = useState({});

  const loadDeploymentStatuses = useCallback(async () => {
    try {
      const res = await API.get('/api/fallback/deployments/runtime-status');
      const { success, data } = res.data || {};
      if (success && data) {
        setDeploymentStatuses(data);
      }
    } catch (e) {
      // 静默失败，状态数据可选
    }
  }, []);

  return { deploymentStatuses, loadDeploymentStatuses };
};
