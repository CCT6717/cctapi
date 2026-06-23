import { API } from '../helpers';

export const getChannels = (page) =>
  API.get(`/api/channel/?p=${page}`);

export const deleteChannel = (id) =>
  API.delete(`/api/channel/${id}/`);

export const updateChannel = (data) =>
  API.put('/api/channel/', data);

export const searchChannels = (keyword) =>
  API.get(`/api/channel/search?keyword=${keyword}`);

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
