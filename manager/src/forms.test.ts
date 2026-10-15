import { test } from "node:test";
import assert from "node:assert/strict";
import { buildRequest, fieldValue, redact, safeLink } from "./forms.ts";
import type { Operation, Field } from "./catalog.ts";
const op = (fields: Field[], extra: Partial<Operation> = {}): Operation => ({
  id: "fixture",
  method: "POST",
  path: "/send/text",
  title: "Fixture",
  category: "compose",
  auth: "instance",
  effect: "send",
  fields,
  ...extra,
});
test("coordinates preserve zero, reject nonfinite/out-of-range and integer coercion", () => {
  const f: Field = {
    name: "latitude",
    label: "Latitude",
    type: "number",
    required: true,
    min: -90,
    max: 90,
  };
  assert.equal(fieldValue(f, "0"), 0);
  assert.throws(() => fieldValue(f, "91"));
  assert.throws(() => fieldValue(f, "NaN"));
  assert.throws(() => fieldValue({ ...f, type: "integer" }, "1.5"));
});
test("false and optional omissions are different, dropdowns reject injected values", () => {
  assert.equal(
    fieldValue({ name: "flag", label: "Flag", type: "boolean" }, "false"),
    false,
  );
  assert.equal(
    fieldValue({ name: "flag", label: "Flag", type: "boolean" }, ""),
    undefined,
  );
  assert.throws(() =>
    fieldValue(
      { name: "type", label: "Type", type: "string", options: ["image"] },
      "script",
    ),
  );
});
test("path/query values are encoded and never become arbitrary destinations", () => {
  const value = buildRequest(
    op(
      [
        {
          name: "pollId",
          label: "Poll",
          type: "string",
          required: true,
          in: "path",
        },
        {
          name: "limit",
          label: "Limit",
          type: "integer",
          in: "query",
          min: 1,
          max: 100,
        },
      ],
      { path: "/polls/:pollId/results", method: "GET" },
    ),
    { pollId: "abc/def?secret", limit: "20" },
    "id",
  );
  assert.equal(value.path, "/polls/abc%2Fdef%3Fsecret/results?limit=20");
  assert.equal(value.body, undefined);
  assert.throws(() =>
    buildRequest(op([], { path: "/polls/:pollId/results" }), {}, "id"),
  );
});
test("reply and list builders produce actual gateway DTOs", () => {
  assert.deepEqual(
    fieldValue(
      { name: "buttons", label: "Buttons", type: "reply-buttons" },
      "Call me\nSend details",
    ),
    [
      { type: "reply", displayText: "Call me", id: "reply_1" },
      { type: "reply", displayText: "Send details", id: "reply_2" },
    ],
  );
  assert.throws(() =>
    fieldValue(
      { name: "buttons", label: "Buttons", type: "reply-buttons" },
      "a\nb\nc\nd",
    ),
  );
  assert.deepEqual(
    fieldValue(
      { name: "sections", label: "Rows", type: "list-rows" },
      "Option one",
    ),
    [{ title: "Options", rows: [{ title: "Option one", rowId: "option_1" }] }],
  );
  assert.deepEqual(
    fieldValue(
      { name: "vcard", label: "Contact", type: "contact-card" },
      "Test Person\n919999999999\nNextDeal",
    ),
    {
      fullName: "Test Person",
      phone: "919999999999",
      organization: "NextDeal",
    },
  );
});
test("nested JSON is typed and prototype keys never pass through", () => {
  assert.throws(() =>
    fieldValue({ name: "cards", label: "Cards", type: "json-array" }, "{}"),
  );
  assert.throws(() =>
    fieldValue(
      { name: "quoted", label: "Quoted", type: "json-object" },
      '{"__proto__":{}}',
    ),
  );
});
test("interactive action buttons keep custom fields and reject mixed reply/action payloads", () => {
  const f: Field = { name: "buttons", label: "Buttons", type: "reply-buttons" };
  assert.deepEqual(
    fieldValue(
      f,
      '[{"type":"url","displayText":"Explore","url":"https://example.com"}]',
    ),
    [{ type: "url", displayText: "Explore", url: "https://example.com" }],
  );
  assert.throws(
    () => fieldValue(f, '[{"type":"reply"},{"type":"url"}]'),
    /without mixing/,
  );
  assert.throws(
    () => fieldValue(f, '[{"type":"pix"},{"type":"copy"}]'),
    /alone/,
  );
});
test("poll answer count is validated against options", () => {
  const fields: Field[] = [
    { name: "options", label: "Options", type: "lines", required: true },
    {
      name: "maxAnswer",
      label: "Maximum answers",
      type: "integer",
      required: true,
      min: 1,
    },
  ];
  assert.throws(() =>
    buildRequest(
      op(fields, { path: "/send/poll" }),
      { options: "One\nTwo", maxAnswer: "3" },
      "id",
    ),
  );
  assert.deepEqual(
    buildRequest(
      op(fields, { path: "/send/poll" }),
      { options: "One\nTwo", maxAnswer: "1" },
      "id",
    ).body?.options,
    ["One", "Two"],
  );
});
test("credentials and opaque webhook/proxy strings are redacted in response views", () => {
  const r = redact(
    {
      token: "secret",
      data: [
        {
          text: "sender-secret",
          webhook: "https://private.example/?key=secret",
          api_key: "x",
        },
      ],
    },
    ["sender-secret"],
  ) as any;
  assert.equal(r.token, "[redacted]");
  assert.equal(r.data[0].text, "[redacted]");
  assert.equal(r.data[0].webhook, "[redacted]");
});
test("passkey links reject script, credentials and HTTP URLs", () => {
  assert.equal(safeLink("javascript:alert(1)"), null);
  assert.equal(safeLink("https://user:pass@example.com"), null);
  assert.equal(safeLink("http://example.com"), null);
  assert.equal(
    safeLink("https://example.com/step"),
    "https://example.com/step",
  );
});
test("file attachments use the gateway multipart contract and safe review metadata", () => {
  const fields: Field[] = [
    { name: "number", label: "Recipient", type: "string", required: true },
    { name: "type", label: "Media", type: "string", required: true },
    { name: "url", label: "URL", type: "string", required: true },
  ];
  const file = new File(["fixture"], "sample.pdf", { type: "application/pdf" });
  const r = buildRequest(
    op(fields, { path: "/send/media" }),
    { number: "919999999999", type: "document" },
    "id",
    file,
  );
  assert.ok(r.body instanceof FormData);
  assert.equal(r.body.get("number"), "919999999999");
  assert.equal((r.body.get("file") as File).name, "sample.pdf");
  assert.deepEqual(r.review.file, {
    name: "sample.pdf",
    bytes: 7,
    type: "application/pdf",
  });
  assert.throws(
    () =>
      buildRequest(
        op(fields, { path: "/send/media" }),
        { number: "919999999999", type: "image" },
        "id",
        file,
      ),
    /does not match/,
  );
  assert.throws(() => buildRequest(op([]), {}, "id", file), /not supported/);
});
