import { API } from '../helpers';

export const getChannels = (page, scope = 'manual') =>
  API.get(`/api/channel/?p=${page}&scope=${scope}`);

export const deleteChannel = (id) =>
  API.delete(`/api/channel/${id}/`);

export const updateChannel = (data) =>
  API.put('/api/channel/', data);

export const searchChannels = (keyword, scope = 'manual') =>
  API.get(`/api/channel/search?keyword=${keyword}&scope=${scope}`);

export const testChannel = (id, model) =>
  API.get(`/api/channel/test/${id}?model=${model}`);

export const testChannels = (scope) =>
  API.get(`/api/channel/test?scope=${scope}`);

export const deleteDisabledChannels = () =>
  API.delete('/api/channel/disabled');

export const updateChannelBalance = (id) =>
  API.get(`/api/channel/update_balance/${id}/`);

export const updateAllChannelsBalance = () =>
  API.get('/api/channel/update_balance');
