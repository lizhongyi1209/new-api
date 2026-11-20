const credits = ["kling-v2-6", "kling-v3", "kling-v3-omni", "kling-3.0", "kling-3.0-omni"];
const standard = "kling-4.0";
const flash = "kling-4.0-flash";
const seconds = { type: "number", unit: "second", description: { zh: "计费秒数", en: "Billable seconds" } };
const source = {
  enum: ["estimated", "reported"],
  enumLabels: { estimated: { zh: "提交预留", en: "Reservation" }, reported: { zh: "官方实扣", en: "Reported" } },
};
const videoInput = {
  enum: ["none", "video"],
  enumLabels: { none: { zh: "无视频输入", en: "No video input" }, video: { zh: "有视频输入", en: "With video input" } },
};
function timedProfile(model, resolutions) {
  return {
    models: [model],
    schema: { billed_seconds: seconds, resolution: { enum: resolutions }, video_input: videoInput },
    examples: [
      { label: "5 秒无参考视频", facts: { billed_seconds: 5, resolution: "720p", video_input: "none" } },
      { label: "3 秒输入 + 5 秒输出", facts: { billed_seconds: 10, resolution: "720p", video_input: "video" } },
    ],
  };
}
export const meta = {
  apiVersion: 1,
  key: "kling-official-video",
  name: "可灵官方视频",
  version: "1.0.0",
  icon: "Kling.Color",
  author: { name: "O1Key" },
  fetchMode: "per_task",
  auth: "api_key",
  models: credits.concat([standard, flash]),
  usageProfiles: [
    {
      models: credits,
      schema: { units: { type: "number", unit: "credit", description: { zh: "官方扣减单位", en: "Provider credit units" } }, units_source: source },
      examples: [
        { label: "提交预留", facts: { units: 0, units_source: "estimated" } },
        { label: "官方扣减 3.6 单位", facts: { units: 3.6, units_source: "reported" } },
      ],
    },
    timedProfile(standard, ["720p", "1080p"]),
    timedProfile(flash, ["720p"]),
  ],
};
function auth(ctx) {
  const key = String(ctx.apiKey || "").trim();
  const parts = key.split("|");
  if (!key || parts.length > 2) throw new Error("Invalid Kling credentials");
  if (parts.length === 1 || !parts[1].trim()) return "Bearer " + parts[0].trim();
  const now = utils.unixNow();
  return "Bearer " + utils.jwtSignHS256({ iss: parts[0].trim(), exp: now + 1800, nbf: now - 5 }, parts[1].trim());
}
function base(ctx) {
  return ctx.baseUrl.replace(/\/+$/, "");
}
function path(action, model) {
  switch (action) {
    case "generate":
    case "image_to_video":
      return "/v1/videos/image2video";
    case "textGenerate":
    case "text_to_video":
      return "/v1/videos/text2video";
    case "motionControl":
      return "/v1/videos/motion-control";
    case "omniVideo":
      return "/v1/videos/omni-video";
    case "motionControl30":
      return "/motion-control/kling-3.0";
    case "omniVideo30":
      return "/omni-video/kling-3.0-omni";
    case "kling40Text":
      return "/text-to-video/" + model;
    case "kling40Image":
      return "/image-to-video/" + model;
    case "kling40Omni":
      return "/omni-video/" + model;
    default:
      throw new Error("Unsupported Kling operation");
  }
}
function modelOf(ctx) {
  return ctx.upstreamModel || ctx.model;
}
export function buildSubmitRequest(ctx) {
  if (!ctx.officialRequest) throw new Error("This plugin requires the configured official Kling video channel");
  return {
    method: "POST",
    url: base(ctx) + path(ctx.action, modelOf(ctx)),
    action: ctx.action,
    headers: { "Content-Type": "application/json", Accept: "application/json", Authorization: auth(ctx), "User-Agent": "kling-sdk/1.0" },
    body: ctx.officialRequest,
  };
}
function readNumber(value) {
  if (typeof value !== "number" && (typeof value !== "string" || !value.trim())) return null;
  const n = Number(value);
  return Number.isFinite(n) && n >= 0 ? n : null;
}
function limit(model) {
  return model === flash ? 20 : 30;
}
function measured(output, input) {
  return input > 0 ? Math.max(input + output, 2 * output) : output;
}
function estimate(ctx) {
  const model = modelOf(ctx);
  if (credits.indexOf(model) >= 0) return { units: 0, units_source: "estimated" };
  const body = ctx.officialRequest,
    settings = body.settings || {},
    max = limit(model);
  const output = settings.duration || max;
  const hasVideo = (body.contents || []).some((c) => ["feature_video", "base_video", "element"].indexOf(c.type) >= 0);
  return {
    resolution: (settings.resolution || (model === flash ? "720p" : "1080p")).toLowerCase(),
    video_input: hasVideo ? "video" : "none",
    billed_seconds: measured(Number(output), hasVideo ? max : 0),
  };
}
export function extractUsage(ctx) {
  return ctx.usagePurpose === "billing_ratios" ? null : estimate(ctx);
}
export function parseSubmitResponse(ctx, resp) {
  const body = resp.body || {},
    data = body.data || {},
    tencent = body.Response || {};
  if ((body.code != null && Number(body.code) !== 0) || tencent.Error)
    throw new Error(body.message || (tencent.Error || {}).Message || "Kling submission failed");
  const id = data.task_id || data.id || tencent.TaskId || body.task_id || body.id;
  if (typeof id !== "string" || !id.trim()) throw new Error("Missing Kling task ID");
  return { taskId: id, taskData: body, state: estimate(ctx) };
}
export function buildQueryRequest(ctx) {
  const route = path(ctx.action, modelOf(ctx)),
    id = encodeURIComponent(ctx.taskId);
  const url = route.indexOf("/v1/videos/") === 0 ? route + "/" + id : "/tasks?task_ids=" + id;
  return { method: "GET", url: base(ctx) + url, headers: { Accept: "application/json", Authorization: auth(ctx), "User-Agent": "kling-sdk/1.0" } };
}
function row(ctx, body) {
  if (body.status) return body;
  if (!Array.isArray(body.data)) return body.data || {};
  return body.data.find((item) => item && String(item.id || item.task_id) === ctx.taskId) || {};
}
function video(row) {
  return (
    (row.outputs || []).find((v) => v.type === "video") || ((row.task_result || {}).videos || [])[0] || (Array.isArray(row.data) ? row.data[0] : null) || {}
  );
}
export function parseTaskResult(ctx, body) {
  if (!body || (body.code != null && Number(body.code) !== 0)) throw new Error((body || {}).message || "Kling query failed");
  const item = row(ctx, body),
    id = item.task_id || item.id;
  if (id && id !== ctx.taskId) throw new Error("Kling returned a different task");
  const states = {
    pending: "SUBMITTED",
    queued: "QUEUED",
    submitted: "SUBMITTED",
    processing: "IN_PROGRESS",
    running: "IN_PROGRESS",
    in_progress: "IN_PROGRESS",
    succeed: "SUCCESS",
    succeeded: "SUCCESS",
    success: "SUCCESS",
    completed: "SUCCESS",
    failed: "FAILURE",
    failure: "FAILURE",
    error: "FAILURE",
    cancelled: "FAILURE",
    canceled: "FAILURE",
  };
  const status = states[String(item.task_status || item.status || "").toLowerCase()];
  if (!status) throw new Error("Unknown Kling task status");
  return {
    taskId: ctx.taskId,
    status: status,
    url: status === "SUCCESS" ? video(item).url || "" : "",
    reason: item.task_status_msg || item.message || (item.error || {}).message || "",
  };
}
export function extractUsageOnComplete(ctx, _result, body) {
  const item = row(ctx, body),
    model = modelOf(ctx);
  if (credits.indexOf(model) >= 0) {
    let units = readNumber(item.final_unit_deduction);
    if (units === null && Array.isArray(item.billing)) {
      const items = item.billing.filter((i) => i && (i.charge_type === "cash" || i.charge_type === "unit"));
      if (items.length && items.every((i) => readNumber(i.amount) !== null)) units = items.reduce((sum, i) => sum + Number(i.amount), 0);
    }
    return units === null ? null : { units: units, units_source: "reported" };
  }
  const usage = item.usage || {},
    submitted = ctx.state || {};
  const output = readNumber(usage.output_duration != null ? usage.output_duration : video(item).duration);
  const input = readNumber(usage.input_duration);
  if (
    output === null ||
    output < 3 ||
    output > limit(model) ||
    (input !== null && input > limit(model)) ||
    (input === null && (usage.input_duration != null || submitted.video_input === "video"))
  )
    return null;
  return { billed_seconds: measured(output, input || 0), video_input: input > 0 ? "video" : "none" };
}
function artifact(ctx) {
  const body = ctx.data || {};
  const item = Array.isArray(body.data)
    ? body.data.find((v) => v && (v.id === ctx.taskId || v.task_id === ctx.taskId)) || body.data[0] || {}
    : body.data || body;
  return video(item).url || "";
}
export function listArtifacts(task) {
  return task.status === "SUCCESS" && artifact(task) ? [{ key: "video", type: "video" }] : [];
}
export function buildContentRequest(ctx) {
  const url = artifact(ctx);
  if (ctx.artifactKey !== "video" || !url) throw new Error("artifact_not_found");
  return { url: url, method: ctx.clientRequest.method, credentialless: true };
}
