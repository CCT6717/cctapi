import React from 'react';
import { Label } from 'semantic-ui-react';
import { copy, showSuccess, showWarning, timestamp2string } from '../../helpers';

/**
 * renderTimestamp — 日志时间戳（点击复制 request_id）
 */
export const renderTimestamp = (timestamp, request_id) => (
  <code
    onClick={async () => {
      if (await copy(request_id)) {
        showSuccess(`已复制请求 ID：${request_id}`);
      } else {
        showWarning(`请求 ID 复制失败：${request_id}`);
      }
    }}
    style={{ cursor: 'pointer' }}
  >
    {timestamp2string(timestamp)}
  </code>
);

/**
 * renderType — 日志类型标签
 */
export const renderType = (type) => {
  switch (type) {
    case 1:
      return (
        <Label basic color='green'>
          充值
        </Label>
      );
    case 2:
      return (
        <Label basic color='olive'>
          消费
        </Label>
      );
    case 3:
      return (
        <Label basic color='orange'>
          管理
        </Label>
      );
    case 4:
      return (
        <Label basic color='purple'>
          系统
        </Label>
      );
    case 5:
      return (
        <Label basic color='violet'>
          测试
        </Label>
      );
    default:
      return (
        <Label basic color='black'>
          未知
        </Label>
      );
  }
};

/**
 * getColorByElapsedTime — 根据耗时返回颜色名
 */
export const getColorByElapsedTime = (elapsedTime) => {
  if (elapsedTime === undefined || 0) return 'black';
  if (elapsedTime < 1000) return 'green';
  if (elapsedTime < 3000) return 'olive';
  if (elapsedTime < 5000) return 'yellow';
  if (elapsedTime < 10000) return 'orange';
  return 'red';
};

/**
 * renderDetail — 日志详情（content + 耗时 + stream 标签）
 */
export const renderDetail = (log) => (
  <>
    {log.content}
    <br />
    {log.elapsed_time && (
      <Label
        basic
        size={'mini'}
        color={getColorByElapsedTime(log.elapsed_time)}
      >
        {log.elapsed_time} ms
      </Label>
    )}
    {log.is_stream && (
      <>
        <Label size={'mini'} color='pink'>
          Stream
        </Label>
      </>
    )}
    {log.system_prompt_reset && (
      <>
        <Label basic size={'mini'} color='red'>
          System Prompt Reset
        </Label>
      </>
    )}
  </>
);
