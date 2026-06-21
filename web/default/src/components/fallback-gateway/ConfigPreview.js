import React, { useState } from 'react';
import { Button, Icon, Message } from 'semantic-ui-react';

const LEGACY_FIELDS = ['routing_mode', 'fallback_order', 'fixed_deployment'];

const detectLegacyFields = (config) => {
  if (!config) return false;
  const vms = config.virtual_models;
  if (!vms) return false;
  if (typeof vms === 'object' && !Array.isArray(vms)) {
    return Object.values(vms).some((vm) => LEGACY_FIELDS.some((f) => vm[f] !== undefined));
  }
  if (Array.isArray(vms)) {
    return vms.some((vm) => LEGACY_FIELDS.some((f) => vm[f] !== undefined));
  }
  return false;
};

const detectPlainKeys = (config) => {
  if (!config) return false;
  const fps = config.free_providers;
  if (!fps || typeof fps !== 'object') return false;
  return Object.values(fps).some((p) => {
    if (!Array.isArray(p.keys)) return false;
    return p.keys.some((k) => typeof k === 'string' && k.length > 0 && !k.includes('****'));
  });
};

const ConfigPreview = ({ config, onSave, saving }) => {
  const [showJson, setShowJson] = useState(false);

  const hasLegacy = detectLegacyFields(config);
  const hasPlainKeys = detectPlainKeys(config);
  const hasV2 = config && (config.virtual_models !== undefined || config.deployments !== undefined || config.free_providers !== undefined);

  const copyJson = () => {
    try { navigator.clipboard.writeText(JSON.stringify(config, null, 2)); } catch { /* ignore */ }
  };

  return (
    <div>
      <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginBottom: 16 }}>
        <span className={`gateway-badge ${hasLegacy ? 'error' : 'enabled'}`}>
          <Icon name={hasLegacy ? 'warning sign' : 'check'} style={{ marginRight: 4 }} />
          {hasLegacy ? '检测到旧版字段' : '未发现旧版字段'}
        </span>
        <span className={`gateway-badge ${hasPlainKeys ? 'error' : 'enabled'}`}>
          <Icon name={hasPlainKeys ? 'warning sign' : 'check'} style={{ marginRight: 4 }} />
          {hasPlainKeys ? '检测到明文密钥' : '未发现明文密钥'}
        </span>
        <span className={`gateway-badge ${hasV2 ? 'enabled' : 'warning'}`}>
          <Icon name={hasV2 ? 'check' : 'exclamation'} style={{ marginRight: 4 }} />
          {hasV2 ? '新版配置有效' : '配置无效'}
        </span>
      </div>

      <div style={{ display: 'flex', gap: 8, marginBottom: 16 }}>
        <Button className='gateway-btn-primary' icon labelPosition='left' onClick={onSave} loading={saving} disabled={saving || hasLegacy}>
          <Icon name='save' /> 保存网关配置
        </Button>
        <Button basic size='small' onClick={() => setShowJson(!showJson)}>
          <Icon name={showJson ? 'eye slash' : 'eye'} /> {showJson ? '隐藏 JSON' : '查看 JSON'}
        </Button>
        {showJson && (
          <Button basic size='small' onClick={copyJson}>
            <Icon name='copy' /> 复制 JSON
          </Button>
        )}
      </div>

      {hasLegacy && (
        <Message error size='small'>
          <Icon name='warning sign' />
          <Message.Content>
            <Message.Header>检测到旧版字段</Message.Header>
            <p>当前配置中存在 routing_mode / fallback_order / fixed_deployment，新版网关编辑器不支持保存这些字段。</p>
          </Message.Content>
        </Message>
      )}

      {!hasV2 && (
        <Message warning size='small'>
          <Icon name='exclamation triangle' /> 当前配置缺少 virtual_models / deployments / free_providers 字段。
        </Message>
      )}

      {showJson && (
        <pre className='gateway-mono' style={{
          background: '#f8fafc', border: '1px solid #e3e8ef', borderRadius: 8,
          padding: 16, lineHeight: 1.6, maxHeight: 600, overflow: 'auto',
          whiteSpace: 'pre-wrap', wordBreak: 'break-all',
        }}>
          {JSON.stringify(config, null, 2)}
        </pre>
      )}
    </div>
  );
};

export default ConfigPreview;
