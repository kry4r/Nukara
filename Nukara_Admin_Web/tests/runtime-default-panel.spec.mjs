import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'

const root = path.resolve(process.cwd(), 'Nukara_Admin_Web')
const appSource = fs.readFileSync(path.join(root, 'src/App.vue'), 'utf8')
const styleSource = fs.readFileSync(path.join(root, 'src/style.css'), 'utf8')
const emailPanelSource = fs.readFileSync(path.join(root, 'src/components/EmailAuthPanel.vue'), 'utf8')
const postTurnPanelSource = fs.readFileSync(path.join(root, 'src/components/PostTurnModelPanel.vue'), 'utf8')
const summaryPanelSource = fs.readFileSync(path.join(root, 'src/components/SummaryModelPanel.vue'), 'utf8')

assert.match(appSource, /const providerListOpen = ref\(false\)/, 'Runtime Default should own an explicit list expander state')
assert.match(appSource, /<div v-if="providerListOpen" class="runtime-default-expander">/, 'Provider list should render inside Runtime Default expander')
assert.ok(
  appSource.indexOf('<div class="default-summary">') < appSource.indexOf('<div v-if="providerListOpen" class="runtime-default-expander">'),
  'Runtime summary should render before expanded provider list',
)
assert.ok(
  appSource.indexOf('<div v-if="providerListOpen" class="runtime-default-expander">') < appSource.lastIndexOf('<div class="provider-card-list">'),
  'Provider list should live inside Runtime Default expander block',
)
assert.doesNotMatch(styleSource, /\.left-column \{[\s\S]*overflow-y:\s*auto;/, 'Left column should not trap scrolling inside a nested scroll area')
assert.doesNotMatch(styleSource, /\.left-column \{[\s\S]*max-height:\s*calc\(100vh - 40px\);/, 'Left column should not cap height to viewport when panels grow taller')

for (const [name, source] of [
  ['EmailAuthPanel', emailPanelSource],
  ['PostTurnModelPanel', postTurnPanelSource],
  ['SummaryModelPanel', summaryPanelSource],
]) {
  assert.match(source, /\.panel-header h2 \{[\s\S]*font-size: 18px;/, `${name} title font should be reduced to 18px`)
  assert.match(source, /\.panel-eyebrow \{[\s\S]*font-size: 11px;/, `${name} eyebrow font should be reduced to 11px`)
  assert.match(source, /\.panel-desc \{[\s\S]*font-size: 13px;/, `${name} description font should be reduced to 13px`)
}
