export const clampScore = (value) => {
  const number = Number(value);
  if (!Number.isFinite(number)) {
    return 0;
  }
  return Math.max(0, Math.min(100, number));
};

export const sortScoreItems = (items) =>
  (Array.isArray(items) ? items : [])
    .filter(Boolean)
    .slice()
    .sort((left, right) => {
      const scoreDiff = Number(right?.value || 0) - Number(left?.value || 0);
      if (scoreDiff !== 0) {
        return scoreDiff;
      }
      return String(left?.deploymentId || '').localeCompare(
        String(right?.deploymentId || ''),
        'zh-CN'
      );
    });
