import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
const source = await readFile(new URL("plugin.js", import.meta.url));
const driver = await import("data:text/javascript;base64," + source.toString("base64"));
const photo = "https://example.com/outfit.jpg";
const ctx = { baseUrl: "https://www.aigasapi.com", apiKey: "fixture", publicTaskId: "task_local" };
const open = driver.protocols.openai_video;
function decode(model, fields) {
  return open.decodeRequest({ model, body: { kind: "json", value: { model, ...fields } } });
}
for (const model of driver.meta.models) {
  const input = { content: [{ type: "text", text: "Show the outfit" }, { type: "image_url", image_url: { url: photo }, role: "reference_image" }], ratio: "9:16", seconds: "5" };
  const job = decode(model, input);
  const submit = driver.buildSubmitRequest({ ...ctx, model, requestBody: job.requestBody });
  assert.equal(submit.body.out_task_id, "o1key-task_local");
  assert.equal(submit.body.model, model);
  assert.equal(submit.body.aspect_ratio, "9:16");
  assert.deepEqual(submit.body.references, [{ type: "image", url: photo, role: "reference" }]);
  assert.deepEqual(driver.extractUsage({ model, requestBody: job.requestBody }), { seconds: 5 });
  for (const duration of [-1, 0, 4.5, 31, true, ""]) assert.throws(() => decode(model, { prompt: "Test", duration }));
  assert.throws(() => decode(model, { ...input, duration: 8 }));
  const result = { status: "SUCCESS" };
  assert.deepEqual(driver.extractUsageOnComplete({ model }, result, { billing_usage: { unit: "second", billable_seconds: 5 }, duration: 5.056 }), { seconds: 5 });
  assert.deepEqual(driver.extractUsageOnComplete({ model }, result, { billing_usage: { unit: "second", billable_seconds: 0 } }), { seconds: 0 });
  assert.deepEqual(driver.extractUsageOnComplete({ model }, result, { billing_usage: { unit: "second", billable_seconds: 99 } }), {});
}
assert.equal(driver.meta.models.length, 11);
assert.throws(() => decode("sd-2.5-flash-720p", { prompt: "Test" }));
assert.throws(() => decode("sd-2-mini-480", { prompt: "Test", duration: 4 }));
assert.throws(() => decode("sd-2-vip-720", { prompt: "Test", duration: 4 }));
assert.equal(decode("sd-2-fast-480", { prompt: "Test", duration: 4 }).requestBody.duration, 4);
const frame = { type: "image", role: "first_frame", url: photo };
assert.throws(() => decode("sd-2.5-720P", { prompt: "Test", references: [frame] }));
assert.equal(decode("sd-2-fast-720", { prompt: "Test", references: [frame, { ...frame, role: "last_frame" }] }).requestBody.references.length, 2);
assert.throws(() => decode("sd-2-fast-720", { prompt: "Test", references: [{ ...frame, role: "last_frame" }] }));
assert.throws(() => decode("sd-2-fast-720", { prompt: "Test", audios: [photo] }));
assert.throws(() => decode("sd-2.5-flash-720P", { prompt: "Test", images: Array(11).fill(photo) }));
assert.throws(() => decode("sd-2.5-480P", { prompt: "Test", images: ["asset://elsewhere"] }));
assert.equal(open.render({}, { status: "SUCCESS", task_id: "task_local", data: { metadata: { url: photo } } }).result_url, "https://api.o1key.com/v1/videos/task_local/content");
assert.deepEqual(driver.buildContentRequest({ artifactKey: "video", clientRequest: { method: "HEAD" }, data: { metadata: { url: "https://oss.example.com/clip.mp4" } } }),
  { url: "https://oss.example.com/clip.mp4", method: "HEAD", credentialless: true });
assert.equal(driver.parseTaskResult({}, { status: "failed", error: { message: "media unavailable" } }).reason, "media unavailable");
assert.equal(driver.parseTaskResult({}, { status: "new_state" }).status, "UNKNOWN");
console.log("Passed 11-model request, duration, billing, media-role, status and download checks.");
