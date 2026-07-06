import React from 'react';
import { Activity } from 'lucide-react';
import { MetricCard } from './MetricCard';

const meta = {
  title: 'CCT UI/MetricCard',
  component: MetricCard,
};

export default meta;

export const Basic = {
  render: () => (
    <MetricCard
      title='Healthy deployments'
      value='18'
      caption='Updated every 15 seconds'
      icon={<Activity size={18} />}
      style={{ width: 260 }}
    />
  ),
};
