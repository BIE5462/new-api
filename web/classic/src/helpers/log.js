/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

export function getLogOther(otherStr) {
  if (otherStr === undefined || otherStr === null || otherStr === '') {
    return {};
  }
  if (typeof otherStr === 'object') {
    return otherStr;
  }
  try {
    return JSON.parse(otherStr);
  } catch (e) {
    console.error(`Failed to parse record.other: "${otherStr}".`, e);
    return null;
  }
}

const QUOTA_ADJUSTMENT_ACTIONS = new Set([
  'user.quota_add',
  'user.quota_subtract',
  'user.quota_override',
]);

// Return the structured quota adjustment descriptor, including legacy records
// that predate the `other.op` audit payload.
export function getQuotaAdjustment(record) {
  if (!record) {
    return null;
  }

  const other = getLogOther(record.other);
  const action = other?.op?.action;
  if (QUOTA_ADJUSTMENT_ACTIONS.has(action)) {
    return {
      action,
      params: other.op.params || {},
      content: record.content || '',
    };
  }

  const content = String(record.content || '');
  if (
    /(?:管理员(?:\([^)]*\))?增加用户额度|Increased user quota by)/i.test(
      content,
    )
  ) {
    return { action: 'user.quota_add', params: {}, content };
  }
  if (
    /(?:管理员(?:\([^)]*\))?减少用户额度|Decreased user quota by)/i.test(
      content,
    )
  ) {
    return { action: 'user.quota_subtract', params: {}, content };
  }
  if (
    /(?:管理员(?:\([^)]*\))?覆盖用户额度|Overrode user quota from)/i.test(
      content,
    )
  ) {
    return { action: 'user.quota_override', params: {}, content };
  }

  return null;
}
