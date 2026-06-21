import React from 'react';
import { Checkbox, Dropdown, Icon, Input, Label, Message, Table } from 'semantic-ui-react';

const POOL_OPTIONS = [
  { key: 'paid_high', value: 'paid_high', text: 'paid_high' },
  { key: 'cheap', value: 'cheap', text: 'cheap' },
  { key: 'local', value: 'local', text: 'local' },
  { key: 'free', value: 'free', text: 'free' },
];

const QUALITY_OPTIONS = [
  { key: 'high', value: 'high', text: 'high' },
  { key: 'medium', value: 'medium', text: 'medium' },
  { key: 'low', value: 'low', text: 'low' },
];

const COST_OPTIONS = [
  { key: 'paid', value: 'paid', text: 'paid' },
  { key: 'cheap', value: 'cheap', text: 'cheap' },
  { key: 'free', value: 'free', text: 'free' },
];

const QUOTA_MODE_OPTIONS = [
  { key: 'normal', value: 'normal', text: 'normal' },
  { key: 'free', value: 'free', text: 'free' },
];

const isAutoDeployment = (id) => String(id || '').startsWith('free:');

const NumberInput = ({ value, onChange, disabled, width }) => (
  <Input
    type='number'
    size='mini'
    value={value === undefined || value === null ? '' : value}
    onChange={(_, { value: v }) => {
      const parsed = v === '' ? 0 : parseInt(v, 10);
      onChange(Number.isFinite(parsed) ? parsed : 0);
    }}
    disabled={disabled}
    style={{ width: width || 80 }}
  />
);

const DeploymentsEditor = ({ deployments, onChange }) => {
  if (!deployments || typeof deployments !== 'object') {
    return <Message warning>部署数据为空或格式错误</Message>;
  }

  const depKeys = Object.keys(deployments);

  const updateDep = (key, field, value) => {
    const updated = {
      ...deployments,
      [key]: {
        ...deployments[key],
        [field]: value,
      },
    };
    onChange(updated);
  };

  const updateCap = (key, capField, value) => {
    // Map short names to backend field names: vision→supports_vision, stream→supports_stream, etc.
    const fieldMap = { vision: 'supports_vision', stream: 'supports_stream', tools: 'supports_tools', json: 'supports_json' };
    const backendField = fieldMap[capField] || capField;
    updateDep(key, backendField, value);
  };

  if (depKeys.length === 0) {
    return <Message info>暂无部署配置。</Message>;
  }

  return (
    <div style={{ overflowX: 'auto' }}>
      <Table compact celled striped size='small'>
        <Table.Header>
          <Table.Row>
            <Table.HeaderCell>ID</Table.HeaderCell>
            <Table.HeaderCell>启用</Table.HeaderCell>
            <Table.HeaderCell>Pool</Table.HeaderCell>
            <Table.HeaderCell>Real Model</Table.HeaderCell>
            <Table.HeaderCell>Channel ID</Table.HeaderCell>
            <Table.HeaderCell>Quality</Table.HeaderCell>
            <Table.HeaderCell>Cost</Table.HeaderCell>
            <Table.HeaderCell>Quota</Table.HeaderCell>
            <Table.HeaderCell>Vision</Table.HeaderCell>
            <Table.HeaderCell>Stream</Table.HeaderCell>
            <Table.HeaderCell>Tools</Table.HeaderCell>
            <Table.HeaderCell>JSON</Table.HeaderCell>
            <Table.HeaderCell>Context</Table.HeaderCell>
            <Table.HeaderCell>RPM</Table.HeaderCell>
            <Table.HeaderCell>RPD</Table.HeaderCell>
            <Table.HeaderCell>TPM</Table.HeaderCell>
            <Table.HeaderCell>TPD</Table.HeaderCell>
            <Table.HeaderCell>Priority</Table.HeaderCell>
            <Table.HeaderCell>Weight</Table.HeaderCell>
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {depKeys.map((key) => {
            const dep = deployments[key];
            const auto = isAutoDeployment(key);

            return (
              <Table.Row key={key}>
                <Table.Cell>
                  <strong>{key}</strong>
                  {auto && (
                    <div>
                      <Label basic size='mini' color='blue'>
                        <Icon name='lightning' /> Auto
                      </Label>
                    </div>
                  )}
                </Table.Cell>
                <Table.Cell>
                  <Checkbox
                    checked={!!dep.enabled}
                    onChange={(_, { checked }) => updateDep(key, 'enabled', checked)}
                    disabled={auto}
                  />
                </Table.Cell>
                <Table.Cell>
                  <Dropdown
                    selection
                    search
                    compact
                    options={POOL_OPTIONS}
                    value={dep.pool || ''}
                    onChange={(_, { value }) => updateDep(key, 'pool', value)}
                    disabled={auto}
                    style={{ minWidth: 100 }}
                  />
                </Table.Cell>
                <Table.Cell>
                  {auto ? (
                    <span style={{ color: '#868b94' }}>{dep.real_model || '-'}</span>
                  ) : (
                    <Input
                      size='mini'
                      value={dep.real_model || ''}
                      onChange={(_, { value }) => updateDep(key, 'real_model', value)}
                      style={{ width: 140 }}
                    />
                  )}
                </Table.Cell>
                <Table.Cell>
                  {auto ? (
                    <span style={{ color: '#868b94' }}>{dep.channel_id || '-'}</span>
                  ) : (
                    <NumberInput
                      value={dep.channel_id}
                      onChange={(v) => updateDep(key, 'channel_id', v)}
                      disabled={auto}
                    />
                  )}
                </Table.Cell>
                <Table.Cell>
                  <Dropdown
                    selection
                    compact
                    options={QUALITY_OPTIONS}
                    value={dep.quality_tier || 'medium'}
                    onChange={(_, { value }) => updateDep(key, 'quality_tier', value)}
                    disabled={auto}
                    style={{ minWidth: 80 }}
                  />
                </Table.Cell>
                <Table.Cell>
                  <Dropdown
                    selection
                    compact
                    options={COST_OPTIONS}
                    value={dep.cost_tier || 'paid'}
                    onChange={(_, { value }) => updateDep(key, 'cost_tier', value)}
                    disabled={auto}
                    style={{ minWidth: 80 }}
                  />
                </Table.Cell>
                <Table.Cell>
                  <Dropdown
                    selection
                    compact
                    options={QUOTA_MODE_OPTIONS}
                    value={dep.quota_mode || 'normal'}
                    onChange={(_, { value }) => updateDep(key, 'quota_mode', value)}
                    disabled={auto}
                    style={{ minWidth: 80 }}
                  />
                </Table.Cell>
                <Table.Cell>
                  <Checkbox
                    checked={!!dep.supports_vision}
                    onChange={(_, { checked }) => updateCap(key, 'vision', checked)}
                    disabled={auto}
                  />
                </Table.Cell>
                <Table.Cell>
                  <Checkbox
                    checked={!!dep.supports_stream}
                    onChange={(_, { checked }) => updateCap(key, 'stream', checked)}
                    disabled={auto}
                  />
                </Table.Cell>
                <Table.Cell>
                  <Checkbox
                    checked={!!dep.supports_tools}
                    onChange={(_, { checked }) => updateCap(key, 'tools', checked)}
                    disabled={auto}
                  />
                </Table.Cell>
                <Table.Cell>
                  <Checkbox
                    checked={!!dep.supports_json}
                    onChange={(_, { checked }) => updateCap(key, 'json', checked)}
                    disabled={auto}
                  />
                </Table.Cell>
                <Table.Cell>
                  <NumberInput
                    value={dep.context_length}
                    onChange={(v) => updateDep(key, 'context_length', v)}
                    disabled={auto}
                    width={90}
                  />
                </Table.Cell>
                <Table.Cell>
                  <NumberInput
                    value={dep.rpm_limit}
                    onChange={(v) => updateDep(key, 'rpm_limit', v)}
                    disabled={auto}
                  />
                </Table.Cell>
                <Table.Cell>
                  <NumberInput
                    value={dep.rpd_limit}
                    onChange={(v) => updateDep(key, 'rpd_limit', v)}
                    disabled={auto}
                  />
                </Table.Cell>
                <Table.Cell>
                  <NumberInput
                    value={dep.tpm_limit}
                    onChange={(v) => updateDep(key, 'tpm_limit', v)}
                    disabled={auto}
                  />
                </Table.Cell>
                <Table.Cell>
                  <NumberInput
                    value={dep.tpd_limit}
                    onChange={(v) => updateDep(key, 'tpd_limit', v)}
                    disabled={auto}
                  />
                </Table.Cell>
                <Table.Cell>
                  <NumberInput
                    value={dep.priority}
                    onChange={(v) => updateDep(key, 'priority', v)}
                    disabled={auto}
                  />
                </Table.Cell>
                <Table.Cell>
                  <NumberInput
                    value={dep.weight}
                    onChange={(v) => updateDep(key, 'weight', v)}
                    disabled={auto}
                  />
                </Table.Cell>
              </Table.Row>
            );
          })}
        </Table.Body>
      </Table>
      <Message info style={{ marginTop: 8 }}>
        <Icon name='info circle' />
        以 <code>free:</code> 开头的部署为自动生成，请在"Free Providers"标签页管理。
      </Message>
    </div>
  );
};

export default DeploymentsEditor;
