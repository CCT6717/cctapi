import React, { useEffect, useMemo, useState } from 'react';
import { Button, Icon, Input, Label, Message, Table } from 'semantic-ui-react';
import FreeProviderRow from './FreeProviderRow';
import {
  buildBulkEnabledProviderConfig,
  buildClearKeysProviderConfig,
  buildFreeProviderRows,
  buildReplaceKeysProviderConfig,
  filterFreeProviderRows,
} from './freePoolUtils';

const FreeProvidersEditor = ({
  freeProviders,
  freeProviderCatalog,
  usageByProviderKey = {},
  onChange,
}) => {
  const providerConfig = useMemo(
    () => (freeProviders && typeof freeProviders === 'object' ? freeProviders : {}),
    [freeProviders],
  );
  const [searchText, setSearchText] = useState('');
  const [statusFilter, setStatusFilter] = useState('all');
  const [capabilityFilter, setCapabilityFilter] = useState('all');
  const [selectedProviders, setSelectedProviders] = useState([]);
  const [bulkMessage, setBulkMessage] = useState('');

  const providerRows = useMemo(
    () => buildFreeProviderRows(providerConfig, freeProviderCatalog),
    [providerConfig, freeProviderCatalog],
  );
  const visibleProviderRows = useMemo(
    () => filterFreeProviderRows(providerRows, {
      searchText,
      statusFilter,
      capabilityFilter,
    }),
    [providerRows, searchText, statusFilter, capabilityFilter],
  );
  const visibleProviderNames = useMemo(
    () => visibleProviderRows.map((provider) => provider.name),
    [visibleProviderRows],
  );
  const visibleProviderNameKey = visibleProviderNames.join('|');

  useEffect(() => {
    const visibleNames = new Set(visibleProviderNames);
    setSelectedProviders((previous) => {
      const next = previous.filter((name) => visibleNames.has(name));
      return next.length === previous.length ? previous : next;
    });
  }, [visibleProviderNameKey, visibleProviderNames]);

  if (providerRows.length === 0) {
    return <Message info>当前目录没有可用的免费供应商。</Message>;
  }

  const updateProvider = (key, field, value) => {
    onChange({
      ...providerConfig,
      [key]: {
        ...(providerConfig[key] || {}),
        [field]: value,
      },
    });
  };

  const updateLimit = (providerKey, limitField, value) => {
    const provider = providerConfig[providerKey] || {};
    const limits = { ...(provider.limits_override || {}), [limitField]: value };
    updateProvider(providerKey, 'limits_override', limits);
  };

  const updateKeys = (providerKey, value) => {
    onChange(buildReplaceKeysProviderConfig(providerConfig, providerKey, value));
  };

  const clearKeys = (providerKey) => {
    onChange(buildClearKeysProviderConfig(providerConfig, providerKey));
  };

  const toggleProviderSelection = (providerKey, checked) => {
    setSelectedProviders((previous) => {
      if (checked) {
        return previous.includes(providerKey) ? previous : [...previous, providerKey];
      }
      return previous.filter((name) => name !== providerKey);
    });
  };

  const selectVisibleProviders = () => {
    setSelectedProviders(visibleProviderNames);
  };

  const clearSelectedProviders = () => {
    setSelectedProviders([]);
  };

  const applyBulkEnabled = (enabled) => {
    const selectedNames = new Set(selectedProviders);
    const selectedRows = providerRows.filter((provider) => selectedNames.has(provider.name));
    if (selectedRows.length === 0) return;

    onChange(buildBulkEnabledProviderConfig(providerConfig, selectedRows, enabled));
    setBulkMessage(
      `已暂存 ${selectedRows.length} 个供应商为${enabled ? '已启用' : '已停用'}，点击“保存配置”后生效。`,
    );
  };

  const usageRowsForProvider = (providerKey) => Object.keys(usageByProviderKey)
    .filter((usageKey) => usageKey.startsWith(`${providerKey}:`))
    .flatMap((usageKey) => usageByProviderKey[usageKey] || []);

  return (
    <div>
      <div className='free-provider-ops'>
        <div className='free-provider-filter-row'>
          <Input
            aria-label='搜索免费供应商'
            icon='search'
            placeholder='搜索供应商、能力或模型模式'
            value={searchText}
            onChange={(_, { value }) => setSearchText(value)}
          />
          <label>
            <span>状态</span>
            <select
              aria-label='按状态筛选免费供应商'
              value={statusFilter}
              onChange={(event) => setStatusFilter(event.target.value)}
            >
              <option value='all'>全部状态</option>
              <option value='enabled'>已启用</option>
              <option value='disabled'>已停用</option>
              <option value='ready'>已就绪</option>
              <option value='needs_key'>需要 key</option>
              <option value='configured'>已配置</option>
              <option value='catalog'>仅目录</option>
            </select>
          </label>
          <label>
            <span>能力</span>
            <select
              aria-label='按能力筛选免费供应商'
              value={capabilityFilter}
              onChange={(event) => setCapabilityFilter(event.target.value)}
            >
              <option value='all'>全部能力</option>
              <option value='keyless'>免 key</option>
              <option value='stream'>流式</option>
              <option value='tools'>工具</option>
              <option value='json'>JSON</option>
              <option value='vision'>视觉</option>
            </select>
          </label>
          <span className='free-provider-count'>
            <strong>{visibleProviderRows.length}</strong> / {providerRows.length} 个供应商
          </span>
        </div>

        <div className='free-provider-bulk-row'>
          <Button
            type='button'
            basic
            size='mini'
            disabled={visibleProviderRows.length === 0}
            onClick={selectVisibleProviders}
          >
            选择可见项
          </Button>
          <Button
            type='button'
            basic
            size='mini'
            disabled={selectedProviders.length === 0}
            onClick={clearSelectedProviders}
          >
            清空选择
          </Button>
          <Button
            type='button'
            basic
            size='mini'
            color='green'
            disabled={selectedProviders.length === 0}
            onClick={() => applyBulkEnabled(true)}
          >
            批量启用
          </Button>
          <Button
            type='button'
            basic
            size='mini'
            color='orange'
            disabled={selectedProviders.length === 0}
            onClick={() => applyBulkEnabled(false)}
          >
            批量停用
          </Button>
          {selectedProviders.length > 0 && (
            <Label basic size='mini' className='free-provider-selected-count'>
              已选择 {selectedProviders.length} 项
            </Label>
          )}
        </div>

        {bulkMessage && (
          <Message info size='small' className='free-provider-bulk-message'>
            {bulkMessage}
          </Message>
        )}
      </div>

      <Table compact celled striped>
        <Table.Header>
          <Table.Row>
            <Table.HeaderCell>选择</Table.HeaderCell>
            <Table.HeaderCell>启用</Table.HeaderCell>
            <Table.HeaderCell>供应商</Table.HeaderCell>
            <Table.HeaderCell>密钥数</Table.HeaderCell>
            <Table.HeaderCell>能力</Table.HeaderCell>
            <Table.HeaderCell>当前限额</Table.HeaderCell>
            <Table.HeaderCell>覆盖限额</Table.HeaderCell>
            <Table.HeaderCell>替换密钥</Table.HeaderCell>
            <Table.HeaderCell>状态</Table.HeaderCell>
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {visibleProviderRows.length === 0 ? (
            <Table.Row>
              <Table.Cell colSpan='9' textAlign='center'>
                当前筛选条件下没有匹配的供应商。
              </Table.Cell>
            </Table.Row>
          ) : visibleProviderRows.map((provider) => (
            <FreeProviderRow
              key={provider.name}
              provider={provider}
              providerConfig={providerConfig}
              onUpdateProvider={updateProvider}
              onUpdateLimit={updateLimit}
              onUpdateKeys={updateKeys}
              onClearKeys={clearKeys}
              usageRows={usageRowsForProvider(provider.name)}
              selected={selectedProviders.includes(provider.name)}
              onSelectProvider={toggleProviderSelection}
            />
          ))}
        </Table.Body>
      </Table>

      <Message info size='small'>
        <Icon name='info circle' />
        已保存的密钥不会显示。密钥输入为空会保留已保存密钥；粘贴新密钥会在保存时替换原密钥。
      </Message>
    </div>
  );
};

export default FreeProvidersEditor;
