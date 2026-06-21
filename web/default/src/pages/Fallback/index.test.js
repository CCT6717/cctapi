import { sortScoreItems } from './scoreUtils';

describe('sortScoreItems', () => {
  it('按分数从高到低排序，100 分必须排前面', () => {
    const result = sortScoreItems([
      { deploymentId: 'model-c', value: 88.2 },
      { deploymentId: 'doubao-pro', value: 100 },
      { deploymentId: 'model-a', value: 91.5 },
    ]);

    expect(result.map((item) => item.deploymentId)).toEqual([
      'doubao-pro',
      'model-a',
      'model-c',
    ]);
  });

  it('同分时按 deploymentId 排序', () => {
    const result = sortScoreItems([
      { deploymentId: 'b-model', value: 90 },
      { deploymentId: 'a-model', value: 90 },
    ]);

    expect(result.map((item) => item.deploymentId)).toEqual([
      'a-model',
      'b-model',
    ]);
  });
});
