import { expect, test } from "bun:test";
import { parseSubmitResponse, parseTaskResult, extractUsageOnComplete } from "./plugin.js";

function task(model, action, request) {
  const ctx = {
    model,
    action,
    taskId: "test",
    officialRequest: request,
    baseUrl: "https://api-beijing.klingai.com",
  };
  ctx.state = parseSubmitResponse(ctx, { body: { data: { id: ctx.taskId } } }).state;
  return ctx;
}
function modern(ctx, extra) {
  return {
    data: [
      {
        id: ctx.taskId,
        status: "succeeded",
        outputs: [{ type: "video", duration: "5.041" }],
        ...extra,
      },
    ],
  };
}
test("4K HDR applies to output once and image Elements settle without video input", () => {
  const ctx = task("kling-4.0", "kling40Omni", {
    settings: { resolution: "4k", duration: 5, dynamic_range: "hdr" },
    contents: [{ type: "element", id: "subject", element_id: "1" }],
  });
  expect(
    extractUsageOnComplete(ctx, null, modern(ctx, { usage: { output_duration: "5" } })),
  ).toEqual({ billed_seconds: 5, video_input: "none", hdr_seconds: 5 });
  expect(
    extractUsageOnComplete(
      ctx,
      null,
      modern(ctx, { usage: { output_duration: "5", input_duration: "6.25" } }),
    ),
  ).toEqual({ billed_seconds: 11.25, video_input: "video", hdr_seconds: 5 });
});
test("a missing reference duration stays pending until final usage arrives", () => {
  const ctx = task("kling-4.0-flash", "kling40Omni", {
    settings: { duration: 5 },
    contents: [{ type: "feature_video" }],
  });
  expect(parseTaskResult(ctx, modern(ctx, { usage: { output_duration: "5" } })).status).toBe(
    "IN_PROGRESS",
  );
  expect(
    parseTaskResult(ctx, modern(ctx, { usage: { output_duration: "5", input_duration: "6" } }))
      .status,
  ).toBe("SUCCESS");
});
test("timestamps are normalized even at the duration limit", () => {
  const ctx = task("kling-4.0-flash", "kling40Text", { settings: {}, contents: [] });
  const response = modern(ctx, { outputs: [{ type: "video", duration: "20.041" }] });
  expect(extractUsageOnComplete(ctx, null, response)).toEqual({
    billed_seconds: 20,
    video_input: "none",
  });
});
test("domestic listed amounts and mixed payments settle once, preserving explicit zeros", () => {
  const ctx = task("kling-v3", "textGenerate", { mode: "std", duration: "5", sound: "off" });
  const row = {
    task_id: "test",
    task_status: "succeed",
    task_result: { videos: [{ duration: "5.041" }] },
  };
  const units = (receipt) =>
    extractUsageOnComplete(ctx, null, { data: { ...row, ...receipt } }).units;
  expect(
    units({
      final_unit_deduction: "0.5",
      final_balance_deduction: { quota: "0.2", list_price: "2.5" },
    }),
  ).toBe(3);
  expect(
    units({ final_unit_deduction: "0", final_balance_deduction: { quota: "0", list_price: "0" } }),
  ).toBe(0);
  const newctx = task("kling-3.0-omni", "omniVideo30", {
    settings: { resolution: "1080p", audio: "native", duration: 5 },
  });
  const check = (lines) =>
    extractUsageOnComplete(newctx, null, modern(newctx, { billing: lines })).units;
  expect(check([{ charge_type: "cash", currency: "USD", amount: "0.50" }])).toBe(5);
  expect(check([{ charge_type: "cash", currency: "USD", amount: "0", list_price: "0" }])).toBe(0);
  expect(
    check([
      { charge_type: "unit", amount: 2 },
      { charge_type: "cash", currency: "CNY", amount: 0.1, list_price: 3 },
    ]),
  ).toBe(5);
});
