import { test } from "node:test";
import assert from "node:assert/strict";
import { GET, POST, DELETE } from "../src/app/api/[...path]/route.ts";
function request(path, method = "GET", body) {
  const url = new URL("http://localhost:3000/api/" + path);
  return Object.assign(new Request(url, { method, body }), { nextUrl: url });
}
test("proxy forwards query, method and body without caching local records", async (t) => {
  const original = globalThis.fetch;
  const previous = process.env.GO_API_URL;
  t.after(() => {
    globalThis.fetch = original;
    if (previous === undefined) delete process.env.GO_API_URL;
    else process.env.GO_API_URL = previous;
  });
  process.env.GO_API_URL = "http://fixture.invalid:8080";
  const calls = [];
  globalThis.fetch = async (url, init) => {
    calls.push({ url: String(url), init });
    return Response.json({ ok: true }, { status: 201 });
  };
  const response = await POST(
    request("deliveries", "POST", '{"gross_aud":40}'),
    { params: Promise.resolve({ path: ["deliveries"] }) },
  );
  assert.equal(response.status, 201);
  assert.equal(response.headers.get("cache-control"), "no-store");
  assert.equal(calls[0].url, "http://fixture.invalid:8080/api/deliveries");
  assert.equal(calls[0].init.body, '{"gross_aud":40}');
  assert.equal(calls[0].init.method, "POST");
  assert.equal(calls[0].init.cache, "no-store");
  await GET(request("maps/venues?lat=-37.81&lon=144.96"), {
    params: Promise.resolve({ path: ["maps", "venues"] }),
  });
  assert.equal(
    calls[1].url,
    "http://fixture.invalid:8080/api/maps/venues?lat=-37.81&lon=144.96",
  );
  await DELETE(request("maps/job", "DELETE"), {
    params: Promise.resolve({ path: ["maps", "job"] }),
  });
  assert.equal(calls[2].init.method, "DELETE");
  const unknown = await GET(request("other"), {
    params: Promise.resolve({ path: ["other"] }),
  });
  assert.equal(unknown.status, 404);
  assert.equal(calls.length, 3);
});
test("backend failures produce an actionable 502 without network access", async (t) => {
  const original = globalThis.fetch;
  t.after(() => (globalThis.fetch = original));
  globalThis.fetch = async () => {
    throw Error("fixture unavailable");
  };
  const result = await GET(request("health"), {
    params: Promise.resolve({ path: ["health"] }),
  });
  assert.equal(result.status, 502);
  assert.match((await result.json()).error, /Go backend unavailable/);
});
