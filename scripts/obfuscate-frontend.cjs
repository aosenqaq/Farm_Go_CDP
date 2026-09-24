const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');

const projectRoot = path.resolve(__dirname, '..');
const frontendRoot = path.join(projectRoot, 'frontend');
const JavaScriptObfuscator = require(require.resolve('javascript-obfuscator', { paths: [frontendRoot] }));

const obfuscatorOptions = {
  compact: true,
  controlFlowFlattening: false,
  deadCodeInjection: false,
  identifierNamesGenerator: 'hexadecimal',
  renameGlobals: false,
  selfDefending: false,
  sourceMap: false,
  stringArray: true,
  stringArrayEncoding: ['base64'],
  stringArrayThreshold: 0.75,
  transformObjectKeys: false,
  unicodeEscapeSequence: false,
  reservedNames: ['^App$', '^Events(On|Off|Once|Emit)$'],
  reservedStrings: ['wailsjs', 'runtime', 'license:status', 'license:revoked'],
};

function isInside(root, candidate) {
  const relative = path.relative(root, candidate);
  return relative !== '' && !relative.startsWith(`..${path.sep}`) && relative !== '..' && !path.isAbsolute(relative);
}

function collectFiles(root, extension) {
  if (!fs.existsSync(root)) {
    return [];
  }
  const files = [];
  for (const entry of fs.readdirSync(root, { withFileTypes: true })) {
    const filePath = path.join(root, entry.name);
    if (entry.isDirectory()) {
      files.push(...collectFiles(filePath, extension));
    } else if (entry.isFile() && filePath.endsWith(extension)) {
      files.push(filePath);
    }
  }
  return files;
}

function collectJavaScriptAssets(distRoot = path.join(frontendRoot, 'dist')) {
  const assetsRoot = path.resolve(distRoot, 'assets');
  if (!fs.existsSync(assetsRoot)) {
    throw new Error(`Protected frontend assets directory is missing: ${assetsRoot}`);
  }
  return collectFiles(assetsRoot, '.js')
    .map((filePath) => path.resolve(filePath))
    .filter((filePath) => isInside(assetsRoot, filePath))
    .sort();
}

function sha256File(filePath) {
  return crypto.createHash('sha256').update(fs.readFileSync(filePath)).digest('hex');
}

function removeSourceMaps(distRoot) {
  const removed = [];
  for (const filePath of collectFiles(distRoot, '.map')) {
    fs.rmSync(filePath);
    removed.push(path.relative(distRoot, filePath).replaceAll(path.sep, '/'));
  }
  return removed.sort();
}

function obfuscateFrontendAssets(distRoot = path.join(frontendRoot, 'dist')) {
  const resolvedDistRoot = path.resolve(distRoot);
  const assetsRoot = path.join(resolvedDistRoot, 'assets');
  const files = collectJavaScriptAssets(resolvedDistRoot);
  if (files.length === 0) {
    throw new Error(`No JavaScript assets found below ${assetsRoot}`);
  }

  const manifestFiles = files.map((filePath) => {
    const source = fs.readFileSync(filePath, 'utf8');
    const protectedSource = JavaScriptObfuscator.obfuscate(source, obfuscatorOptions).getObfuscatedCode();
    fs.writeFileSync(filePath, protectedSource, 'utf8');
    return {
      path: path.relative(resolvedDistRoot, filePath).replaceAll(path.sep, '/'),
      bytes: fs.statSync(filePath).size,
      sha256: sha256File(filePath),
    };
  });

  return { files: manifestFiles, sourceMapsRemoved: removeSourceMaps(resolvedDistRoot) };
}

if (require.main === module) {
  process.stdout.write(`${JSON.stringify(obfuscateFrontendAssets(), null, 2)}\n`);
}

module.exports = { collectJavaScriptAssets, obfuscateFrontendAssets, sha256File };
