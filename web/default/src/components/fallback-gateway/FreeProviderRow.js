import React from 'react';
import { Checkbox, Form, Icon, Input, Label, Message, Table } from 'semantic-ui-react';
import {
  LIMIT_FIELDS,
  PROVIDER_DISPLAY,
  formatLimit,
  validateLimits,
} from './freeProviderDisplay';

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

const FreeProviderRow = ({
  provider,
  providerConfig,
  onUpdateProvider,
  onUpdateLimit,
  onUpdateKeys,
}) => {
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
          onChange={(_, { checked }) => onUpdateProvider(key, 'enabled', checked)}
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
                    onUpdateLimit(key, field, Number.isFinite(parsed) ? parsed : undefined);
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
            onChange={(_, { value }) => onUpdateKeys(key, value)}
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
};

export default FreeProviderRow;
