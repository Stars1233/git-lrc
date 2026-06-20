import test from 'node:test';
import assert from 'node:assert/strict';

import { hasIcon, ICON_ALIASES, ICON_SELECTION_GUIDANCE, renderIcon } from './icons.js';

function html(strings, ...values) {
  return strings.reduce((accumulator, part, index) => {
    let value = '';
    if (index < values.length) {
      const current = values[index];
      if (Array.isArray(current)) {
        value = current.join('');
      } else if (current === null || current === undefined) {
        value = '';
      } else {
        value = String(current);
      }
    }
    return accumulator + part + value;
  }, '');
}

test('semantic action aliases resolve for shared UI controls', () => {
  assert.equal(ICON_ALIASES.sendToAgent, 'send');
  assert.equal(ICON_ALIASES.copyLogs, 'copy');
  assert.equal(ICON_ALIASES.filesTab, 'folder');
  assert.equal(hasIcon('sendToAgent'), true);
  assert.equal(hasIcon('claudeBrand'), true);
});

test('selection guidance keeps action buttons semantic-first', () => {
  assert.match(ICON_SELECTION_GUIDANCE.semanticFirst, /action/i);
  assert.match(ICON_SELECTION_GUIDANCE.brandForIdentity, /identity/i);
  assert.match(ICON_SELECTION_GUIDANCE.noForcedLogos, /semantic icon plus text/i);
});

test('renderIcon returns svg for semantic actions and monogram for approved brand identity', () => {
  const sendMarkup = renderIcon(html, 'sendToAgent');
  const brandMarkup = renderIcon(html, 'claudeBrand', { decorative: false, label: 'Claude' });

  assert.match(sendMarkup, /<svg/);
  assert.match(sendMarkup, /stroke="currentColor"/);
  assert.match(brandMarkup, /icon-brand-monogram/);
  assert.match(brandMarkup, />\s*C\s*</);
});