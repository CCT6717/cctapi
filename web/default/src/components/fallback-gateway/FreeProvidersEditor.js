import React from 'react';
import { Checkbox, Form, Icon, Input, Label, Message, Table } from 'semantic-ui-react';
import { buildFreeProviderRows } from './freePoolUtils';

const PROVIDER_DISPLAY = {
  openrouter: { title: 'OpenRouter', color: 'purple', icon: 'cloud' },
  groq: { title: 'Groq', color: 'orange', icon: 'lightning' },
  google: { title: 'Google AI', color: 'blue', icon: 'google' },
  kilo: { title: 'Kilo', color: 'teal', icon: 'bolt' },
  pollinations: { title: 'Pollinations', color: 'green', icon: 'leaf' },
  ovh: { title: 'OVH Cloud', color: 'blue', icon: 'server' },
  nvidia: { title: 'NVIDIA', color: 'green', icon: 'microchip' },
  cohere: { title: 'Cohere', color: 'teal', icon: 'comment alternate' },
  huggingface: { title: 'Hugging Face', color: 'yellow', icon: 'smile' },
  llm7: { title: 'LLM7', color: 'violet', icon: 'bolt' },
  opencode: { title: 'OpenCode', color: 'grey', icon: 'code' },
  aihorde: { title: 'AI Horde', color: 'olive', icon: 'users' },
  routeway: { title: 'Routeway', color: 'blue', icon: 'road' },
  bazaarlink: { title: 'Bazaarlink', color: 'pink', icon: 'linkify' },
  ainative: { title: 'AINative', color: 'purple', icon: 'brain' },
  agnes: { title: 'Agnes', color: 'red', icon: 'magic' },
  reka: { title: 'Reka', color: 'orange', icon: 'star' },
  siliconflow: { title: 'SiliconFlow', color: 'violet', icon: 'microchip' },
  zhipu: { title: 'Zhipu AI', color: 'red', icon: 'brain' },
  mistral: { title: 'Mistral', color: 'yellow', icon: 'wind' },
  togetherai: { title: 'Together AI', color: 'pink', icon: 'users' },
  novita: { title: 'Novita', color: 'olive', icon: 'rocket' },
  cloudflare: { title: 'Cloudflare', color: 'orange', icon: 'shield' },
  cerebras: { title: 'Cerebras', color: 'blue', icon: 'microchip' },
  sambanova: { title: 'SambaNova', color: 'purple', icon: 'server' },
  github: { title: 'GitHub Models', color: 'grey', icon: 'github' },
  chutes: { title: 'Chutes', color: 'green', icon: 'bolt' },
  fireworks: { title: 'Fireworks', color: 'red', icon: 'fire' },
  nebius: { title: 'Nebius', color: 'teal', icon: 'cloud' },
  lambdalabs: { title: 'Lambda Labs', color: 'violet', icon: 'lambda' },
};

const LIMIT_FIELDS = ['rpm_limit', 'rpd_limit', 'tpm_limit', 'tpd_limit'];

const formatNumber = (value) => {
  const n = Number(value);
  if (!Number.isFinite(n)) return '-';
  return new Intl.NumberFormat('zh-CN').format(n);
};

const formatLimit = (value) => {
  const n = Number(value);
  if (!Number.isFinite(n)) return '-';
  return n === 0 ? 'unlimited' : formatNumber(n);
};

const renderCapabilityLabels = (provider) => {
  const labels = [];
  if (provider.keyless) labels.push({ text: 'keyless', color: 'green' });
  if (provider.requires_key && !provider.keyless) labels.push({ text: 'key required', color: 'grey' });
  if (provider.supports_stream) labels.push({ text: 'stream', color: 'blue' });
  if (provider.supports_tools) labels.push({ text: 'tools', color: 'teal' });
  if (provider.supports_json) labels.push({ text: 'json', color: 'violet' });
  if (provider.supports_vision) labels.push({ text: 'vision', color: 'purple' });
  if (labels.length === 0) labels.push({ text: 'metadata pending', color: 'grey' });
  return labels.map((label) => (
    <Label key={label.text} basic color={label.color} size='mini'>
      {label.text}
    </Label>
  ));
};

const renderQuirkLabels = (quirks) => {
  if (!quirks) {
    return <Label basic size='mini'>standard</Label>;
  }
  const labels = [];
  if (quirks.force_parallel_tool_calls === false) labels.push('parallel_tools=false');
  if (quirks.default_user_agent) labels.push('user-agent');
  if (quirks.disable_stream) labels.push('no stream');
  if (quirks.max_output_tokens) labels.push(`max ${quirks.max_output_tokens}`);
  if (quirks.drop_stop) labels.push('drop stop');
  if (labels.length === 0) labels.push('standard');
  return labels.map((label) => (
    <Label key={label} basic color={label === 'standard' ? undefined : 'violet'} size='mini'>
      {label}
    </Label>
  ));
};

const FreeProvidersEditor = ({ freeProviders, freeProviderCatalog, onChange }) => {
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
    const keys = String(value || '')
      .split(/\r?\n/)
      .map((key) => key.trim())
      .filter((key) => key && !key.includes('*'));
    updateProvider(providerKey, 'keys', keys);
  };

  const validateLimits = (limits) => {
    if (!limits) return true;
    return LIMIT_FIELDS.every((field) => {
      const value = limits[field];
      return value === undefined
        || value === ''
        || value === null
        || (Number.isFinite(Number(value)) && Number(value) >= 0);
    });
  };

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
          {providerRows.map((provider) => {
            const key = provider.name;
            const display = PROVIDER_DISPLAY[key] || { title: key, color: 'grey', icon: 'key' };
            const limits = provider.limits_override || {};
            const keyCount = provider.key_count || 0;
            const invalidLimits = !validateLimits(limits);
            const stagedKeys = providerConfig[key]?.keys || [];

            return (
              <Table.Row key={key} warning={invalidLimits}>
                <Table.Cell collapsing>
                  <Checkbox
                    toggle
                    checked={!!provider.enabled}
                    onChange={(_, { checked }) => updateProvider(key, 'enabled', checked)}
                  />
                </Table.Cell>
                <Table.Cell>
                  <strong>{display.title}</strong>
                  <div style={{ marginTop: 4 }}>
                    <Label basic color={display.color} size='small'>
                      <Icon name={display.icon} /> {key}
                    </Label>
                    {provider.configured ? (
                      <Label basic color='green' size='small'>configured</Label>
                    ) : (
                      <Label basic size='small'>catalog</Label>
                    )}
                  </div>
                  {provider.model_fetch_mode && (
                    <div style={{ marginTop: 4 }}>
                      <Label basic size='mini'>{provider.model_fetch_mode}</Label>
                    </div>
                  )}
                </Table.Cell>
                <Table.Cell>{keyCount}</Table.Cell>
                <Table.Cell>
                  {renderCapabilityLabels(provider)}
                  <div style={{ marginTop: 4 }}>{renderQuirkLabels(provider.quirks)}</div>
                </Table.Cell>
                <Table.Cell>
                  <div>RPM {formatLimit(provider.rpm_limit)}</div>
                  <div>RPD {formatLimit(provider.rpd_limit)}</div>
                  <div>TPM {formatLimit(provider.tpm_limit)}</div>
                  <div>TPD {formatLimit(provider.tpd_limit)}</div>
                </Table.Cell>
                <Table.Cell>
                  <Form size='small'>
                    <Form.Group widths='equal'>
                      {LIMIT_FIELDS.map((field) => (
                        <Form.Field key={field}>
                          <label>{field.replace('_limit', '').toUpperCase()}</label>
                          <Input
                            type='number'
                            size='mini'
                            placeholder='default'
                            value={limits[field] === undefined ? '' : limits[field]}
                            onChange={(_, { value }) => {
                              const parsed = value === '' ? undefined : parseInt(value, 10);
                              updateLimit(key, field, Number.isFinite(parsed) ? parsed : undefined);
                            }}
                          />
                        </Form.Field>
                      ))}
                    </Form.Group>
                    {invalidLimits && (
                      <Message error size='mini'>
                        <Icon name='warning sign' />
                        Limits cannot be negative.
                      </Message>
                    )}
                  </Form>
                </Table.Cell>
                <Table.Cell>
                  <Form size='small'>
                    <Form.TextArea
                      rows={2}
                      placeholder='Paste new keys, one per line'
                      value={stagedKeys.join('\n')}
                      onChange={(_, { value }) => updateKeys(key, value)}
                    />
                  </Form>
                </Table.Cell>
                <Table.Cell>
                  {provider.enabled ? (
                    <Label color='green' basic>enabled ({keyCount})</Label>
                  ) : (
                    <Label color='grey' basic>disabled</Label>
                  )}
                </Table.Cell>
              </Table.Row>
            );
          })}
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
