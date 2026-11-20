const resolutions = ["480P", "720P", "1080P"];
export const meta = {
  apiVersion: 1,
  key: "wan-official-video",
  name: "通义万相官方协议视频",
  version: "1.0.0",
  icon: "Alibaba.Color",
  author: { name: "O1Key" },
  fetchMode: "per_task",
  auth: "api_key",
  models: ["wan3.0-video", "wan3.0-video-prime"],
  usageSchema: {
    billed_seconds: { type: "number", unit: "second", description: { en: "Input + output seconds", zh: "输入与输出计费秒数" } },
    resolution: { enum: resolutions },
  },
  usageExamples: [
    { label: "5 秒输出", facts: { billed_seconds: 5, resolution: "720P" } },
    { label: "3 秒参考视频 + 5 秒输出", facts: { billed_seconds: 8, resolution: "1080P" } },
  ],
};
function base(ctx) {
  return ctx.baseUrl.replace(/\/+$/, "");
}
export function buildSubmitRequest(ctx) {
  if (!ctx.officialRequest) throw new Error("This plugin requires the configured DashScope video channel");
  return {
    method: "POST",
    url: base(ctx) + "/api/v1/services/aigc/video-generation/video-synthesis",
    body: ctx.officialRequest,
    headers: { "Content-Type": "application/json", Authorization: "Bearer " + ctx.apiKey, "X-DashScope-Async": "enable" },
  };
}
function reserve(ctx) {
  const body = ctx.officialRequest,
    p = body.parameters || {},
    media = (body.input || {}).media || [];
  const reference = media.some((x) => x.type === "reference_video");
  const seconds = p.duration === -1 ? 30 : Math.min(30, Number(p.duration || 5) + (reference ? 15 : 0));
  return { reference: reference, seconds: seconds, resolution: String(p.resolution || "720P").toUpperCase() };
}
export function extractUsage(ctx) {
  if (ctx.usagePurpose === "billing_ratios") return null;
  const reservation = reserve(ctx);
  return { billed_seconds: reservation.seconds, resolution: reservation.resolution };
}
export function parseSubmitResponse(ctx, resp) {
  const body = resp.body || {},
    output = body.output || {};
  if (body.code || !output.task_id) throw new Error(body.message || "Missing DashScope task ID");
  return { taskId: output.task_id, taskData: body, state: { reference: reserve(ctx).reference } };
}
export function buildQueryRequest(ctx) {
  return { method: "GET", url: base(ctx) + "/api/v1/tasks/" + encodeURIComponent(ctx.taskId), headers: { Authorization: "Bearer " + ctx.apiKey } };
}
export function parseTaskResult(ctx, body) {
  const output = body.output || {};
  if (output.task_id && output.task_id !== ctx.taskId) throw new Error("DashScope returned a different task");
  const states = {
    PENDING: "QUEUED",
    RUNNING: "IN_PROGRESS",
    SUCCEEDED: "SUCCESS",
    FAILED: "FAILURE",
    CANCELED: "FAILURE",
    CANCELLED: "FAILURE",
    UNKNOWN: "FAILURE",
  };
  if (!states[output.task_status]) throw new Error(body.message || "Unknown DashScope task status");
  return { taskId: ctx.taskId, status: states[output.task_status], url: output.video_url || "", reason: body.message || output.message || "" };
}
function duration(value, max) {
  if (typeof value !== "number" && (typeof value !== "string" || !value.trim())) return null;
  const n = Number(value);
  return Number.isFinite(n) && n >= 0 && n <= max ? n : null;
}
export function extractUsageOnComplete(ctx, _result, body) {
  const u = body.usage || {},
    output = duration(u.output_video_duration, 30);
  const input = u.input_video_duration == null && !(ctx.state || {}).reference ? 0 : duration(u.input_video_duration, 15);
  if (output === null || input === null || output + input > 30) return null;
  const facts = { billed_seconds: output + input };
  if (u.SR != null) {
    const res = String(u.SR).toUpperCase().replace(/P$/, "") + "P";
    if (resolutions.indexOf(res) < 0) return null;
    facts.resolution = res;
  }
  return facts;
}
function artifact(ctx) {
  return ((ctx.data || {}).output || {}).video_url || "";
}
export function listArtifacts(task) {
  return task.status === "SUCCESS" && artifact(task) ? [{ key: "video", type: "video" }] : [];
}
export function buildContentRequest(ctx) {
  const url = artifact(ctx);
  if (ctx.artifactKey !== "video" || !url) throw new Error("artifact_not_found");
  return { url: url, method: ctx.clientRequest.method, credentialless: true };
}
