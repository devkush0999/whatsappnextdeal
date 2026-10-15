import { test } from "node:test";
import assert from "node:assert/strict";
import { ApiClient, ApiError } from "./api.ts";
test("native fetch keeps the global receiver when called through the client", async () => {
  const nativeLikeFetch = async function (this: typeof globalThis) {
    assert.equal(this, globalThis);
    return new Response('{"ok":true}', {
      headers: { "content-type": "application/json" },
    });
  };
  const client = new ApiClient(nativeLikeFetch as typeof fetch);
  assert.deepEqual(await client.request("/instance/all", "key"), { ok: true });
});
test("sender credentials go only in the header; JSON calls never retry", async () => {
  let calls = 0;
  const client = new ApiClient((async (path: any, init: any) => {
    calls++;
    assert.equal(path, "/send/text");
    assert.equal(init.headers.apikey, "sender-key");
    assert.equal(init.credentials, "omit");
    assert.equal(init.redirect, "error");
    return new Response('{"error":"failed"}', {
      status: 503,
      headers: { "content-type": "application/json" },
    });
  }) as typeof fetch);
  await assert.rejects(
    client.request("/send/text", "sender-key", "POST", { text: "fixture" }),
    (e: ApiError) => e.status === 503 && e.uncertain,
  );
  assert.equal(calls, 1);
});
test("HTML fallback and oversized responses fail instead of becoming successful data", async () => {
  const html = new ApiClient(
    (async () =>
      new Response("<html/>", {
        headers: { "content-type": "text/html" },
      })) as typeof fetch,
  );
  await assert.rejects(html.request("/user/contacts", "key"));
  const large = new ApiClient(
    (async () =>
      new Response(" ".repeat(2_000_001), {
        headers: { "content-type": "application/json" },
      })) as typeof fetch,
  );
  await assert.rejects(large.request("/user/contacts", "key"), /too large/);
});
test("locking invalidates a late response even if the transport ignores abort", async () => {
  let release: (v: Response) => void = () => {};
  const client = new ApiClient(
    (() =>
      new Promise<Response>((resolve) => {
        release = resolve;
      })) as typeof fetch,
  );
  const pending = client.request("/instance/all", "key");
  client.cancelAll();
  release(
    new Response("[]", { headers: { "content-type": "application/json" } }),
  );
  await assert.rejects(pending, /locked/);
});
test("foreign destinations are refused before fetch", async () => {
  let calls = 0;
  const c = new ApiClient((async () => {
    calls++;
    return new Response();
  }) as typeof fetch);
  await assert.rejects(c.request("//evil.example/instance/all", "key"));
  assert.equal(calls, 0);
});
test("multipart keeps the browser boundary and does not JSON-encode a file", async () => {
  const body = new FormData();
  body.append("file", new File(["test"], "test.txt"));
  const client = new ApiClient((async (_path: any, init: any) => {
    assert.equal(init.body, body);
    assert.equal(init.headers["Content-Type"], undefined);
    return new Response("{}", {
      headers: { "content-type": "application/json" },
    });
  }) as typeof fetch);
  await client.request("/send/media", "key", "POST", body);
});
