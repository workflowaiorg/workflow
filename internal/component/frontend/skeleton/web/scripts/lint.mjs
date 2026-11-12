// The checks CI runs on web/, each for a rule a review cannot hold by hand. A name is matched as
// plain text, so a comment or a string that mentions one fails too.
//
// 1. HTTP seam: only src/services/ talks to the network (fetch, XMLHttpRequest, WebSocket,
//    EventSource, sendBeacon) or reads api.gen, so every call the app makes is in one place, typed
//    against the contract.
// 2. No raw HTML: nothing under src/ turns a string into markup (v-html, innerHTML, outerHTML,
//    insertAdjacentHTML, setHTMLUnsafe, parseHTMLUnsafe, parseFromString, document.write,
//    createContextualFragment, srcdoc). Text in {{ }} is escaped by Vue; text that users or a
//    model wrote is not safe as markup. When markdown is truly needed, sanitise it where it is
//    rendered and make this check require the sanitiser there; do not delete it.
// 3. Fresh API types: src/services/api.gen.d.ts is what npm run api:types writes from
//    ../api/openapi.yaml, so a field renamed in the API fails the type check, not a screen.
import { execFileSync } from 'node:child_process'
import { mkdtempSync, readdirSync, readFileSync, rmSync, statSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, relative } from 'node:path'
import { fileURLToPath } from 'node:url'

const WEB = fileURLToPath(new URL('..', import.meta.url))
const SRC = join(WEB, 'src')
const GENERATED = join(SRC, 'services', 'api.gen.d.ts')

const files = []
function walk(dir) {
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry)
    if (statSync(full).isDirectory()) walk(full)
    else if (/\.(js|ts|vue)$/.test(entry) && full !== GENERATED) files.push({ rel: relative(SRC, full), text: readFileSync(full, 'utf8') })
  }
}
walk(SRC)
const failures = []

// 1. HTTP seam
const NETWORK = /(?<![.\w])fetch\s*\(|\b(?:window|globalThis|self)\.fetch\b|\b(?:XMLHttpRequest|WebSocket|EventSource|sendBeacon)\b|api\.gen\b/g
for (const f of files.filter((f) => !f.rel.startsWith('services/'))) {
  for (const use of new Set(f.text.match(NETWORK))) {
    failures.push(`seam: src/${f.rel} uses ${use.replace(/\s*\($/, '')}: call a function of src/services/ instead`)
  }
}

// 2. Raw HTML
const RAW_HTML = /v-html|\b(?:innerHTML|outerHTML|insertAdjacentHTML|setHTMLUnsafe|parseHTMLUnsafe|parseFromString|createContextualFragment|srcdoc)\b|document\.write/g
for (const f of files) {
  for (const sink of new Set(f.text.match(RAW_HTML))) {
    failures.push(`raw html: src/${f.rel} uses ${sink}: render text with {{ }}`)
  }
}

// 3. Fresh API types
const scratch = mkdtempSync(join(tmpdir(), 'api-types-'))
try {
  const fresh = join(scratch, 'api.gen.d.ts')
  execFileSync(join(WEB, 'node_modules', '.bin', 'openapi-typescript'), ['../api/openapi.yaml', '-o', fresh], { cwd: WEB, stdio: 'pipe' })
  let committed
  try {
    committed = readFileSync(GENERATED, 'utf8')
  } catch {
    failures.push('api types: there is no src/services/api.gen.d.ts: run npm run api:types and commit it')
  }
  if (committed !== undefined && committed !== readFileSync(fresh, 'utf8')) {
    failures.push('api types: src/services/api.gen.d.ts does not match ../api/openapi.yaml: run npm run api:types and commit it')
  }
} finally {
  rmSync(scratch, { recursive: true, force: true })
}

if (failures.length > 0) {
  console.error('web checks failed:')
  for (const line of failures) console.error(`  ${line}`)
  process.exit(1)
}
console.log('web checks passed: HTTP seam, raw HTML, fresh API types.')
