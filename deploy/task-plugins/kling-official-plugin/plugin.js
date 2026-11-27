// Upstreams that expose only Kling's unified task API (/{task}/{model} and
// GET /tasks) with a Bearer API key, such as TokenMart. Kling 3.0 keeps the
// credit meter of kling-official-video so both plugins share price expressions.
const credits = ["kling-3.0", "kling-3.0-omni"];
const standard = "kling-4.0";
const flash = "kling-4.0-flash";
// Kling Image 4.0 (/image-generation and /long-image-generation). Kling's image
// credits are not its video credits, so prices are written in yuan per image.
const image = "kling-image-4.0";
const imageKinds = ["image", "webpage", "ecommerce_short", "ecommerce_medium", "ecommerce_long"];
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
    schema: { billed_seconds: seconds, ...(model === standard ? { hdr_seconds: { type: "number", unit: "second", description: { zh: "HDR 输出秒数", en: "HDR output seconds" } } } : {}), resolution: { enum: resolutions }, video_input: videoInput },
    examples: [
      { label: "5 秒无参考视频", facts: { ...(model === standard ? { hdr_seconds: 0 } : {}), billed_seconds: 5, resolution: "720p", video_input: "none" } },
      { label: "3 秒输入 + 5 秒输出", facts: { ...(model === standard ? { hdr_seconds: 0 } : {}), billed_seconds: 10, resolution: "720p", video_input: "video" } },
    ],
  };
}
export const meta = {
  apiVersion: 1,
  key: "kling-official-plugin",
  name: "可灵官方格式上游",
  version: "1.2.0",
  icon: "Kling.Color",
  author: { name: "O1Key" },
  fetchMode: "per_task",
  auth: "api_key",
  models: credits.concat([standard, flash, image]),
  usageProfiles: [
    {
      models: credits,
      schema: { units: { type: "number", unit: "credit", description: { zh: "官方扣减单位", en: "Provider credit units" } }, units_source: source },
      examples: [
        { label: "提交预留", facts: { units: 0, units_source: "estimated" } },
        { label: "官方扣减 3.6 单位", facts: { units: 3.6, units_source: "reported" } },
      ],
    },
    timedProfile(standard, ["720p", "1080p", "4k"]),
    timedProfile(flash, ["720p"]),
    {
      models: [image],
      schema: {
        images: { type: "number", unit: "count", description: { zh: "输出张数", en: "Output images" } },
        reference_images: { type: "number", unit: "count", description: { zh: "参考图和参考主体数", en: "Reference images and elements" } },
        kind: {
          enum: imageKinds,
          enumLabels: {
            image: { zh: "单图 / 组图", en: "Single / series" },
            webpage: { zh: "长图·网页设计", en: "Long image · webpage" },
            ecommerce_short: { zh: "长图·电商详情页·短", en: "Long image · ecommerce · short" },
            ecommerce_medium: { zh: "长图·电商详情页·中", en: "Long image · ecommerce · medium" },
            ecommerce_long: { zh: "长图·电商详情页·长", en: "Long image · ecommerce · long" },
          },
        },
        resolution: { enum: ["1k", "2k", "4k", "8k"] },
      },
      examples: [
        { label: "4 张 2K 组图 + 3 张参考", facts: { images: 4, reference_images: 3, kind: "image", resolution: "2k" } },
        { label: "电商详情页·中 2K + 2 张参考", facts: { images: 1, reference_images: 2, kind: "ecommerce_medium", resolution: "2k" } },
      ],
    },
  ],
};
function auth(ctx) {
  const key = String(ctx.apiKey || "").trim();
  if (!key || key.indexOf("|") >= 0) throw new Error("Kling native upstreams take one Bearer API key");
  return "Bearer " + key;
}
function base(ctx) {
  return ctx.baseUrl.replace(/\/+$/, "");
}
function path(action, model) {
  switch (action) {
    case "kling40Text":
      return "/text-to-video/" + model;
    case "kling40Image":
      return "/image-to-video/" + model;
    case "kling40Omni":
      return "/omni-video/" + model;
    // The earlier /kling/omni-video/kling-3.0-omni route sends the same
    // official body; upstreams serve it on the unified path.
    case "omniVideo30":
      return "/omni-video/kling-3.0-omni";
    case "klingImage40":
      return "/image-generation/kling-4.0";
    case "klingLongImage40":
      return "/long-image-generation/kling-4.0";
    default:
      throw new Error("This upstream serves only Kling's unified video and Image 4.0 API");
  }
}
function modelOf(ctx) {
  return ctx.upstreamModel || ctx.model;
}
export function buildSubmitRequest(ctx) {
  if (!ctx.officialRequest) throw new Error("This plugin requires a Kling channel bound to kling-official-plugin");
  const request = ctx.officialRequest;
  if (request.callback_url || (request.options || {}).callback_url)
    throw new Error("Kling callbacks are disabled. Omit callback_url and poll using the task ID.");
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

function creditRate(ctx) {
  const body = ctx.officialRequest || {};
  const settings = body.settings || {};
  const column = ["720p", "1080p", "4k"].indexOf(settings.resolution || "720p");
  const native = settings.audio === "native";
  let rates;
  if (modelOf(ctx) === "kling-3.0") {
    rates = native ? [0.9, 1.2, 3] : [0.6, 0.8, 3];
  } else {
    const reference = (body.contents || []).some((v) => v.type === "base_video" || v.type === "feature_video");
    rates = reference ? [0.9, 1.2, 3] : native ? [0.8, 1, 3] : [0.6, 0.8, 3];
  }
  return readNumber(rates[column]);
}
function retailUnits(ctx, item) {
  const state = ctx.state || {};
  const mediaSeconds = readNumber(video(item).duration);
  const seconds = readNumber((item.usage || {}).output_duration) || (mediaSeconds && Math.round(mediaSeconds)) || state.requestedSeconds;
  const rate = readNumber(state.creditRate);
  return rate !== null && seconds > 0 ? Number((rate * seconds).toFixed(6)) : null;
}
function completedCredits(ctx, item) {
  const fallback = () => retailUnits(ctx, item);
  if (Array.isArray(item.billing)) {
    let total = 0;
    if (!item.billing.length) return fallback();
    for (const line of item.billing) {
      if (!line) return fallback();
      const amount = readNumber(line.amount);
      if (line.charge_type === "unit" && amount !== null) {
        total += amount;
      } else if (line.charge_type === "cash") {
        const list = readNumber(line.list_price);
        if (amount === 0 && (list === 0 || line.list_price == null)) continue;
        if (String(line.currency || "").toUpperCase() !== "CNY" || list === null) return fallback();
        total += list;
      } else if (amount !== 0 || line.charge_type) {
        return fallback();
      }
    }
    return Number(total.toFixed(6));
  }
  const units = readNumber(item.final_unit_deduction);
  return units !== null ? units : fallback();
}

function limit(model) {
  return model === flash ? 20 : 30;
}
function measured(output, input) {
  return input > 0 ? Math.max(input + output, 2 * output) : output;
}
function imageEstimate(ctx) {
  const body = ctx.officialRequest || {},
    settings = body.settings || {};
  const references = (body.contents || []).filter((c) => c && c.type !== "prompt").length;
  if (ctx.action === "klingLongImage40") {
    const kind = settings.skill === "webpage" ? "webpage" : "ecommerce_" + (settings.length || "medium");
    return { images: 1, reference_images: references, kind, resolution: settings.resolution || "1k" };
  }
  // A series of "auto" length yields 2 to 10 images; reserve the maximum.
  const series = settings.result_type === "series";
  const amount = settings.series_amount === "auto" ? 10 : Number(settings.series_amount || 4);
  return { images: series ? amount : Number(settings.n || 1), reference_images: references, kind: "image", resolution: settings.resolution || "2k" };
}
function estimate(ctx) {
  const model = modelOf(ctx);
  if (model === image) return imageEstimate(ctx);
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
    ...(model === standard ? { hdr_seconds: settings.dynamic_range === "hdr" ? Number(output) : 0 } : {}),
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
  const request = ctx.officialRequest || {};
  const state = estimate(ctx);
  const model = modelOf(ctx);
  if (model === image) return { taskId: id, taskData: body, state };
  if (credits.indexOf(model) >= 0) {
    state.creditRate = creditRate(ctx);
  } else {
    state.knownVideo = (request.contents || []).some((v) => v.type === "base_video" || v.type === "feature_video");
  }
  state.requestedSeconds = readNumber((request.settings || {}).duration || request.duration);
  return { taskId: id, taskData: body, state };
}
export function buildQueryRequest(ctx) {
  return { method: "GET", url: base(ctx) + "/tasks?task_ids=" + encodeURIComponent(ctx.taskId), headers: { Accept: "application/json", Authorization: auth(ctx), "User-Agent": "kling-sdk/1.0" } };
}
function row(ctx, body) {
  if (body.status) return body;
  if (!Array.isArray(body.data)) return body.data || {};
  return body.data.find((item) => item && String(item.id || item.task_id) === ctx.taskId) || {};
}
function images(row) {
  return (row.outputs || []).filter((v) => v && v.type === "image" && v.url);
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
  // A successful asset with no verifiable final usage must not turn its
  // submission reservation into a final charge. The normal poller retries.
  if (status === "SUCCESS" && !extractUsageOnComplete(ctx, null, body))
    return { taskId: ctx.taskId, status: "IN_PROGRESS", reason: "Awaiting final video usage" };
  return {
    taskId: ctx.taskId,
    status: status,
    url: status !== "SUCCESS" ? "" : modelOf(ctx) === image ? (images(item)[0] || {}).url || "" : video(item).url || "",
    reason: item.task_status_msg || item.message || (item.error || {}).message || "",
  };
}
export function extractUsageOnComplete(ctx, _result, body) {
  const item = row(ctx, body),
    model = modelOf(ctx);
  const submitted = ctx.state || {};
  if (model === image) {
    // Bill the images actually delivered; references were sent with the request.
    const count = images(item).length;
    return count > 0 ? { images: count, reference_images: submitted.reference_images || 0, kind: submitted.kind || "image", resolution: submitted.resolution || "2k" } : null;
  }
  if (credits.indexOf(model) >= 0) {
    const units = completedCredits(ctx, item);
    return units === null ? null : { units, units_source: "reported" };
  }
  const usage = item.usage || {};
  const rawFile = readNumber(video(item).duration);
  const output = usage.output_duration != null
    ? readNumber(usage.output_duration)
    : readNumber(submitted.requestedSeconds) || (rawFile !== null ? Math.round(rawFile) : null);
  const input = readNumber(usage.input_duration);
  if (output === null || !Number.isInteger(output) || output < 3 || output > limit(model) ||
      (usage.input_duration != null && (input === null || input > limit(model)))) return null;
  if (input === null && submitted.video_input === "video" &&
      (submitted.knownVideo !== false || usage.output_duration == null)) return null;
  const actual = { billed_seconds: measured(output, input || 0), video_input: input > 0 ? "video" : "none" };
  if (model === standard) actual.hdr_seconds = submitted.hdr_seconds > 0 ? output : 0;
  return actual;
}
function artifactRow(ctx) {
  const body = ctx.data || {};
  return Array.isArray(body.data)
    ? body.data.find((v) => v && (v.id === ctx.taskId || v.task_id === ctx.taskId)) || body.data[0] || {}
    : body.data || body;
}
function isImageTask(task) {
  return task.action === "klingImage40" || task.action === "klingLongImage40";
}
export function listArtifacts(task) {
  if (task.status !== "SUCCESS") return [];
  const item = artifactRow(task);
  if (isImageTask(task)) return images(item).map((_, index) => ({ key: "image-" + (index + 1), type: "image" }));
  return video(item).url ? [{ key: "video", type: "video" }] : [];
}
export function buildContentRequest(ctx) {
  const item = artifactRow(ctx);
  let url = "";
  if (ctx.artifactKey === "video") {
    url = video(item).url || "";
  } else if (/^image-[1-9][0-9]*$/.test(String(ctx.artifactKey || ""))) {
    url = (images(item)[Number(ctx.artifactKey.slice(6)) - 1] || {}).url || "";
  }
  if (!url) throw new Error("artifact_not_found");
  return { url: url, method: ctx.clientRequest.method, credentialless: true };
}
