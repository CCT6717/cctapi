import React, { useState } from 'react';
import { Checkbox, Dropdown, Icon, Input, Label, Message } from 'semantic-ui-react';

const POOL_CN = { paid_high: '付费高质量池', cheap: '低成本池', local: '本地池', free: '免费池' };
const POOL_OPTIONS = Object.entries(POOL_CN).map(([k, v]) => ({ key: k, value: k, text: v }));

const QUALITY_CN = { high: '高', medium: '中', low: '低' };
const QUALITY_OPTIONS = Object.entries(QUALITY_CN).map(([k, v]) => ({ key: k, value: k, text: v }));

const COST_CN = { paid: '付费', cheap: '低成本', free: '免费' };
const COST_OPTIONS = Object.entries(COST_CN).map(([k, v]) => ({ key: k, value: k, text: v }));

const QUOTA_CN = { normal: '普通', free: '免费额度' };
const QUOTA_OPTIONS = Object.entries(QUOTA_CN).map(([k, v]) => ({ key: k, value: k, text: v }));

const FILTER_OPTIONS = [
  { key: 'All', text: '全部' },
  { key: 'paid_high', text: '付费高质量池' },
  { key: 'cheap', text: '低成本池' },
  { key: 'free', text: '免费池' },
  { key: 'local', text: '本地池' },
  { key: 'disabled', text: '已停用' },
];

const isAuto = (id) => String(id || '').startsWith('free:');

const NumInput = ({ value, onChange, disabled, width }) => (
  <Input type='number' size='mini' value={value == null ? '' : value}
    onChange={(_, { value: v }) => { const n = v === '' ? 0 : parseInt(v, 10); onChange(Number.isFinite(n) ? n : 0); }}
    disabled={disabled} style={{ width: width || 80 }} />
);

const DeploymentsEditor = ({ deployments, onChange }) => {
  const [filter, setFilter] = useState('All');
  const [expandedRows, setExpandedRows] = useState({});

  if (!deployments || typeof deployments !== 'object') return <Message warning>部署数据为空或格式错误</Message>;
  const allKeys = Object.keys(deployments);
  if (allKeys.length === 0) return <Message info>暂无部署配置。</Message>;

  const toggleRow = (k) => setExpandedRows((p) => ({ ...p, [k]: !p[k] }));
  const updateDep = (key, field, value) => {
    onChange({ ...deployments, [key]: { ...deployments[key], [field]: value } });
  };
  const updateCap = (key, cap, value) => {
    const map = { vision: 'supports_vision', stream: 'supports_stream', tools: 'supports_tools', json: 'supports_json' };
    updateDep(key, map[cap] || cap, value);
  };

  const filteredKeys = allKeys.filter((k) => {
    const dep = deployments[k];
    if (filter === 'All') return true;
    if (filter === 'disabled') return !dep.enabled;
    return dep.pool === filter;
  });

  return (
    <div>
      <div className='gateway-filter-bar'>
        {FILTER_OPTIONS.map(({ key, text }) => (
          <button key={key} className={`gateway-filter-btn ${filter === key ? 'active' : ''}`} onClick={() => setFilter(key)}>
            {text}
            {key !== 'All' && key !== 'disabled' && (
              <span style={{ marginLeft: 4, opacity: 0.7 }}>
                ({allKeys.filter((k) => deployments[k].pool === key).length})
              </span>
            )}
          </button>
        ))}
      </div>

      <div style={{ overflowX: 'auto' }}>
        <table className='gateway-table'>
          <thead>
            <tr>
              <th style={{ width: 30 }}></th>
              <th>启用</th>
              <th>部署 ID</th>
              <th>路由池</th>
              <th>真实模型</th>
              <th>质量等级</th>
              <th>成本等级</th>
              <th>额度模式</th>
              <th>通道</th>
            </tr>
          </thead>
          <tbody>
            {filteredKeys.length === 0 ? (
              <tr><td colSpan='9' style={{ textAlign: 'center', color: '#9ca3af' }}>无匹配部署</td></tr>
            ) : filteredKeys.map((key) => {
              const dep = deployments[key];
              const auto = isAuto(key);
              const rowOpen = !!expandedRows[key];
              return (
                <React.Fragment key={key}>
                  <tr className={auto ? 'auto-row' : ''}>
                    <td>
                      <span className='gateway-expand-toggle' onClick={() => toggleRow(key)}>
                        <Icon name={rowOpen ? 'chevron down' : 'chevron right'} />
                      </span>
                    </td>
                    <td>
                      <Checkbox checked={!!dep.enabled} onChange={(_, { checked }) => updateDep(key, 'enabled', checked)} disabled={auto} />
                    </td>
                    <td>
                      <strong style={{ fontSize: 12 }}>{key}</strong>
                      {auto && <Label basic size='mini' color='green' style={{ marginLeft: 6, fontSize: 10 }}><Icon name='lock' /> 自动免费池</Label>}
                    </td>
                    <td>
                      <Dropdown selection compact search options={POOL_OPTIONS} value={dep.pool || ''} onChange={(_, { value }) => updateDep(key, 'pool', value)} disabled={auto} style={{ minWidth: 110 }} />
                    </td>
                    <td>
                      {auto ? (
                        <span className='gateway-muted' title='该部署由免费供应商自动生成'>{dep.real_model || '-'}</span>
                      ) : (
                        <Input size='mini' value={dep.real_model || ''} onChange={(_, { value }) => updateDep(key, 'real_model', value)} style={{ width: 180 }} placeholder='如 gpt-4.1' />
                      )}
                    </td>
                    <td>
                      <Dropdown selection compact options={QUALITY_OPTIONS} value={dep.quality_tier || 'medium'} onChange={(_, { value }) => updateDep(key, 'quality_tier', value)} disabled={auto} style={{ minWidth: 60 }} />
                    </td>
                    <td>
                      <Dropdown selection compact options={COST_OPTIONS} value={dep.cost_tier || 'paid'} onChange={(_, { value }) => updateDep(key, 'cost_tier', value)} disabled={auto} style={{ minWidth: 70 }} />
                    </td>
                    <td>
                      <Dropdown selection compact options={QUOTA_OPTIONS} value={dep.quota_mode || 'normal'} onChange={(_, { value }) => updateDep(key, 'quota_mode', value)} disabled={auto} style={{ minWidth: 70 }} />
                    </td>
                    <td>
                      {auto ? <span className='gateway-muted'>{dep.channel_id || '-'}</span> : (
                        <NumInput value={dep.channel_id} onChange={(v) => updateDep(key, 'channel_id', v)} disabled={auto} width={70} />
                      )}
                    </td>
                  </tr>
                  {rowOpen && (
                    <tr>
                      <td colSpan='9' className='gateway-row-expanded'>
                        {auto && (
                          <Message info size='small' style={{ marginBottom: 8 }}>
                            <Icon name='lock' /> 该部署由免费供应商自动生成，请在免费供应商模块中管理。
                          </Message>
                        )}
                        <div className='gateway-detail-grid'>
                          <div className='gateway-detail-item'>
                            <span className='detail-label'>真实模型</span>
                            {auto ? (
                              <span className='gateway-mono'>{dep.real_model || '-'}</span>
                            ) : (
                              <Input size='mini' value={dep.real_model || ''} onChange={(_, { value }) => updateDep(key, 'real_model', value)} style={{ width: 200 }} placeholder='如 deepseek-chat' />
                            )}
                          </div>
                          <div className='gateway-detail-item'>
                            <span className='detail-label'>上下文长度</span>
                            <NumInput value={dep.context_length} onChange={(v) => updateDep(key, 'context_length', v)} disabled={auto} width={90} />
                          </div>
                          <div className='gateway-detail-item'>
                            <span className='detail-label'>优先级</span>
                            <NumInput value={dep.priority} onChange={(v) => updateDep(key, 'priority', v)} disabled={auto} width={60} />
                          </div>
                          <div className='gateway-detail-item'>
                            <span className='detail-label'>权重</span>
                            <NumInput value={dep.weight} onChange={(v) => updateDep(key, 'weight', v)} disabled={auto} width={60} />
                          </div>
                          <div className='gateway-detail-item'>
                            <span className='detail-label'>每分钟请求数</span>
                            <NumInput value={dep.rpm_limit} onChange={(v) => updateDep(key, 'rpm_limit', v)} disabled={auto} width={70} />
                          </div>
                          <div className='gateway-detail-item'>
                            <span className='detail-label'>每日请求数</span>
                            <NumInput value={dep.rpd_limit} onChange={(v) => updateDep(key, 'rpd_limit', v)} disabled={auto} width={70} />
                          </div>
                          <div className='gateway-detail-item'>
                            <span className='detail-label'>每分钟 Token 数</span>
                            <NumInput value={dep.tpm_limit} onChange={(v) => updateDep(key, 'tpm_limit', v)} disabled={auto} width={70} />
                          </div>
                          <div className='gateway-detail-item'>
                            <span className='detail-label'>每日 Token 数</span>
                            <NumInput value={dep.tpd_limit} onChange={(v) => updateDep(key, 'tpd_limit', v)} disabled={auto} width={70} />
                          </div>
                        </div>
                        <div className='gateway-detail-caps'>
                          {[
                            { label: '视觉', field: 'vision', val: dep.supports_vision },
                            { label: '流式', field: 'stream', val: dep.supports_stream },
                            { label: '工具', field: 'tools',  val: dep.supports_tools },
                            { label: 'JSON', field: 'json',   val: dep.supports_json },
                          ].map(({ label, field, val }) => (
                            <span key={field} style={{ display: 'inline-flex', alignItems: 'center', gap: 4, fontSize: 12 }}>
                              <Checkbox checked={!!val} onChange={(_, { checked }) => updateCap(key, field, checked)} disabled={auto} />
                              {label}
                            </span>
                          ))}
                        </div>
                      </td>
                    </tr>
                  )}
                </React.Fragment>
              );
            })}
          </tbody>
        </table>
      </div>
      <Message info size='small' style={{ marginTop: 8 }}>
        <Icon name='info circle' />
        <code>free:</code> 开头的部署为自动生成（<Icon name='lock' style={{ display: 'inline' }} />），请在免费供应商标签页管理。
      </Message>
    </div>
  );
};

export default DeploymentsEditor;
