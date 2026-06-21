import React, { useState } from 'react';
import { Button, Card, Checkbox, Form, Icon, Input, Label, Message } from 'semantic-ui-react';

const PROVIDER_DISPLAY = {
  openrouter: { title: 'OpenRouter', color: 'purple', icon: 'cloud' },
  groq: { title: 'Groq', color: 'orange', icon: 'lightning' },
};

const FreeProvidersEditor = ({ freeProviders, onChange }) => {
  const [newKeys, setNewKeys] = useState({});

  if (!freeProviders || typeof freeProviders !== 'object') {
    return <Message warning>Free Providers 数据为空或格式错误</Message>;
  }

  const providerKeys = Object.keys(freeProviders);

  const updateProvider = (key, field, value) => {
    const updated = {
      ...freeProviders,
      [key]: {
        ...freeProviders[key],
        [field]: value,
      },
    };
    onChange(updated);
  };

  const updateLimit = (providerKey, limitField, value) => {
    const provider = freeProviders[providerKey] || {};
    const limits = { ...(provider.limits_override || {}), [limitField]: value };
    updateProvider(providerKey, 'limits_override', limits);
  };

  const validateLimits = (limits) => {
    if (!limits) return true;
    const fields = ['rpm_limit', 'rpd_limit', 'tpm_limit', 'tpd_limit'];
    return fields.every((f) => {
      const v = limits[f];
      return v === undefined || v === '' || v === null || (Number.isFinite(Number(v)) && Number(v) >= 0);
    });
  };

  if (providerKeys.length === 0) {
    return <Message info>暂无 Free Provider 配置。</Message>;
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      {providerKeys.map((key) => {
        const provider = freeProviders[key];
        const display = PROVIDER_DISPLAY[key] || { title: key, color: 'grey', icon: 'key' };
        const limits = provider.limits_override || {};
        const keyCount = Array.isArray(provider.keys) ? provider.keys.length : (provider.key_count || 0);
        const hasNewKey = !!(newKeys[key] && newKeys[key].trim());

        return (
          <Card fluid key={key} color={display.color}>
            <Card.Content>
              <Card.Header style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                <Icon name={display.icon} />
                {display.title}
                <Label basic size='small'>{key}</Label>
              </Card.Header>
              <Card.Meta>
                {keyCount} 个密钥已配置
              </Card.Meta>
              <Card.Description style={{ marginTop: 12 }}>
                <Form>
                  <Form.Field>
                    <Checkbox
                      toggle
                      label='启用'
                      checked={!!provider.enabled}
                      onChange={(_, { checked }) => updateProvider(key, 'enabled', checked)}
                    />
                  </Form.Field>

                  <Form.Field style={{ marginTop: 10 }}>
                    <label>添加新密钥</label>
                    <Input
                      type='password'
                      placeholder='输入新密钥（留空不保存）'
                      value={newKeys[key] || ''}
                      onChange={(_, { value }) =>
                        setNewKeys((prev) => ({ ...prev, [key]: value }))
                      }
                      action={
                        hasNewKey ? (
                          <Button
                            basic
                            color='green'
                            size='small'
                            onClick={() => {
                              const trimmed = (newKeys[key] || '').trim();
                              if (!trimmed) return;
                              const existingKeys = Array.isArray(provider.keys) ? provider.keys : [];
                              updateProvider(key, 'keys', [...existingKeys, trimmed]);
                              setNewKeys((prev) => ({ ...prev, [key]: '' }));
                            }}
                          >
                            <Icon name='plus' /> 添加
                          </Button>
                        ) : null
                      }
                    />
                  </Form.Field>

                  <Form.Group style={{ marginTop: 12 }}>
                    <Form.Field width={4}>
                      <label>RPM</label>
                      <Input
                        type='number'
                        size='small'
                        placeholder='默认'
                        value={limits.rpm_limit === undefined ? '' : limits.rpm_limit}
                        onChange={(_, { value }) => {
                          const parsed = value === '' ? undefined : parseInt(value, 10);
                          updateLimit(key, 'rpm_limit', Number.isFinite(parsed) ? parsed : undefined);
                        }}
                      />
                    </Form.Field>
                    <Form.Field width={4}>
                      <label>RPD</label>
                      <Input
                        type='number'
                        size='small'
                        placeholder='默认'
                        value={limits.rpd_limit === undefined ? '' : limits.rpd_limit}
                        onChange={(_, { value }) => {
                          const parsed = value === '' ? undefined : parseInt(value, 10);
                          updateLimit(key, 'rpd_limit', Number.isFinite(parsed) ? parsed : undefined);
                        }}
                      />
                    </Form.Field>
                    <Form.Field width={4}>
                      <label>TPM</label>
                      <Input
                        type='number'
                        size='small'
                        placeholder='默认'
                        value={limits.tpm_limit === undefined ? '' : limits.tpm_limit}
                        onChange={(_, { value }) => {
                          const parsed = value === '' ? undefined : parseInt(value, 10);
                          updateLimit(key, 'tpm_limit', Number.isFinite(parsed) ? parsed : undefined);
                        }}
                      />
                    </Form.Field>
                    <Form.Field width={4}>
                      <label>TPD</label>
                      <Input
                        type='number'
                        size='small'
                        placeholder='默认'
                        value={limits.tpd_limit === undefined ? '' : limits.tpd_limit}
                        onChange={(_, { value }) => {
                          const parsed = value === '' ? undefined : parseInt(value, 10);
                          updateLimit(key, 'tpd_limit', Number.isFinite(parsed) ? parsed : undefined);
                        }}
                      />
                    </Form.Field>
                  </Form.Group>
                  <Message info size='small' style={{ marginTop: 6 }}>
                    <Icon name='info circle' />
                    限额说明：留空 = 使用默认值，0 = 不限制，值必须 >= 0
                  </Message>
                  {!validateLimits(limits) && (
                    <Message error size='small'>
                      <Icon name='warning sign' />
                      限额值不能为负数
                    </Message>
                  )}
                </Form>
              </Card.Description>
            </Card.Content>
          </Card>
        );
      })}
    </div>
  );
};

export default FreeProvidersEditor;
