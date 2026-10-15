import { test } from "node:test";
import assert from "node:assert/strict";
import { instancesFrom, escapeHtml, qrImage, maskedNumber } from "./model.ts";
test("normalizes API wrappers and rejects invalid connection state", () => {
  assert.equal(
    instancesFrom({
      data: [{ id: "a", name: "Sender", token: "secret", connected: false }],
    })[0].connected,
    false,
  );
  assert.throws(() =>
    instancesFrom({ data: [{ id: "a", token: "k", connected: "false" }] }),
  );
});
test("escapes untrusted instance names", () =>
  assert.equal(
    escapeHtml('<img onerror="x">'),
    "&lt;img onerror=&quot;x&quot;&gt;",
  ));
test("QR image accepts PNG base64 only; never remote URLs or SVG", () => {
  assert.equal(qrImage("https://example.com/a.png"), null);
  assert.equal(qrImage("data:image/svg+xml;base64,abc"), null);
  assert.ok(qrImage("A".repeat(64)));
});
test("masks linked numbers", () =>
  assert.equal(maskedNumber("919696340330:2@s.whatsapp.net"), "•••• 0330"));
