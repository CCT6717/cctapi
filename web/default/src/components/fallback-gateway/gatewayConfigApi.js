import { API } from '../../helpers';

export const getGatewayConfig = () =>
  API.get('/api/fallback/gateway/config');

export const saveGatewayConfig = (config) =>
  API.put('/api/fallback/gateway/config', config);

export const getManualConfig = () =>
  API.get('/api/fallback/manual-config');

export const saveManualConfig = (config) =>
  API.put('/api/fallback/manual-config', config);

export const reloadConfig = () =>
  API.post('/api/fallback/config/reload');

export const syncFreePool = () =>
  API.post('/api/fallback/free-pool/sync');

export const cleanupDryRun = () =>
  API.post('/api/fallback/free-pool/cleanup/dry-run');

export const getRuntimeStatus = () =>
  API.get('/api/fallback/deployments/runtime-status');
