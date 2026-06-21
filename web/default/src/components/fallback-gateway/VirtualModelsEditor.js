import React from 'react';
import { Card, Checkbox, Dropdown, Form, Icon, Label, Message } from 'semantic-ui-react';

const STRATEGY_OPTIONS = [
  { key: 'quality_first', value: 'quality_first', text: '质量优先' },
  { key: 'cost_first', value: 'cost_first', text: '成本优先' },
  { key: 'free_first', value: 'free_first', text: '免费优先' },
];

const DEFAULT_POOL_OPTIONS = [
  { key: 'paid_high', value: 'paid_high', text: 'paid_high' },
  { key: 'cheap', value: 'cheap', text: 'cheap' },
  { key: 'local', value: 'local', text: 'local' },
  { key: 'free', value: 'free', text: 'free' },
];

const VM_DISPLAY = {
  'cct/high': { title: 'CCT High', color: 'blue', desc: '高质量模型路由' },
  'cct/low': { title: 'CCT Low', color: 'teal', desc: '低成本模型路由' },
  'cct/free': { title: 'CCT Free', color: 'green', desc: '免费模型路由' },
};

const VirtualModelsEditor = ({ virtualModels, onChange }) => {
  if (!virtualModels || typeof virtualModels !== 'object') {
    return <Message warning>虚拟模型数据为空或格式错误</Message>;
  }

  const vmKeys = Object.keys(virtualModels);

  const updateVM = (key, field, value) => {
    const updated = {
      ...virtualModels,
      [key]: {
        ...virtualModels[key],
        [field]: value,
      },
    };
    onChange(updated);
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      {vmKeys.length === 0 && (
        <Message info>暂无虚拟模型配置，请检查后端数据。</Message>
      )}
      {vmKeys.map((key) => {
        const vm = virtualModels[key];
        const display = VM_DISPLAY[key] || { title: key, color: 'grey', desc: '' };
        const showDegradeToLow = key === 'cct/high';
        const showDegradeToFree = key === 'cct/high' || key === 'cct/low';

        return (
          <Card fluid key={key} color={display.color}>
            <Card.Content>
              <Card.Header style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                <Icon name='server' />
                {display.title}
                <Label basic size='small'>{key}</Label>
              </Card.Header>
              <Card.Meta>{display.desc}</Card.Meta>
              <Card.Description style={{ marginTop: 12 }}>
                <Form>
                  <Form.Group inline style={{ flexWrap: 'wrap', gap: 8 }}>
                    <Form.Field>
                      <Checkbox
                        toggle
                        label='启用'
                        checked={!!vm.enabled}
                        onChange={(_, { checked }) => updateVM(key, 'enabled', checked)}
                      />
                    </Form.Field>
                    <Form.Field width={4}>
                      <label>策略</label>
                      <Dropdown
                        selection
                        search
                        options={STRATEGY_OPTIONS}
                        value={vm.strategy || 'quality_first'}
                        onChange={(_, { value }) => updateVM(key, 'strategy', value)}
                        style={{ minWidth: 140 }}
                      />
                    </Form.Field>
                    <Form.Field width={6}>
                      <label>Pools</label>
                      <Dropdown
                        selection
                        multiple
                        search
                        allowAdditions
                        options={DEFAULT_POOL_OPTIONS}
                        value={Array.isArray(vm.pools) ? vm.pools : []}
                        onChange={(_, { value }) => updateVM(key, 'pools', value)}
                        onAddItem={(_, { value }) => {
                          // allowAdditions handles this automatically
                        }}
                        style={{ minWidth: 240 }}
                        placeholder='选择或添加 pool'
                      />
                    </Form.Field>
                  </Form.Group>
                  {showDegradeToLow && (
                    <Form.Field style={{ marginTop: 8 }}>
                      <Checkbox
                        label='允许降级到 cct/low'
                        checked={!!vm.allow_degrade_to_low}
                        onChange={(_, { checked }) => updateVM(key, 'allow_degrade_to_low', checked)}
                      />
                    </Form.Field>
                  )}
                  {showDegradeToFree && (
                    <Form.Field style={{ marginTop: 4 }}>
                      <Checkbox
                        label='允许降级到 cct/free'
                        checked={!!vm.allow_degrade_to_free}
                        onChange={(_, { checked }) => updateVM(key, 'allow_degrade_to_free', checked)}
                      />
                    </Form.Field>
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

export default VirtualModelsEditor;
