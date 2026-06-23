import { useCallback, useEffect, useState } from 'react';
import { isAdmin, showError, timestamp2string } from '../../helpers';
import * as logApi from '../logApi';
import { ITEMS_PER_PAGE } from '../../constants';

const getLogOptions = (t) => [
  { key: '0', text: t('log.type.all'), value: 0 },
  { key: '1', text: t('log.type.topup'), value: 1 },
  { key: '2', text: t('log.type.usage'), value: 2 },
  { key: '3', text: t('log.type.admin'), value: 3 },
  { key: '4', text: t('log.type.system'), value: 4 },
  { key: '5', text: t('log.type.test'), value: 5 },
];

export const useLogsTable = (t) => {
  const [logs, setLogs] = useState([]);
  const [showStat, setShowStat] = useState(false);
  const [loading, setLoading] = useState(true);
  const [activePage, setActivePage] = useState(1);
  const [searchKeyword, setSearchKeyword] = useState('');
  const [searching, setSearching] = useState(false);
  const [logType, setLogType] = useState(0);
  const isAdminUser = isAdmin();

  const now = new Date();
  const [inputs, setInputs] = useState({
    username: '',
    token_name: '',
    model_name: '',
    start_timestamp: timestamp2string(0),
    end_timestamp: timestamp2string(now.getTime() / 1000 + 3600),
    channel: '',
  });
  const {
    username,
    token_name,
    model_name,
    start_timestamp,
    end_timestamp,
    channel,
  } = inputs;

  const [stat, setStat] = useState({ quota: 0, token: 0 });

  const LOG_OPTIONS = getLogOptions(t);

  const handleInputChange = useCallback(
    (e, { name, value }) => {
      setInputs((prev) => ({ ...prev, [name]: value }));
    },
    []
  );

  const getLogSelfStat = useCallback(async () => {
    const localStartTimestamp = Date.parse(start_timestamp) / 1000;
    const localEndTimestamp = Date.parse(end_timestamp) / 1000;
    const params = `type=${logType}&token_name=${token_name}&model_name=${model_name}&start_timestamp=${localStartTimestamp}&end_timestamp=${localEndTimestamp}`;
    const res = await logApi.getSelfLogStat(params);
    const { success, message, data } = res.data;
    if (success) {
      setStat(data);
    } else {
      showError(message);
    }
  }, [logType, token_name, model_name, start_timestamp, end_timestamp]);

  const getLogStat = useCallback(async () => {
    const localStartTimestamp = Date.parse(start_timestamp) / 1000;
    const localEndTimestamp = Date.parse(end_timestamp) / 1000;
    const params = `type=${logType}&username=${username}&token_name=${token_name}&model_name=${model_name}&start_timestamp=${localStartTimestamp}&end_timestamp=${localEndTimestamp}&channel=${channel}`;
    const res = await logApi.getLogStat(params);
    const { success, message, data } = res.data;
    if (success) {
      setStat(data);
    } else {
      showError(message);
    }
  }, [logType, username, token_name, model_name, start_timestamp, end_timestamp, channel]);

  const handleEyeClick = useCallback(async () => {
    if (!showStat) {
      if (isAdminUser) {
        await getLogStat();
      } else {
        await getLogSelfStat();
      }
    }
    setShowStat((prev) => !prev);
  }, [showStat, isAdminUser, getLogStat, getLogSelfStat]);

  const loadLogs = useCallback(
    async (startIdx) => {
      const localStartTimestamp = Date.parse(start_timestamp) / 1000;
      const localEndTimestamp = Date.parse(end_timestamp) / 1000;
      const params = `type=${logType}&username=${username}&token_name=${token_name}&model_name=${model_name}&start_timestamp=${localStartTimestamp}&end_timestamp=${localEndTimestamp}&channel=${channel}`;
      const selfParams = `type=${logType}&token_name=${token_name}&model_name=${model_name}&start_timestamp=${localStartTimestamp}&end_timestamp=${localEndTimestamp}`;
      const res = isAdminUser
        ? await logApi.getLogs(startIdx, params)
        : await logApi.getSelfLogs(startIdx, selfParams);
      const { success, message, data } = res.data;
      if (success) {
        if (startIdx === 0) {
          setLogs(data);
        } else {
          setLogs((prev) => {
            const newLogs = [...prev];
            newLogs.splice(
              startIdx * ITEMS_PER_PAGE,
              data.length,
              ...data
            );
            return newLogs;
          });
        }
      } else {
        showError(message);
      }
      setLoading(false);
    },
    [
      isAdminUser,
      logType,
      username,
      token_name,
      model_name,
      start_timestamp,
      end_timestamp,
      channel,
    ]
  );

  const onPaginationChange = useCallback(
    (e, { activePage: page }) => {
      (async () => {
        if (page === Math.ceil(logs.length / ITEMS_PER_PAGE) + 1) {
          await loadLogs(page - 1);
        }
        setActivePage(page);
      })();
    },
    [logs.length, loadLogs]
  );

  const refresh = useCallback(async () => {
    setLoading(true);
    await loadLogs(activePage - 1);
  }, [activePage, loadLogs]);

  useEffect(() => {
    refresh();
  }, [logType]);

  const searchLogs = useCallback(async () => {
    if (searchKeyword === '') {
      await loadLogs(0);
      setActivePage(1);
      return;
    }
    setSearching(true);
    const res = await logApi.searchSelfLogs(searchKeyword);
    const { success, message, data } = res.data;
    if (success) {
      setLogs(data);
      setActivePage(1);
    } else {
      showError(message);
    }
    setSearching(false);
  }, [searchKeyword, loadLogs]);

  const handleKeywordChange = useCallback((e, { value }) => {
    setSearchKeyword(value.trim());
  }, []);

  const sortLog = useCallback(
    (key) => {
      if (logs.length === 0) return;
      setLoading(true);
      const sortedLogs = [...logs];
      if (typeof sortedLogs[0][key] === 'string') {
        sortedLogs.sort((a, b) => ('' + a[key]).localeCompare(b[key]));
      } else {
        sortedLogs.sort((a, b) => {
          if (a[key] === b[key]) return 0;
          if (a[key] > b[key]) return -1;
          if (a[key] < b[key]) return 1;
        });
      }
      if (sortedLogs[0].id === logs[0].id) {
        sortedLogs.reverse();
      }
      setLogs(sortedLogs);
      setLoading(false);
    },
    [logs]
  );

  return {
    // State
    logs,
    loading,
    activePage,
    searchKeyword,
    searching,
    logType,
    showStat,
    stat,
    isAdminUser,
    inputs,
    LOG_OPTIONS,
    // Destructured inputs for JSX convenience
    username,
    token_name,
    model_name,
    start_timestamp,
    end_timestamp,
    channel,
    // Setters
    setLogType,
    setSearchKeyword,
    // Handlers
    loadLogs,
    refresh,
    onPaginationChange,
    searchLogs,
    handleInputChange,
    handleKeywordChange,
    handleEyeClick,
    sortLog,
  };
};
