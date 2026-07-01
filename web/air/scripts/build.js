const fs = require('fs');
const path = require('path');

const source = path.join(__dirname, '..', 'build');
const target = path.join(__dirname, '..', '..', 'build', 'air');

if (fs.existsSync(target)) {
  fs.rmSync(target, { recursive: true, force: true });
}

if (!fs.existsSync(source)) {
  console.error(`Build output not found: ${source}`);
  process.exit(1);
}

fs.renameSync(source, target);
console.log(`Moved build output to ${target}`);
