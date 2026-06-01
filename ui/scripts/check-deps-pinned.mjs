import fs from 'node:fs'

const pkg = JSON.parse(fs.readFileSync(new URL('../package.json', import.meta.url), 'utf8'))
const groups = ['dependencies', 'devDependencies', 'optionalDependencies']
let failed = false
for (const group of groups) {
  for (const [name, version] of Object.entries(pkg[group] ?? {})) {
    if (typeof version === 'string' && (/^[~^]/.test(version) || version === 'latest')) {
      console.error(`${group}.${name} is not pinned: ${version}`)
      failed = true
    }
  }
}
if (failed) process.exit(1)
