// Exact upstream IDs matter: in particular the lowercase flash-720p is a different tariff.
const SD2 = { maxSeconds: 15, maxImages: 9, maxVideos: 3, maxAudios: 3 };
const STANDARD = { maxSeconds: 30, maxImages: 30, maxVideos: 10, maxAudios: 10, maxText: 15000 };
const FLASH = { maxSeconds: 30, maxImages: 10, maxVideos: 5, maxAudios: 5, maxText: 15000 };
const PROFILES = {
  "sd-2-fast-480": { ...SD2, pixels: "480p", frames: true, audioNeedsVisual: true, implicitResolution: true },
  "sd-2-fast-720": { ...SD2, pixels: "720p", frames: true, audioNeedsVisual: true, implicitResolution: true },
  "sd-2-mini-480": { ...SD2, pixels: "480p", minSeconds: 5, watermark: true, implicitResolution: true },
  "sd-2-mini-720": { ...SD2, pixels: "720p", watermark: true, maxText: 5000 },
  "sd-2-vip-480": { ...SD2, pixels: "480p", watermark: true, maxText: 5000 },
  "sd-2-vip-720": { ...SD2, pixels: "720p", minSeconds: 5, watermark: true, maxText: 5000 },
  "sd-2.5-480P": { ...STANDARD, pixels: "480p" },
  "sd-2.5-720P": { ...STANDARD, pixels: "720p" },
  "sd-2.5-1080P": { ...STANDARD, pixels: "1080p", watermark: true },
  "sd-2.5-flash-480P": { ...FLASH, pixels: "480p" },
  "sd-2.5-flash-720P": { ...FLASH, pixels: "720p" },
};
const RATIOS = ["16:9", "4:3", "1:1", "3:4", "21:9", "9:16"];
const TYPES = ["image", "video", "audio"];
const STATUS = { queued: "QUEUED", in_progress: "IN_PROGRESS", completed: "SUCCESS", failed: "FAILURE", canceled: "FAILURE", cancelled: "FAILURE" };

export const meta = {
  apiVersion: 1, key: "aigas-seedance", name: "SD 2 / SD 2.5", version: "1.0.1",
  author: { name: "O1Key" }, icon: "ByteDance.Color",
  description: { en: "Seedance video generation with image, video and audio references", zh: "支持图片、视频和音频参考的 Seedance 视频生成" },
  baseUrl: "https://www.aigasapi.com", auth: "api_key", fetchMode: "per_task",
  models: Object.keys(PROFILES), protocols: ["openai_video"],
  usageSchema: { seconds: { type: "number", unit: "second", description: { en: "Video generation unit price", zh: "视频生成单价" } } },
  usageExamples: [{ label: "5s", facts: { seconds: 5 } }, { label: "10s", facts: { seconds: 10 } }],
};

function profileFor(name) {
  if (!Object.prototype.hasOwnProperty.call(PROFILES, name)) throw new Error("Unknown SD model; use the exact published model ID");
  return PROFILES[name];
}

function durationFor(value, profile) {
  const seconds = typeof value === "string" && /^\d+$/.test(value) ? Number(value) : value;
  const min = profile.minSeconds || 4;
  if (!Number.isInteger(seconds) || seconds < min || seconds > profile.maxSeconds) {
    throw new Error("duration must be an integer from " + min + " to " + profile.maxSeconds + " seconds for this model");
  }
  return seconds;
}

function mediaURL(value) {
  if (typeof value !== "string" || !/^https?:\/\/[^\s/?#@\\]+(?:[/?#][^\s\\]*)?$/i.test(value)) {
    throw new Error("Reference media must have a public HTTP(S) URL. Upload local media with POST /v1/o1key/uploads");
  }
  return value;
}

function reference(type, url, role, name) {
  if (!TYPES.includes(type)) throw new Error("Reference type must be image, video or audio");
  if (role === undefined || role === "reference_" + type) role = "reference";
  if (!["reference", "first_frame", "last_frame"].includes(role)) throw new Error("Unknown reference role");
  if (role !== "reference" && type !== "image") throw new Error("Only images can be used as first or last frames");
  const item = { type, role, url: mediaURL(url) };
  if (name !== undefined) {
    if (typeof name !== "string") throw new Error("Reference name must be a string");
    item.name = name;
  }
  return item;
}

function validateReferences(refs, profile) {
  const counts = { image: 0, video: 0, audio: 0, first_frame: 0, last_frame: 0 };
  for (const ref of refs) {
    counts[ref.type]++;
    if (ref.role !== "reference") counts[ref.role]++;
  }
  if (counts.image > profile.maxImages || counts.video > profile.maxVideos || counts.audio > profile.maxAudios) {
    throw new Error("Too many references for this model (images " + profile.maxImages + ", videos " + profile.maxVideos + ", audio " + profile.maxAudios + ")");
  }
  const frames = counts.first_frame + counts.last_frame;
  if (frames && !profile.frames) throw new Error("Use an ordinary image reference for this model; frame roles are supported by sd-2-fast-480/720");
  if (frames && (counts.first_frame !== 1 || counts.last_frame > 1 || counts.image !== frames)) {
    throw new Error("Use one first frame and optionally one last frame, without additional ordinary image references");
  }
  if (profile.audioNeedsVisual && counts.audio && !counts.image && !counts.video) throw new Error("Audio references require an image or a video for this model");
}

function normalize(model, req) {
  const p = profileFor(model);
  if (!req || typeof req !== "object" || Array.isArray(req)) throw new Error("Expected a JSON object");
  const publicFields = ["model", "prompt", "content", "references", "image", "images", "videos", "audios", "input_reference", "duration", "seconds", "resolution", "aspect_ratio", "ratio", "watermark"];
  for (const key of Object.keys(req)) {
    if (!publicFields.includes(key)) throw new Error("Unsupported SD parameter: " + key);
  }
  const refs = [];
  const texts = [];
  if (req.content !== undefined) {
    if (!Array.isArray(req.content)) throw new Error("content must be an array");
    for (const item of req.content) {
      if (!item || typeof item !== "object") throw new Error("Invalid content item");
      if (item.type === "text") {
        if (typeof item.text !== "string") throw new Error("content text must be a string");
        if (item.text.trim()) texts.push(item.text.trim());
      } else {
        const type = TYPES.find(t => item.type === t + "_url");
        if (!type) throw new Error("Unknown content item type");
        const media = item[item.type];
        refs.push(reference(type, typeof media === "string" ? media : media && media.url, item.role, item.name));
      }
    }
  }
  if (req.prompt !== undefined && (typeof req.prompt !== "string" || texts.length)) throw new Error("Supply a single prompt using prompt or content text");
  const prompt = (req.prompt === undefined ? texts.join("\n") : req.prompt).trim();
  if (!prompt) throw new Error("A non-empty prompt is required");
  if (p.maxText && Array.from(prompt).length > p.maxText) throw new Error("Prompt exceeds the limit of " + p.maxText + " characters");
  const legacy = req.image !== undefined || req.input_reference !== undefined || TYPES.some(t => req[t + "s"] !== undefined);
  if ((refs.length && (req.references !== undefined || legacy)) || (req.references !== undefined && legacy)) {
    throw new Error("Provide media in one format: references, content, or image/video/audio URL fields");
  }
  if (req.references !== undefined) {
    if (!Array.isArray(req.references)) throw new Error("references must be an array");
    for (const item of req.references) {
      if (!item || typeof item !== "object") throw new Error("Invalid reference object");
      refs.push(reference(item.type, item.url, item.role, item.name));
    }
  }
  if (legacy) {
    const single = [req.image, req.input_reference].filter(value => value !== undefined);
    if (single.length > 1 || (single.length && req.images !== undefined)) throw new Error("Choose one image field");
    if (single.length) refs.push(reference("image", single[0]));
    for (const type of TYPES) {
      const urls = req[type + "s"];
      if (urls === undefined) continue;
      if (!Array.isArray(urls)) throw new Error(type + "s must be an array of URLs");
      for (const url of urls) refs.push(reference(type, url));
    }
  }
  validateReferences(refs, p);
  if (req.duration !== undefined && req.seconds !== undefined && durationFor(req.duration, p) !== durationFor(req.seconds, p)) throw new Error("duration and seconds disagree");
  const duration = durationFor(req.duration !== undefined ? req.duration : (req.seconds !== undefined ? req.seconds : 5), p);
  if (req.resolution !== undefined && (typeof req.resolution !== "string" || req.resolution.toLowerCase() !== p.pixels)) throw new Error("This model produces " + p.pixels);
  if (req.ratio !== undefined && req.aspect_ratio !== undefined && req.ratio !== req.aspect_ratio) throw new Error("ratio and aspect_ratio disagree");
  const ratio = req.aspect_ratio !== undefined ? req.aspect_ratio : req.ratio;
  if (ratio !== undefined && !RATIOS.includes(ratio)) throw new Error("aspect_ratio must be one of " + RATIOS.join(", "));
  const body = { model, prompt, duration };
  if (!p.implicitResolution) body.resolution = p.pixels;
  if (ratio !== undefined) body.aspect_ratio = ratio;
  if (refs.length) body.references = refs;
  if (req.watermark !== undefined) {
    if (typeof req.watermark !== "boolean" || (req.watermark && !p.watermark)) throw new Error("Invalid watermark setting for this model");
    if (p.watermark) body.watermark = req.watermark;
  }
  return body;
}

export const protocols = {
  openai_video: {
    decodeRequest(ctx) {
      if (!ctx.body || ctx.body.kind !== "json") throw new Error("Submit a JSON request with public URLs for reference media");
      const body = normalize(ctx.model, ctx.body.value);
      const refs = body.references || [];
      const action = refs.some(r => r.type === "video") ? "video_to_video" : refs.length ? "image_to_video" : "text_to_video";
      return { kind: "submit", model: ctx.model, action, requestBody: body };
    },
    render(_ctx, task) {
      const result = {};
      if (task.status === "SUCCESS") {
        const url = "https://api.o1key.com/v1/videos/" + encodeURIComponent(task.task_id) + "/content";
        result.result_url = url;
        result.metadata = { url };
        const billed = (task.data || {}).billing_usage;
        if (billed && Number.isInteger(billed.billable_seconds)) result.seconds = String(billed.billable_seconds);
      }
      if (task.status === "FAILURE") result.error = { code: "video_generation_failed", message: task.fail_reason || "Video generation failed" };
      return result;
    },
  },
};

export function buildSubmitRequest(ctx) {
  const model = ctx.upstreamModel || ctx.model;
  const body = normalize(model, ctx.requestBody);
  if (!ctx.publicTaskId) throw new Error("A gateway task ID is required for idempotent submission");
  body.out_task_id = "o1key-" + ctx.publicTaskId;
  return { url: ctx.baseUrl + "/v1/videos", method: "POST", headers: { Authorization: "Bearer " + ctx.apiKey, "Content-Type": "application/json" }, body };
}

export function extractUsage(ctx) {
  return { seconds: durationFor(ctx.requestBody.duration, profileFor(ctx.upstreamModel || ctx.model)) };
}

export function parseSubmitResponse(_ctx, response) {
  const body = response.body || {};
  const id = body.task_id || body.id;
  if (typeof id !== "string" || !id) throw new Error("Video provider returned no task ID");
  return { taskId: id, taskData: { status: body.status } };
}

export function buildQueryRequest(ctx) {
  return { url: ctx.baseUrl + "/v1/videos/" + encodeURIComponent(ctx.taskId), method: "GET", headers: { Authorization: "Bearer " + ctx.apiKey } };
}

export function parseTaskResult(_ctx, body) {
  const status = Object.prototype.hasOwnProperty.call(STATUS, body.status) ? STATUS[body.status] : "UNKNOWN";
  const result = { status };
  if (Number.isInteger(body.progress) && body.progress >= 0 && body.progress <= 100) result.progress = body.progress + "%";
  if (status === "FAILURE") result.reason = (body.error || {}).message || body.fail_reason || "Video generation failed";
  if (status === "SUCCESS") result.url = mediaURL((body.metadata || {}).url);
  return result;
}

export function extractUsageOnComplete(ctx, result, body) {
  if (result.status !== "SUCCESS" || !body || !body.billing_usage) return {};
  const seconds = body.billing_usage.billable_seconds;
  const p = profileFor(ctx.upstreamModel || ctx.model);
  if (body.billing_usage.unit !== "second" || !Number.isInteger(seconds) || seconds < 0 || seconds > p.maxSeconds) return {};
  return { seconds };
}

export function listArtifacts(task) {
  return task.status === "SUCCESS" ? [{ key: "video", type: "video", mimeType: "video/mp4" }] : [];
}

export function buildContentRequest(ctx) {
  if (ctx.artifactKey !== "video") throw new Error("Unknown video artifact");
  return { url: mediaURL(((ctx.data || {}).metadata || {}).url), method: ctx.clientRequest.method, credentialless: true };
}
