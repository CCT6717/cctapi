import React from 'react';
import { timestamp2string } from '../../helpers';

/**
 * renderTimestamp — 通用时间戳渲染（各表格共用）
 */
export const renderTimestamp = (timestamp) => {
  if (!timestamp) return <></>;
  return <>{timestamp2string(timestamp)}</>;
};
