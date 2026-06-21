import React from 'react';
import { Button, Icon, Message } from 'semantic-ui-react';

const LEGACY_FIELDS = ['routing_mode', 'fallback_order', 'fixed_deployment'];

const detectLegacyFields = (config) => {
  if (!config) return false;
  const vms = config.virtual_models;
  if (!vms) return false;

  if (typeof vms === 'object' && !Array.isArray(vms)) {
    return Object.values(vms).some((vm) =>
      LEGACY_FIELDS.some((f) => vm[f] !== undefined)
    );
  }

  if (Array.isArray(vms)) {
    return vms.some((vm) =>
      LEGACY_FIELDS.some((f) => vm[f] !== undefined)
    );
  }

  return false;
};

const ConfigPreview = ({ config, onSave, saving }) => {
  const hasLegacy = detectLegacyFields(config);
  const hasV2Fields =
    config &&
    (config.virtual_models !== undefined ||
      config.deployments !== undefined ||
      config.free_providers !== undefined);

  return (
    <div>
      <Message info>
        <Message.Header>保存说明</Message.Header>
        <Message.List>
          <Message.Item>将写入新版字段：virtual_models, deployments, free_providers</Message.Item>
          <Message.Item>不会写入 routing_mode / fallback_order / fixed_deployment</Message.Item>
        </Message.List>
      </Message>

      {hasLegacy && (
        <Message error>
          <Icon name='warning sign' />
          <Message.Content>
            <Message.Header>检测到 legacy 字段</Message.Header>
            <p>当前配置中存在 routing_mode / fallback_order / fixed_deployment 字段，
            新版网关编辑器不支持保存这些字段。请移除后再保存。</p>
          </Message.Content>
        </Message>
      )}

      {!hasV2Fields && (
        <Message warning>
          <Icon name='exclamation triangle' />
          <Message.Content>
            当前配置缺少 virtual_models / deployments / free_providers 字段，
            请确认后端已返回新版网关配置。
          </Message.Content>
        </Message>
      )}

      <div style={{ marginTop: 16, marginBottom: 16 }}>
        <Button
          primary
          icon
          labelPosition='left'
          onClick={onSave}
          loading={saving}
          disabled={saving || hasLegacy}
        >
          <Icon name='save' />
          保存配置
        </Button>
      </div>

      <pre
        style={{
          background: '#f8fafc',
          border: '1px solid #e3e8ef',
          borderRadius: 8,
          padding: 16,
          fontSize: 13,
          lineHeight: 1.6,
          maxHeight: 600,
          overflow: 'auto',
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-all',
        }}
      >
        {JSON.stringify(config, null, 2)}
      </pre>
    </div>
  );
};

export default ConfigPreview;
