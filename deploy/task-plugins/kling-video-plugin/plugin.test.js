import { expect, test } from "bun:test";
import {
  meta,
  buildSubmitRequest,
  buildQueryRequest,
  parseSubmitResponse,
  parseTaskResult,
  extractUsageOnComplete,
} from "./plugin.js";

globalThis.utils = { unixNow: () => 0, jwtSignHS256: () => "unused" };

function task(model, action, request) {
  const ctx = {
    model,
    action,
    taskId: "950079555921253320",
    officialRequest: request,
    apiKey: "sk-upstream",
    baseUrl: "https://model.service-inference.ai/",
  };
  ctx.state = parseSubmitResponse(ctx, { body: { code: 0, data: { id: ctx.taskId, status: "submitted" } } }).state;
  return ctx;
}
function done(ctx, extra) {
  return {
    code: 0,
    data: [{ id: ctx.taskId, status: "succeeded", outputs: [{ type: "video", url: "https://cdn/x.mp4", duration: "5.04" }], ...extra }],
  };
}

test("serves the four unified-API models", () => {
  expect(meta.key).toBe("kling-video-plugin");
  expect(meta.models.sort()).toEqual(["kling-3.0", "kling-3.0-omni", "kling-4.0", "kling-4.0-flash"]);
});

test("submits to the unified path with the Bearer key and queries /tasks", () => {
  const ctx = task("kling-3.0", "kling40Image", { contents: [{ type: "first_frame", url: "https://x/f.png" }] });
  const submit = buildSubmitRequest(ctx);
  expect(submit.url).toBe("https://model.service-inference.ai/image-to-video/kling-3.0");
  expect(submit.headers.Authorization).toBe("Bearer sk-upstream");
  expect(buildQueryRequest(ctx).url).toBe("https://model.service-inference.ai/tasks?task_ids=950079555921253320");
  expect(buildSubmitRequest(task("kling-3.0-omni", "kling40Omni", { contents: [] })).url).toBe(
    "https://model.service-inference.ai/omni-video/kling-3.0-omni",
  );
});

test("rejects operations and credentials the upstream does not serve", () => {
  expect(() => buildSubmitRequest({ ...task("kling-3.0", "motionControl30", {}) })).toThrow("unified");
  expect(() => buildSubmitRequest({ ...task("kling-4.0", "kling40Text", {}), apiKey: "ak|sk" })).toThrow("Bearer");
  expect(() => buildSubmitRequest({ ...task("kling-4.0", "kling40Text", {}), officialRequest: null })).toThrow("kling-video-plugin");
});

test("Kling 3.0 settles credits from the rate card without billing lines", () => {
  const silent = task("kling-3.0", "kling40Text", { prompt: "p", settings: { resolution: "1080p", duration: 5 } });
  expect(extractUsageOnComplete(silent, null, done(silent))).toEqual({ units: 4, units_source: "reported" });
  const audio = task("kling-3.0", "kling40Text", { prompt: "p", settings: { audio: "native" } });
  expect(extractUsageOnComplete(audio, null, done(audio))).toEqual({ units: 4.5, units_source: "reported" });
  const omniRef = task("kling-3.0-omni", "kling40Omni", {
    contents: [{ type: "feature_video", url: "https://x/v.mp4" }],
    settings: { resolution: "4k" },
  });
  expect(extractUsageOnComplete(omniRef, null, done(omniRef))).toEqual({ units: 15, units_source: "reported" });
});

test("Kling 4.0 settles reported seconds and maps official statuses", () => {
  const ctx = task("kling-4.0-flash", "kling40Text", { prompt: "p", settings: { duration: 3 } });
  const response = done(ctx, { usage: { output_duration: "3" } });
  expect(extractUsageOnComplete(ctx, null, response)).toEqual({ billed_seconds: 3, video_input: "none" });
  expect(parseTaskResult(ctx, response)).toMatchObject({ status: "SUCCESS", url: "https://cdn/x.mp4" });
  expect(parseTaskResult(ctx, { code: 0, data: [{ id: ctx.taskId, status: "processing" }] }).status).toBe("IN_PROGRESS");
  expect(parseTaskResult(ctx, { code: 0, data: [{ id: ctx.taskId, status: "failed", message: "audit" }] })).toMatchObject({
    status: "FAILURE",
    reason: "audit",
  });
});

test("an upstream rejection surfaces its message", () => {
  const ctx = task("kling-4.0", "kling40Text", { prompt: "p" });
  expect(() => parseSubmitResponse(ctx, { body: { code: 1201, message: "bad resolution" } })).toThrow("bad resolution");
});
