import { useCallback, useEffect, useState } from 'react';
import {
  loadChannelModels,
  showError,
  showInfo,
  showSuccess,
} from '../../helpers';
import * as channelApi from '../channelApi';
import { ITEMS_PER_PAGE } from '../../constants';
import { processChannelData } from '../utils/channelRenderers';

export const useChannelsTable = (t) => {
  const [channels, setChannels] = useState([]);
  const [loading, setLoading] = useState(true);
  const [activePage, setActivePage] = useState(1);
  const [searchKeyword, setSearchKeyword] = useState('');
  const [searching, setSearching] = useState(false);
  const [updatingBalance, setUpdatingBalance] = useState(false);

  const loadChannels = useCallback(async (startIdx) => {
    const res = await channelApi.getChannels(startIdx);
    const { success, message, data } = res.data;
    if (success) {
      let localChannels = data.map(processChannelData);
      if (startIdx === 0) {
        setChannels(localChannels);
      } else {
        setChannels((prev) => {
          const newChannels = [...prev];
          newChannels.splice(
            startIdx * ITEMS_PER_PAGE,
            data.length,
            ...localChannels
          );
          return newChannels;
        });
      }
    } else {
      showError(message);
    }
    setLoading(false);
  }, []);

  const onPaginationChange = useCallback(
    (e, { activePage: page }) => {
      (async () => {
        if (
          page ===
          Math.ceil(channels.length / ITEMS_PER_PAGE) + 1
        ) {
          await loadChannels(page - 1);
        }
        setActivePage(page);
      })();
    },
    [channels.length, loadChannels]
  );

  const refresh = useCallback(async () => {
    setLoading(true);
    await loadChannels(activePage - 1);
  }, [activePage, loadChannels]);

  useEffect(() => {
    loadChannels(0).catch((reason) => showError(reason));
    loadChannelModels();
  }, [loadChannels]);

  const manageChannel = useCallback(
    async (id, action, idx, value) => {
      const data = { id };
      let res;
      switch (action) {
        case 'delete':
          res = await channelApi.deleteChannel(id);
          break;
        case 'enable':
          data.status = 1;
          res = await channelApi.updateChannel(data);
          break;
        case 'disable':
          data.status = 2;
          res = await channelApi.updateChannel(data);
          break;
        case 'priority':
          if (value === '') return;
          data.priority = parseInt(value);
          res = await channelApi.updateChannel(data);
          break;
        case 'weight':
          if (value === '') return;
          data.weight = parseInt(value);
          if (data.weight < 0) data.weight = 0;
          res = await channelApi.updateChannel(data);
          break;
      }
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('channel.messages.operation_success'));
        const channel = res.data.data;
        setChannels((prev) => {
          const newChannels = [...prev];
          const realIdx = (activePage - 1) * ITEMS_PER_PAGE + idx;
          if (!newChannels[realIdx]) return prev;
          if (action === 'delete') {
            newChannels[realIdx] = {
              ...newChannels[realIdx],
              deleted: true,
            };
          } else {
            newChannels[realIdx] = {
              ...newChannels[realIdx],
              status: channel.status,
            };
          }
          return newChannels;
        });
      } else {
        showError(message);
      }
    },
    [activePage, t]
  );

  const searchChannels = useCallback(async () => {
    if (searchKeyword === '') {
      await loadChannels(0);
      setActivePage(1);
      return;
    }
    setSearching(true);
    const res = await channelApi.searchChannels(searchKeyword);
    const { success, message, data } = res.data;
    if (success) {
      setChannels(data.map(processChannelData));
      setActivePage(1);
    } else {
      showError(message);
    }
    setSearching(false);
  }, [searchKeyword, loadChannels]);

  const switchTestModel = useCallback(
    (idx, model) => {
      setChannels((prev) => {
        const newChannels = [...prev];
        const realIdx = (activePage - 1) * ITEMS_PER_PAGE + idx;
        if (newChannels[realIdx]) {
          newChannels[realIdx] = {
            ...newChannels[realIdx],
            test_model: model,
          };
        }
        return newChannels;
      });
    },
    [activePage]
  );

  const testChannel = useCallback(
    async (id, name, idx, m) => {
      const res = await channelApi.testChannel(id, m);
      const { success, message, time, model } = res.data;
      if (success) {
        setChannels((prev) => {
          const newChannels = [...prev];
          const realIdx = (activePage - 1) * ITEMS_PER_PAGE + idx;
          if (newChannels[realIdx]) {
            newChannels[realIdx] = {
              ...newChannels[realIdx],
              response_time: time * 1000,
              test_time: Date.now() / 1000,
            };
          }
          return newChannels;
        });
        showSuccess(
          t('channel.messages.test_success', {
            name,
            model,
            time,
            message,
          })
        );
      } else {
        showError(message);
      }
    },
    [activePage, t]
  );

  const testChannels = useCallback(
    async (scope) => {
      const res = await channelApi.testChannels(scope);
      const { success, message } = res.data;
      if (success) {
        showInfo(t('channel.messages.test_all_started'));
      } else {
        showError(message);
      }
    },
    [t]
  );

  const deleteAllDisabledChannels = useCallback(async () => {
    const res = await channelApi.deleteDisabledChannels();
    const { success, message, data } = res.data;
    if (success) {
      showSuccess(
        t('channel.messages.delete_disabled_success', { count: data })
      );
      await refresh();
    } else {
      showError(message);
    }
  }, [refresh, t]);

  const updateChannelBalance = useCallback(
    async (id, name, idx) => {
      const res = await channelApi.updateChannelBalance(id);
      const { success, message, balance } = res.data;
      if (success) {
        setChannels((prev) => {
          const newChannels = [...prev];
          const realIdx = (activePage - 1) * ITEMS_PER_PAGE + idx;
          if (newChannels[realIdx]) {
            newChannels[realIdx] = {
              ...newChannels[realIdx],
              balance,
              balance_updated_time: Date.now() / 1000,
            };
          }
          return newChannels;
        });
        showSuccess(t('channel.messages.balance_update_success', { name }));
      } else {
        showError(message);
      }
    },
    [activePage, t]
  );

  const updateAllChannelsBalance = useCallback(async () => {
    setUpdatingBalance(true);
    const res = await channelApi.updateAllChannelsBalance();
    const { success, message } = res.data;
    if (success) {
      showInfo(t('channel.messages.all_balance_updated'));
    } else {
      showError(message);
    }
    setUpdatingBalance(false);
  }, [t]);

  const handleKeywordChange = useCallback((e, { value }) => {
    setSearchKeyword(value.trim());
  }, []);

  const sortChannel = useCallback(
    (key) => {
      if (channels.length === 0) return;
      setLoading(true);
      const sortedChannels = [...channels];
      sortedChannels.sort((a, b) => {
        if (!isNaN(a[key])) return a[key] - b[key];
        return ('' + a[key]).localeCompare(b[key]);
      });
      if (sortedChannels[0].id === channels[0].id) {
        sortedChannels.reverse();
      }
      setChannels(sortedChannels);
      setLoading(false);
    },
    [channels]
  );

  return {
    // State
    channels,
    loading,
    activePage,
    searchKeyword,
    searching,
    updatingBalance,
    // Setters (for UI-only state managed in component)
    setActivePage,
    setSearchKeyword,
    // Handlers
    loadChannels,
    refresh,
    onPaginationChange,
    manageChannel,
    searchChannels,
    switchTestModel,
    testChannel,
    testChannels,
    deleteAllDisabledChannels,
    updateChannelBalance,
    updateAllChannelsBalance,
    handleKeywordChange,
    sortChannel,
  };
};
