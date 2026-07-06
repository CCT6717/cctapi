import React from 'react';
import { Space } from 'antd';
import { StatusTag } from './StatusTag';

const meta = {
  title: 'CCT UI/StatusTag',
  component: StatusTag,
};

export default meta;

export const States = {
  render: () => (
    <Space wrap>
      <StatusTag status='healthy'>healthy</StatusTag>
      <StatusTag status='rate_limited'>rate limited</StatusTag>
      <StatusTag status='invalid'>invalid</StatusTag>
      <StatusTag status='disabled'>disabled</StatusTag>
    </Space>
  ),
};
