const fs = require('fs');
const path = require('path');

const source = path.resolve(__dirname, '..', 'build');
const targetRoot = path.resolve(__dirname, '..', '..', 'build');
const target = path.join(targetRoot, 'default');

function assertWithin(root, candidate) {
  const relative = path.relative(root, candidate);
  if (relative.startsWith('..') || path.isAbsolute(relative)) {
    throw new Error(`Refusing to write outside build root: ${candidate}`);
  }
}

if (!fs.existsSync(source)) {
  throw new Error(`Build directory not found: ${source}`);
}

assertWithin(targetRoot, target);

fs.rmSync(target, { recursive: true, force: true });
fs.mkdirSync(targetRoot, { recursive: true });
fs.renameSync(source, target);

console.log(`Build output moved to ${target}`);
