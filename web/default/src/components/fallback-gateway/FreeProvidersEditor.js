import React, { useEffect, useMemo, useState } from 'react';
import { Button, Icon, Input, Label, Message, Table } from 'semantic-ui-react';
import FreeProviderRow from './FreeProviderRow';
import {
  buildBulkEnabledProviderConfig,
  buildClearKeysProviderConfig,
  filterFreeProviderRows,
  buildFreeProviderRows,
  buildReplaceKeysProviderConfig,
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
    return <Message info>No free providers are available in the current catalog.</Message>;
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
      `Staged ${selectedRows.length} provider${selectedRows.length === 1 ? '' : 's'} as ${enabled ? 'enabled' : 'disabled'}. Save config to apply.`
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
            aria-label='Search free providers'
            icon='search'
            placeholder='Search providers, capability, or model mode'
            value={searchText}
            onChange={(_, { value }) => setSearchText(value)}
          />
          <label>
            <span>Status</span>
            <select
              aria-label='Filter free providers by status'
              value={statusFilter}
              onChange={(event) => setStatusFilter(event.target.value)}
            >
              <option value='all'>All statuses</option>
              <option value='enabled'>Enabled</option>
              <option value='disabled'>Disabled</option>
              <option value='ready'>Ready</option>
              <option value='needs_key'>Needs key</option>
              <option value='configured'>Configured</option>
              <option value='catalog'>Catalog only</option>
            </select>
          </label>
          <label>
            <span>Capability</span>
            <select
              aria-label='Filter free providers by capability'
              value={capabilityFilter}
              onChange={(event) => setCapabilityFilter(event.target.value)}
            >
              <option value='all'>All capabilities</option>
              <option value='keyless'>Keyless</option>
              <option value='stream'>Stream</option>
              <option value='tools'>Tools</option>
              <option value='json'>JSON</option>
              <option value='vision'>Vision</option>
            </select>
          </label>
          <span className='free-provider-count'>
            {visibleProviderRows.length} / {providerRows.length} providers
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
            Select visible
          </Button>
          <Button
            type='button'
            basic
            size='mini'
            disabled={selectedProviders.length === 0}
            onClick={clearSelectedProviders}
          >
            Clear selection
          </Button>
          <Button
            type='button'
            basic
            size='mini'
            color='green'
            disabled={selectedProviders.length === 0}
            onClick={() => applyBulkEnabled(true)}
          >
            Enable selected
          </Button>
          <Button
            type='button'
            basic
            size='mini'
            color='orange'
            disabled={selectedProviders.length === 0}
            onClick={() => applyBulkEnabled(false)}
          >
            Disable selected
          </Button>
          {selectedProviders.length > 0 && (
            <Label basic size='mini'>
              {selectedProviders.length} selected
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
            <Table.HeaderCell>Select</Table.HeaderCell>
            <Table.HeaderCell>Enabled</Table.HeaderCell>
            <Table.HeaderCell>Provider</Table.HeaderCell>
            <Table.HeaderCell>Keys</Table.HeaderCell>
            <Table.HeaderCell>Capabilities</Table.HeaderCell>
            <Table.HeaderCell>Effective Limits</Table.HeaderCell>
            <Table.HeaderCell>Override Limits</Table.HeaderCell>
            <Table.HeaderCell>Replace Keys</Table.HeaderCell>
            <Table.HeaderCell>Status</Table.HeaderCell>
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {visibleProviderRows.length === 0 ? (
            <Table.Row>
              <Table.Cell colSpan='9' textAlign='center'>
                No providers match the current filters.
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
        Existing keys are never shown. Empty key input keeps stored keys; pasted keys replace them on save.
      </Message>
    </div>
  );
};

export default FreeProvidersEditor;
