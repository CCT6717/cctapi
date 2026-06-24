import { useCallback, useState } from 'react';
import { API } from '../../helpers';

/**
 * useChannels — fetches all channels (paginated) and parses the
 * comma-separated models string into an array.
 * Optional data: failures are swallowed silently.
 *
 * Returns:
 *   channels: [{ id, name, models: string[] }]
 *   loadChannels: () => Promise<void>
 */
export const useChannels = () => {
  const [channels, setChannels] = useState([]);

  const loadChannels = useCallback(async () => {
    try {
      const allChannels = [];
      let page = 0;
      let hasMore = true;
      while (hasMore) {
        const res = await API.get(`/api/channel/?p=${page}`);
        const { success, data } = res.data || {};
        if (!success || !Array.isArray(data) || data.length === 0) {
          hasMore = false;
          break;
        }
        allChannels.push(...data);
        if (data.length < 10) {
          hasMore = false;
        } else {
          page++;
        }
      }
      const parsed = allChannels.map((ch) => ({
        id: ch.id,
        name: ch.name || `渠道 ${ch.id}`,
        base_url: ch.base_url || '',
        key: ch.key || '',
        models: (ch.models || '').split(',').map((m) => m.trim()).filter(Boolean),
      }));
      setChannels(parsed);
    } catch (e) {
      // 静默失败，渠道数据可选
    }
  }, []);

  return { channels, loadChannels };
};
