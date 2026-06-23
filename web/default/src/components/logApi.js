import { API } from '../helpers';

export const getSelfLogStat = (params) =>
  API.get(`/api/log/self/stat?${params}`);

export const getLogStat = (params) =>
  API.get(`/api/log/stat?${params}`);

export const getLogs = (page, params) =>
  API.get(`/api/log/?p=${page}&${params}`);

export const getSelfLogs = (page, params) =>
  API.get(`/api/log/self/?p=${page}&${params}`);

export const searchSelfLogs = (keyword) =>
  API.get(`/api/log/self/search?keyword=${keyword}`);
