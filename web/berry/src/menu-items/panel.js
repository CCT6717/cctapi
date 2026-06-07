// assets
import {
  IconActivity,
  IconAdjustments,
  IconArticle,
  IconCoin,
  IconDashboard,
  IconGardenCart,
  IconKey,
  IconSitemap,
  IconSortDescending,
  IconUser,
  IconUserScan
} from '@tabler/icons-react';

// constant
const icons = { IconDashboard, IconSitemap, IconArticle, IconCoin, IconAdjustments, IconKey, IconGardenCart, IconUser, IconUserScan };

// ==============================|| DASHBOARD MENU ITEMS ||============================== //

const panel = {
  id: 'panel',
  type: 'group',
  children: [
    {
      id: 'dashboard',
      title: '总览',
      type: 'item',
      url: '/panel/dashboard',
      icon: icons.IconDashboard,
      breadcrumbs: false,
      isAdmin: false
    },
    {
      id: 'channel',
      title: '渠道',
      type: 'item',
      url: '/panel/channel',
      icon: icons.IconSitemap,
      breadcrumbs: false,
      isAdmin: true
    },
    {
      id: 'token',
      title: '令牌',
      type: 'item',
      url: '/panel/token',
      icon: icons.IconKey,
      breadcrumbs: false
    },
    {
      id: 'log',
      title: '日志',
      type: 'item',
      url: '/panel/log',
      icon: icons.IconArticle,
      breadcrumbs: false
    },
    {
      id: 'redemption',
      title: '兑换',
      type: 'item',
      url: '/panel/redemption',
      icon: icons.IconCoin,
      breadcrumbs: false,
      isAdmin: true
    },
    {
      id: 'topup',
      title: '充值',
      type: 'item',
      url: '/panel/topup',
      icon: icons.IconGardenCart,
      breadcrumbs: false
    },
    {
      id: 'user',
      title: '用户',
      type: 'item',
      url: '/panel/user',
      icon: icons.IconUser,
      breadcrumbs: false,
      isAdmin: true
    },
    {
      id: 'profile',
      title: '我的',
      type: 'item',
      url: '/panel/profile',
      icon: icons.IconUserScan,
      breadcrumbs: false,
      isAdmin: false
    },
    // —— 回退系统 ——
    {
      id: 'fallback-panel',
      title: '回退面板',
      type: 'item',
      url: '/fallback/dashboard',
      icon: icons.IconDashboard,
      breadcrumbs: false,
      external: true,
      isAdmin: true
    },
    {
      id: 'fallback-metrics',
      title: '监控指标',
      type: 'item',
      url: '/fallback/metrics',
      icon: icons.IconActivity,
      breadcrumbs: false,
      external: true,
      isAdmin: true
    },
    {
      id: 'fallback-scores',
      title: '排序分数',
      type: 'item',
      url: '/fallback/scores',
      icon: icons.IconSortDescending,
      breadcrumbs: false,
      external: true,
      isAdmin: true
    },
    {
      id: 'fallback-alerts',
      title: '告警历史',
      type: 'item',
      url: '/fallback/alerts',
      icon: icons.IconActivity,
      breadcrumbs: false,
      external: true,
      isAdmin: true
    },
    {
      id: 'fallback-logs',
      title: '切换日志',
      type: 'item',
      url: '/fallback/logs',
      icon: icons.IconArticle,
      breadcrumbs: false,
      external: true,
      isAdmin: true
    },
    {
      id: 'setting',
      title: '设置',
      type: 'item',
      url: '/panel/setting',
      icon: icons.IconAdjustments,
      breadcrumbs: false,
      isAdmin: true
    }
  ]
};

export default panel;
