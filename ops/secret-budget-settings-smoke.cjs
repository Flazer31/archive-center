// Synthetic settings storage boundaries; production normalization/save/load owners.
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const assert = require('node:assert/strict');
const file = process.argv[2] || path.join(__dirname, '../Archive Center.js');
const source = fs.readFileSync(file, 'utf8').replace(/\r/g, '');
assert(source.includes('await init();'));
let stored = null;
const writes = [];
const context = vm.createContext({console, URL, TextEncoder, TextDecoder,
  setTimeout() { throw new Error('Unexpected timer'); },
  setInterval() { throw new Error('Unexpected timer'); },
  fetch() { throw new Error('Unexpected network'); },
  recordSettings: async (key, value) => { assert(key.endsWith('_settings')); stored = value; writes.push(value); },
  readSettings: async (key) => { assert(key.endsWith('_settings')); return stored; },
});
const expose = `
  persistentSet = globalThis.recordSettings;
  persistentGet = globalThis.readSettings;
  syncConfigToBackend = async () => ({ok:true});
  markBackendRuntimeConfigDirty = () => {};
  updateRuntimeState = () => {};
  globalThis.testSettings = {sanitizeSettings, updateSettings, loadSettings, getSettings, t};
`;
(async () => {
  await vm.runInContext(source.replace('await init();', expose), context, {filename: file});
  const api = context.testSettings;
  for (const [value, expected] of [[undefined,4000],['',0],[0,0],[4000,4000],['5000',5000],[6000,6000],[7351,7351]]) {
    const raw = value === undefined ? {} : {protectedSecretBudgetChars:value};
    const normalized = api.sanitizeSettings(raw);
    assert.equal(normalized.protectedSecretBudgetChars, expected);
    assert.equal(await api.updateSettings(normalized), true);
    assert.equal(JSON.parse(stored).protectedSecretBudgetChars, expected);
    await api.loadSettings();
    assert.equal(api.getSettings().protectedSecretBudgetChars, expected);
  }
  const legacy = api.sanitizeSettings({memoryDeliveryBudgets:{protected_secret:1200}});
  assert.equal(legacy.protectedSecretBudgetChars,4000);
  assert.equal(Object.hasOwn(legacy.memoryDeliveryBudgets,'protected_secret'),false);
  for (const lang of ['ko','en','ja']) {
    await api.updateSettings({uiLanguage:lang});
    for (const key of ['settings.label.protectedSecretBudgetChars','settings.hint.protectedSecretBudgetChars']) {
      assert.notEqual(api.t(key), key);
    }
  }
  assert.equal(writes.length,10);
  console.log('PASS: production normalization, persistent save/reload, legacy value isolation and three localizations; synthetic Host storage/config boundary.');
})().catch(error => { console.error(error); process.exitCode=1; });
