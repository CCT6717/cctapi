import React from 'react';
import { Label, Popup } from 'semantic-ui-react';
import { CHANNEL_OPTIONS } from '../../constants';
import { getChannelModels } from '../../helpers';
import { renderTimestamp } from './commonRenderers';

// Re-export for backward compatibility
export { renderTimestamp };

/**
 * processChannelData — 为 channel 添加 model_options 和 test_model
 */
export const processChannelData = (channel) => {
  const models = getChannelModels(channel.type);
  const modelOptions = models.map((model) => ({
    key: model,
    text: model,
    value: model,
  }));
  return {
    ...channel,
    model_options: modelOptions,
    test_model: channel.test_model || (models.length > 0 ? models[0] : ''),
  };
};

/**
 * renderType — 渠道类型标签
 */
export const renderType = (type, t) => {
  const option = CHANNEL_OPTIONS.find((o) => o.value === type);
  const label = option ? option.text : t('channel.types.unknown');
  return (
    <Label basic size='mini'>
      {label}
    </Label>
  );
};

/**
 * renderBalance — 余额显示（负数红色）
 */
export const renderBalance = (type, balance, t) => {
  if (type === 1) {
    if (balance === -1) {
      return (
        <Label basic size='mini'>
          {t('channel.balance.unlimited')}
        </Label>
      );
    } else if (balance === -2) {
      return (
        <Label basic size='mini'>
          {t('channel.balance.unknown')}
        </Label>
      );
    } else {
      if (balance < 0) {
        return (
          <Label basic color='red' size='mini'>
            ${Math.abs(balance).toFixed(2)}
          </Label>
        );
      } else {
        return (
          <Label basic color='green' size='mini'>
            ${balance.toFixed(2)}
          </Label>
        );
      }
    }
  }
  return <></>;
};

/**
 * renderStatus — 渠道状态标签（纯展示）
 */
export const renderStatus = (status, t) => {
  switch (status) {
    case 1:
      return (
        <Label basic color='green'>
          {t('channel.table.status_enabled')}
        </Label>
      );
    case 2:
      return (
        <Popup
          trigger={
            <Label basic color='red'>
              {t('channel.table.status_disabled')}
            </Label>
          }
          content={t('channel.table.status_disabled_tip')}
          basic
        />
      );
    case 3:
      return (
        <Popup
          trigger={
            <Label basic color='yellow'>
              {t('channel.table.status_auto_disabled')}
            </Label>
          }
          content={t('channel.table.status_auto_disabled_tip')}
          basic
        />
      );
    default:
      return (
        <Label basic color='grey'>
          {t('channel.table.status_unknown')}
        </Label>
      );
  }
};

/**
 * renderResponseTime — 响应时间标签（5 档颜色，单位 ms → 秒显示）
 */
export const renderResponseTime = (responseTime, t) => {
  let time = responseTime / 1000;
  time = time.toFixed(2) + 's';
  if (responseTime === 0) {
    return (
      <Label basic color='grey'>
        {t('channel.table.not_tested')}
      </Label>
    );
  } else if (responseTime <= 1000) {
    return (
      <Label basic color='green'>
        {time}
      </Label>
    );
  } else if (responseTime <= 3000) {
    return (
      <Label basic color='olive'>
        {time}
      </Label>
    );
  } else if (responseTime <= 5000) {
    return (
      <Label basic color='yellow'>
        {time}
      </Label>
    );
  } else {
    return (
      <Label basic color='red'>
        {time}
      </Label>
    );
  }
};
