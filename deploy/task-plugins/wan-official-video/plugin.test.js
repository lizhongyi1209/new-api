import { expect, test } from "bun:test";
import {
  parseSubmitResponse,
  parseTaskResult,
  extractUsage,
  extractUsageOnComplete,
} from "./plugin.js";

function task(reference = false) {
  const ctx = {
    model: "wan3.0-video",
    taskId: "wan-test",
    officialRequest: {
      input: { media: reference ? [{ type: "reference_video" }] : [] },
      parameters: { duration: -1, resolution: "1080P" },
    },
  };
  ctx.state = parseSubmitResponse(ctx, { body: { output: { task_id: ctx.taskId } } }).state;
  return ctx;
}
function success(usage) {
  return {
    output: {
      task_id: "wan-test",
      task_status: "SUCCEEDED",
      video_url: "https://example.invalid/video.mp4",
    },
    usage,
  };
}
test("successful assets wait for missing or invalid usage and resume when it arrives", () => {
  const ctx = task(true);
  for (const usage of [
    undefined,
    {},
    { output_video_duration: 5 },
    { output_video_duration: 5, input_video_duration: 16 },
    { output_video_duration: 5, input_video_duration: 3, SR: 1440 },
  ]) {
    const result = parseTaskResult(ctx, success(usage));
    expect(result.status).toBe("IN_PROGRESS");
    expect(result.url).toBeUndefined();
  }
  const body = success({ output_video_duration: 5, input_video_duration: 2.75, SR: 720 });
  expect(parseTaskResult(ctx, body).status).toBe("SUCCESS");
  expect(extractUsageOnComplete(ctx, null, body)).toEqual({
    billed_seconds: 7.75,
    resolution: "720P",
  });
});
test("zero usage and failure remain terminal without retaining a reservation", () => {
  const ctx = task();
  const body = success({ output_video_duration: 0, input_video_duration: 0, SR: 1080 });
  expect(parseTaskResult(ctx, body).status).toBe("SUCCESS");
  expect(extractUsageOnComplete(ctx, null, body).billed_seconds).toBe(0);
  expect(
    parseTaskResult(ctx, { output: { task_id: ctx.taskId, task_status: "FAILED" } }).status,
  ).toBe("FAILURE");
});
