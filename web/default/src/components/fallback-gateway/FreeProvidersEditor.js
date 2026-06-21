import React, { useState } from 'react';
import { Button, Checkbox, Form, Icon, Input, Message } from 'semantic-ui-react';

const PROVIDER_META = {
  openrouter: { title: 'OpenRouter', icon: 'cloud' },
  groq:       { title: 'Groq',       icon: 'lightning' },
};
const LIMIT_FIELDS = [
  { key: 'rpm_limit', label: '每分钟请求数' },
  { key: 'rpd_limit', label: '每日请求数' },
  { key: 'tpm_limit', label: '每分钟 Token 数' },
  { key: 'tpd_limit', label: '每日 Token 数' },
];

const FreeProvidersEditor = ({ freeProviders, onChange }) => {
  const [newKeys, setNewKeys] = useState({});
  const [expanded, setExpanded] = useState({});

  if (!freeProviders || typeof freeProviders !== 'object') return <Message warning>免费供应商数据为空或格式错误</Message>;
  const providerKeys = Object.keys(freeProviders);
  if (providerKeys.length === 0) return <Message info>暂无免费供应商配置。</Message>;

  const updateProvider = (key, field, value) => {
    onChange({ ...freeProviders, [key]: { ...freeProviders[key], [field]: value } });
  };
  const updateLimit = (pk, lk, value) => {
    const p = freeProviders[pk] || {};
    updateProvider(pk, 'limits_override', { ...(p.limits_override || {}), [lk]: value });
  };
  const validateLimits = (limits) => {
    if (!limits) return true;
    return LIMIT_FIELDS.every(({ key }) => {
      const v = limits[key];
      return v === undefined || v === '' || v === null || (Number.isFinite(Number(v)) && Number(v) >= 0);
    });
  };

  return (
    <div>
      {providerKeys.map((key) => {
        const provider = freeProviders[key];
        const meta = PROVIDER_META[key] || { title: key, icon: 'key' };
        const limits = provider.limits_override || {};
        const keyCount = Array.isArray(provider.keys) ? provider.keys.length : (provider.key_count || 0);
        const hasNewKey = !!(newKeys[key] && newKeys[key].trim());
        const isOpen = !!expanded[key];

        const keyInfoLines = [];
        if (provider.key_hash) keyInfoLines.push(`密钥哈希: ${provider.key_hash}`);
        if (provider.key_masked) keyInfoLines.push(`脱敏密钥: ${provider.key_masked}`);

        return (
          <div key={key} className='gateway-provider-section'>
            <div className='gateway-provider-header' onClick={() => setExpanded((p) => ({ ...p, [key]: !p[key] }))}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <Icon name={isOpen ? 'chevron down' : 'chevron right'} style={{ fontSize: 12 }} />
                <Icon name={meta.icon} />
                <strong>{meta.title}</strong>
                <span className={`gateway-badge ${provider.enabled ? 'enabled' : 'disabled'}`}>
                  {provider.enabled ? '已启用' : '已停用'}
                </span>
                <span className='gateway-muted' style={{ fontSize: 12 }}>{keyCount} 个密钥</span>
              </div>
              <div onClick={(e) => e.stopPropagation()}>
                <Checkbox toggle checked={!!provider.enabled} onChange={(_, { checked }) => updateProvider(key, 'enabled', checked)} />
              </div>
            </div>

            {isOpen && (
              <div className='gateway-provider-body'>
                {keyInfoLines.length > 0 && (
                  <div className='gateway-provider-key-info'>
                    {keyInfoLines.map((line, i) => <div key={i}>{line}</div>)}
                  </div>
                )}

                <div style={{ marginTop: 10 }}>
                  <label style={{ fontSize: 12, color: '#6b7280', display: 'block', marginBottom: 4 }}>添加新密钥</label>
                  <Input
                    type='password' size='small' placeholder='输入新密钥（留空不保存）'
                    value={newKeys[key] || ''}
                    onChange={(_, { value }) => setNewKeys((p) => ({ ...p, [key]: value }))}
                    action={hasNewKey ? (
                      <Button basic color='green' size='small' onClick={() => {
                        const trimmed = (newKeys[key] || '').trim();
                        if (!trimmed) return;
                        const existing = Array.isArray(provider.keys) ? provider.keys : [];
                        updateProvider(key, 'keys', [...existing, trimmed]);
                        setNewKeys((p) => ({ ...p, [key]: '' }));
                      }}><Icon name='plus' /> 添加密钥</Button>
                    ) : null}
                    style={{ width: '100%' }}
                  />
                </div>

                <div style={{ marginTop: 14 }}>
                  <label style={{ fontSize: 12, fontWeight: 600, display: 'block', marginBottom: 6 }}>限额覆盖</label>
                  <Form>
                    <Form.Group>
                      {LIMIT_FIELDS.map(({ key: fk, label }) => (
                        <Form.Field key={fk} width={4}>
                          <label style={{ fontSize: 11 }}>{label}</label>
                          <Input type='number' size='small' placeholder='默认'
                            value={limits[fk] === undefined ? '' : limits[fk]}
                            onChange={(_, { value }) => {
                              const n = value === '' ? undefined : parseInt(value, 10);
                              updateLimit(key, fk, Number.isFinite(n) ? n : undefined);
                            }} />
                        </Form.Field>
                      ))}
                    </Form.Group>
                  </Form>
                  <div style={{ fontSize: 11, color: '#9ca3af', marginTop: 4 }}>
                    空值表示使用系统默认限额，0 表示不限制，大于 0 表示覆盖默认限额。
                  </div>
                  {!validateLimits(limits) && (
                    <Message error size='small' style={{ marginTop: 6 }}>
                      <Icon name='warning sign' /> 限额值不能为负数
                    </Message>
                  )}
                </div>
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
};

export default FreeProvidersEditor;
