import React from 'react';
import { Icon, Message, Table } from 'semantic-ui-react';
import FreeProviderRow from './FreeProviderRow';
import {
  buildClearKeysProviderConfig,
  buildFreeProviderRows,
  buildReplaceKeysProviderConfig,
} from './freePoolUtils';

const FreeProvidersEditor = ({
  freeProviders,
  freeProviderCatalog,
  usageByProviderKey = {},
  onChange,
}) => {
  const providerConfig = freeProviders && typeof freeProviders === 'object' ? freeProviders : {};
  const providerRows = buildFreeProviderRows(providerConfig, freeProviderCatalog);

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

  const usageRowsForProvider = (providerKey) => Object.keys(usageByProviderKey)
    .filter((usageKey) => usageKey.startsWith(`${providerKey}:`))
    .flatMap((usageKey) => usageByProviderKey[usageKey] || []);

  return (
    <div>
      <Table compact celled striped>
        <Table.Header>
          <Table.Row>
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
          {providerRows.map((provider) => (
            <FreeProviderRow
              key={provider.name}
              provider={provider}
              providerConfig={providerConfig}
              onUpdateProvider={updateProvider}
              onUpdateLimit={updateLimit}
              onUpdateKeys={updateKeys}
              onClearKeys={clearKeys}
              usageRows={usageRowsForProvider(provider.name)}
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
