import React from 'react';
import { Checkbox, Form, Icon, Input, Label, Message, Table } from 'semantic-ui-react';

const PROVIDER_DISPLAY = {
  openrouter: { title: 'OpenRouter', color: 'purple', icon: 'cloud' },
  groq: { title: 'Groq', color: 'orange', icon: 'lightning' },
  kilo: { title: 'Kilo', color: 'teal', icon: 'bolt' },
  pollinations: { title: 'Pollinations', color: 'green', icon: 'leaf' },
  ovh: { title: 'OVH Cloud', color: 'blue', icon: 'server' },
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

const FreeProvidersEditor = ({ freeProviders, onChange }) => {

  if (!freeProviders || typeof freeProviders !== 'object') {
    return <Message warning>免费供应商数据为空或格式错误</Message>;
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
    return <Message info>暂无免费供应商配置。</Message>;
  }

  return (
    <div>
      <Table compact celled striped>
        <Table.Header>
          <Table.Row>
            <Table.HeaderCell>启用</Table.HeaderCell>
            <Table.HeaderCell>供应商</Table.HeaderCell>
            <Table.HeaderCell>密钥数量</Table.HeaderCell>
            <Table.HeaderCell>限额覆盖</Table.HeaderCell>
            <Table.HeaderCell>状态</Table.HeaderCell>
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {providerKeys.map((key) => {
            const provider = freeProviders[key];
            const display = PROVIDER_DISPLAY[key] || { title: key, color: 'grey', icon: 'key' };
            const limits = provider.limits_override || {};
            const keyCount = provider.key_count || 0;
            const invalidLimits = !validateLimits(limits);

            return (
              <Table.Row key={key}>
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
                  </div>
                </Table.Cell>
                <Table.Cell>
                  {keyCount} 个密钥
                </Table.Cell>
                <Table.Cell>
                  <Form size='small'>
                    <Form.Group widths='equal'>
                      {['rpm_limit', 'rpd_limit', 'tpm_limit', 'tpd_limit'].map((field) => (
                        <Form.Field key={field}>
                          <label>{field.replace('_limit', '').toUpperCase()}</label>
                          <Input
                            type='number'
                            size='mini'
                            placeholder='默认'
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
                        限额值不能为负数
                      </Message>
                    )}
                  </Form>
                </Table.Cell>
                <Table.Cell>
                  {provider.enabled ? (
                    <Label color='green' basic>已启用（{keyCount} 个密钥）</Label>
                  ) : (
                    <Label color='grey' basic>已停用</Label>
                  )}
                </Table.Cell>
              </Table.Row>
            );
          })}
        </Table.Body>
      </Table>
      <Message info size='small'>
        <Icon name='info circle' />
        空值表示使用系统默认限额，0 表示不限制，大于 0 表示覆盖默认限额。密钥只显示数量，不在页面中展示或编辑。
      </Message>
    </div>
  );
};

export default FreeProvidersEditor;
