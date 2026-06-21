import {
  isAutoFreeDeploymentId,
  isFreeDeployment,
  providerFromDeploymentId,
} from './freePoolUtils';

describe('freePoolUtils', () => {
  it('只把后端自动生成格式识别为自动免费部署', () => {
    expect(isAutoFreeDeploymentId('free:openrouter-1')).toBe(true);
    expect(isAutoFreeDeploymentId('free:groq-a1b2c3d4')).toBe(true);
    expect(isAutoFreeDeploymentId('free:custom-model')).toBe(false);
    expect(isAutoFreeDeploymentId('paid_high-model')).toBe(false);
  });

  it('按 pool=free 或自动部署 ID 判断免费部署', () => {
    expect(isFreeDeployment('manual-id', { pool: 'free' })).toBe(true);
    expect(isFreeDeployment('free:openrouter-a1b2c3d4', { pool: 'cheap' })).toBe(true);
    expect(isFreeDeployment('normal-dep', { pool: 'cheap' })).toBe(false);
  });

  it('从自动部署 ID 提取供应商', () => {
    expect(providerFromDeploymentId('free:openrouter-a1b2c3d4')).toBe('openrouter');
    expect(providerFromDeploymentId('free:groq-22')).toBe('groq');
    expect(providerFromDeploymentId('free:custom-model')).toBe('-');
  });
});
