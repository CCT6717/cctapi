import React from 'react';
import {
  Button,
  Checkbox,
  Form,
  Icon,
  Input,
  Label,
  Message,
  Table,
} from 'semantic-ui-react';
import {
  LIMIT_FIELDS,
  PROVIDER_DISPLAY,
  formatLimit,
  validateLimits,
} from './freeProviderDisplay';

const renderCapabilityLabels = (provider) => {
  const labels = [];
  if (provider.keyless) labels.push({ text: '免 key', color: 'green' });
  if (provider.requires_key && !provider.keyless) labels.push({ text: '需要 key', color: 'grey' });
  if (provider.supports_stream) labels.push({ text: '流式', color: 'blue' });
  if (provider.supports_tools) labels.push({ text: '工具', color: 'teal' });
  if (provider.supports_json) labels.push({ text: 'JSON', color: 'violet' });
  if (provider.supports_vision) labels.push({ text: '视觉', color: 'purple' });
  if (labels.length === 0) labels.push({ text: '元数据待补充', color: 'grey' });
  return labels.map((label) => (
    <Label key={label.text} basic color={label.color} size='mini'>
      {label.text}
    </Label>
  ));
};

const renderQuirkLabels = (quirks) => {
  if (!quirks) {
    return <Label basic size='mini'>标准</Label>;
  }
  const labels = [];
  if (quirks.force_parallel_tool_calls === false) labels.push('禁用并行工具');
  if (quirks.default_user_agent) labels.push('user-agent');
  if (quirks.disable_stream) labels.push('禁用流式');
  if (quirks.max_output_tokens) labels.push(`最大输出 ${quirks.max_output_tokens}`);
  if (quirks.drop_stop) labels.push('忽略 stop');
  if (labels.length === 0) labels.push('标准');
  return labels.map((label) => (
    <Label key={label} basic color={label === '标准' ? undefined : 'violet'} size='mini'>
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
  onClearKeys,
  usageRows = [],
  selected = false,
  onSelectProvider,
}) => {
  const key = provider.name;
  const display = PROVIDER_DISPLAY[key] || { title: key, color: 'grey', icon: 'key' };
  const limits = provider.limits_override || {};
  const keyCount = provider.key_count || 0;
  const invalidLimits = !validateLimits(limits);
  const stagedKeys = providerConfig[key]?.keys || [];

  return (
    <Table.Row key={key} warning={invalidLimits}>
      {onSelectProvider && (
        <Table.Cell collapsing>
          <Checkbox
            checked={selected}
            aria-label={`选择 ${display.title}`}
            onChange={(_, { checked }) => onSelectProvider(key, checked)}
          />
        </Table.Cell>
      )}
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
            <Label basic color='green' size='small'>已配置</Label>
          ) : (
            <Label basic size='small'>目录</Label>
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
                  placeholder='默认'
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
              限额不能为负数。
            </Message>
          )}
        </Form>
      </Table.Cell>
      <Table.Cell>
        <Form size='small'>
          <Form.TextArea
            rows={2}
            placeholder='粘贴新密钥，每行一个'
            value={stagedKeys.join('\n')}
            onChange={(_, { value }) => onUpdateKeys(key, value)}
          />
          <Button
            type='button'
            basic
            size='mini'
            color='red'
            icon
            labelPosition='left'
            disabled={keyCount === 0 && stagedKeys.length === 0}
            onClick={() => onClearKeys(key)}
          >
            <Icon name='trash' />
            清空已存密钥
          </Button>
          {providerConfig[key]?.clear_keys && (
            <Label basic color='red' size='mini'>保存时将清空密钥</Label>
          )}
        </Form>
      </Table.Cell>
      <Table.Cell>
        {provider.enabled ? (
          <Label color='green' basic>已启用（{keyCount}）</Label>
        ) : (
          <Label color='grey' basic>已停用</Label>
        )}
        {usageRows.length > 0 && (
          <div style={{ marginTop: 6 }}>
            <Label basic size='mini'>
              {usageRows.reduce((sum, row) => sum + Number(row.total_tokens || 0), 0).toLocaleString()} tokens
            </Label>
          </div>
        )}
      </Table.Cell>
    </Table.Row>
  );
};

export default FreeProviderRow;
