import type { Field, Operation } from './catalog.js';

export type FormValues = Record<string, string>;
const forbiddenKeys = new Set(['__proto__', 'constructor', 'prototype']);
function parseJson(value: string, array: boolean, label: string): unknown {
  let result: unknown;
  try { result = JSON.parse(value, (key, v: unknown) => { if (forbiddenKeys.has(key)) throw Error(); return v; }); }
  catch { throw new Error(`${label}: enter valid JSON without prototype keys.`); }
  if (array ? !Array.isArray(result) : !result || typeof result !== 'object' || Array.isArray(result))
    throw new Error(`${label}: expected ${array ? 'a list' : 'an object'}.`);
  return result;
}
export function fieldValue(f: Field, raw: string): unknown {
  if (raw.length > 100_000) throw new Error(`${f.label} is too large.`);
  const text = raw.trim();
  if (!text) {
    if (f.required) throw new Error(`${f.label} is required.`);
    return undefined;
  }
  if (f.type === 'boolean') {
    if (!['true', 'false'].includes(text)) throw new Error(`${f.label}: choose Yes or No.`);
    return text === 'true';
  }
  if (f.type === 'integer' || f.type === 'number') {
    const n = Number(text);
    if (!Number.isFinite(n) || (f.type === 'integer' && !Number.isInteger(n))) throw new Error(`${f.label}: enter a valid number.`);
    if (f.min !== undefined && n < f.min || f.max !== undefined && n > f.max) throw new Error(`${f.label}: use ${f.min ?? 'no minimum'} to ${f.max ?? 'no maximum'}.`);
    return n;
  }
  if (f.type === 'json-array' || f.type === 'json-object') return parseJson(text, f.type === 'json-array', f.label);
  if (f.type === 'contact-card') {
    const lines = text.split('\n').map(x => x.trim());
    if (!lines[0] || !/^\+?\d{7,15}$/.test(lines[1] || '')) throw new Error('Contact card: enter a name, phone with country code, and optional organization on separate lines.');
    return { fullName: lines[0], phone: lines[1], organization: lines[2] || '' };
  }
  if (['lines', 'reply-buttons', 'list-rows'].includes(f.type)) {
    const lines = text.split('\n').map(x => x.trim()).filter(Boolean);
    if (lines.length > 100) throw new Error(`${f.label}: use at most 100 entries.`);
    if (f.required && !lines.length) throw new Error(`${f.label} is required.`);
    if (f.type === 'reply-buttons') {
      if (lines.length > 3) throw new Error('Use up to three reply buttons.');
      return lines.map((displayText, i) => ({type:'reply',displayText,id:`reply_${i+1}`}));
    }
    if (f.type === 'list-rows') return [{title:'Options',rows:lines.map((title,i)=>({title,rowId:`option_${i+1}`}))}];
    return lines;
  }
  if (f.options && !f.options.map(String).includes(text)) throw new Error(`${f.label}: choose a listed option.`);
  if (['url', 'imgUrl', 'imageUrl', 'videoUrl', 'webhookUrl'].includes(f.name)) {
    let u: URL;
    try { u = new URL(text); } catch { throw new Error(`${f.label}: enter an https URL.`); }
    if (!['http:', 'https:'].includes(u.protocol) || u.username || u.password) throw new Error(`${f.label}: use an http(s) URL without embedded credentials.`);
  }
  return raw;
}
export function buildRequest(op: Operation, values: FormValues, instanceId: string): { path:string; body?:Record<string,unknown> } {
  const body: Record<string,unknown> = Object.create(null);
  let path = op.path.replace(':instanceId', encodeURIComponent(instanceId));
  const query = new URLSearchParams();
  for (const f of op.fields) {
    const value = fieldValue(f, values[f.name] ?? '');
    if (value === undefined) continue;
    if (f.in === 'path') path = path.replace(`:${f.name}`, encodeURIComponent(String(value).trim()));
    else if (f.in === 'query') query.set(f.name,String(value));
    else body[f.name] = value;
  }
  if (/:[A-Za-z]/.test(path)) throw new Error('Choose a sender and supply the required identifier.');
  if (op.path === '/send/poll') {
    const count = Array.isArray(body.options) ? body.options.length : 0;
    if (count < 2 || count > 12 || Number(body.maxAnswer) > count) throw new Error('A poll needs 2–12 options; allowed answers cannot exceed its options.');
  }
  if (query.size) path += '?' + query;
  return {path,body:op.method === 'GET' || op.method === 'DELETE' ? undefined : body};
}
export function collectFields(form: HTMLFormElement, fields: Field[]): FormValues {
  const fd = new FormData(form), values: FormValues = {};
  for (const f of fields) values[f.name] = String(fd.get(f.name) ?? '');
  return values;
}
export function safeLink(value: unknown): string | null {
  if (typeof value !== 'string') return null;
  try { const u=new URL(value); return u.protocol==='https:' && !u.username && !u.password ? u.href : null; } catch {return null;}
}
export function redact(value: unknown, secrets: string[] = [], depth = 0): unknown {
  if (depth > 12) return '[nested data]';
  if (typeof value === 'string') {
    let text=value.slice(0,60_000);
    for (const secret of secrets) if (secret.length > 3) text=text.split(secret).join('[redacted]');
    return text.replace(/(apikey|token|secret|password)(["'\s]*[:=]["'\s]*)([^\s,}"']+)/gi,'$1$2[redacted]');
  }
  if (Array.isArray(value)) return value.slice(0,500).map(v=>redact(v,secrets,depth+1));
  if (value && typeof value==='object') return Object.fromEntries(Object.entries(value).slice(0,100).map(([k,v])=>[k,/token|password|secret|authorization|api.?key|qrcode|passkey|proxy|webhook/i.test(k)?'[redacted]':redact(v,secrets,depth+1)]));
  return value;
}
