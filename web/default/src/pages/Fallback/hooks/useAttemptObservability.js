import { useCallback, useEffect, useRef, useState } from 'react';
import { API } from '../../../helpers';

const POLL_INTERVAL_MS = 30000;
const UNAVAILABLE_ERROR = '精准尝试数据暂时不可用';
const ARRAY_FIELDS = [
  'top_deployments',
  'top_providers',
  'top_models',
  'error_categories',
  'outcomes',
  'recent_chains',
];

const normalizeData = (data) => {
  const normalized = data && typeof data === 'object' ? { ...data } : {};
  ARRAY_FIELDS.forEach((field) => {
    normalized[field] = Array.isArray(normalized[field]) ? normalized[field] : [];
  });
  return normalized;
};

export const useAttemptObservability = () => {
  const mountedRef = useRef(false);
  const [data, setData] = useState(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    if (mountedRef.current) {
      setLoading(true);
    }

    try {
      const response = await API.get('/api/fallback/attempt-observability');
      if (!response.data?.success) {
        throw new Error('attempt observability unavailable');
      }
      if (mountedRef.current) {
        setData(normalizeData(response.data.data));
        setError('');
      }
    } catch {
      if (mountedRef.current) {
        setError(UNAVAILABLE_ERROR);
      }
    } finally {
      if (mountedRef.current) {
        setLoading(false);
      }
    }
  }, []);

  useEffect(() => {
    mountedRef.current = true;
    refresh();
    const timer = window.setInterval(refresh, POLL_INTERVAL_MS);

    return () => {
      mountedRef.current = false;
      window.clearInterval(timer);
    };
  }, [refresh]);

  return { data, error, loading, refresh };
};
