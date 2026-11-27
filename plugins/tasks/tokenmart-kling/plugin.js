// TokenMart Kling China upstream (official Kling API 2.0 shape).
//
// Upstream base: https://model.service-inference.ai
// Auth: Authorization: Bearer <platform API key>
// Paths sit at the root (no /kling prefix and no product prefix) and model
// names stay the China official -cn forms. The path model is forwarded as-is.
// International names such as kling-4.0 stay on the built-in /kling routes.
// These do not overlap the built-in /kling adaptor or the legacy kling plugin.
// /v1/videos/* is not registered: it intersects the OpenAI video routes.
//
// TokenMart's own write-up says some official calls are missing or ignored
// (POST /tasks, element library, callback_url, motion control, billing[]).
// Those notes are caveats, not refusals: official bodies and paths are
// forwarded unchanged. A 400/404 or a dropped field comes back as the upstream
// sent it. Media URLs must be downloadable from mainland China.
//
// Enable this plugin on its own key. Do not bind it to channel type 50.
// Task Plugin channels are type 62. The route pins this key; Distribute then
// accepts only a type 62 or New API channel bound to tokenmart-kling.

const VIDEO_MODELS = [
  "kling-4.0-cn",
  "kling-4.0-flash-cn",
  "kling-3.0-omni-cn",
  "kling-3.0-cn",
  "kling-3.0-turbo-cn",
  "kling-o1-cn",
  "kling-2.6-cn",
  "kling-2.5-turbo-cn",
];
const IMAGE_MODEL = "kling-image-4.0-cn";
const ELEMENT_MODEL = "kling-element-cn";
const RESOURCE_MODEL = "kling-resource-cn";

const VIDEO_RESOLUTION = {
  enum: ["720p", "1080p", "4k"],
  description: { en: "Output video resolution", zh: "输出视频分辨率" },
  enumLabels: {
    "720p": { en: "720p", zh: "720p" },
    "1080p": { en: "1080p", zh: "1080p" },
    "4k": { en: "4K", zh: "4K" },
  },
};

const VIDEO_SCHEMA = {
  output_seconds: {
    type: "number",
    unit: "second",
    description: { en: "Video generation unit price", zh: "视频生成单价" },
  },
  resolution: VIDEO_RESOLUTION,
  audio: {
    enum: ["native", "off", "original", "always"],
    description: { en: "Output audio mode", zh: "输出音频模式" },
    enumLabels: {
      native: { en: "Native audio", zh: "原生音频" },
      off: { en: "Silent", zh: "无声" },
      original: { en: "Original audio", zh: "原始音频" },
      always: { en: "Always on", zh: "始终有声" },
    },
  },
  dynamic_range: {
    enum: ["sdr", "hdr"],
    description: { en: "Output dynamic range", zh: "输出动态范围" },
    enumLabels: {
      sdr: { en: "SDR", zh: "SDR" },
      hdr: { en: "HDR", zh: "HDR" },
    },
  },
  input_video: {
    enum: ["none", "video"],
    description: { en: "Reference video input", zh: "参考视频输入" },
    enumLabels: {
      none: { en: "No reference video", zh: "无参考视频" },
      video: { en: "With reference video", zh: "有参考视频" },
    },
  },
  watermark: {
    type: "boolean",
    description: { en: "Whether a watermark is generated", zh: "是否生成水印" },
  },
};

const IMAGE_SCHEMA = {
  image_count: {
    type: "number",
    unit: "count",
    unitLabel: { en: "image", zh: "张" },
    description: { en: "Image generation unit price", zh: "图片生成单价" },
  },
  resolution: {
    enum: ["1k", "2k", "4k", "8k"],
    description: { en: "Output image resolution", zh: "输出图片分辨率" },
    enumLabels: {
      "1k": { en: "1K", zh: "1K" },
      "2k": { en: "2K", zh: "2K" },
      "4k": { en: "4K", zh: "4K" },
      "8k": { en: "8K", zh: "8K" },
    },
  },
  refer_images: {
    type: "number",
    unit: "count",
    unitLabel: { en: "image", zh: "张" },
    description: { en: "Reference image unit price", zh: "参考图单价" },
  },
  result_type: {
    enum: ["single", "series", "long"],
    description: { en: "Image result layout", zh: "图片结果形态" },
    enumLabels: {
      single: { en: "Single images", zh: "单图" },
      series: { en: "Image series", zh: "组图" },
      long: { en: "Long image", zh: "长图" },
    },
  },
  skill: {
    enum: ["none", "ecommerce", "webpage"],
    description: { en: "Long-image skill", zh: "长图技能" },
    enumLabels: {
      none: { en: "Not a long image", zh: "非长图" },
      ecommerce: { en: "Product detail page", zh: "电商详情页" },
      webpage: { en: "Web page design", zh: "网页设计" },
    },
  },
  length: {
    enum: ["none", "short", "medium", "long"],
    description: { en: "Long-image length", zh: "长图长度" },
    enumLabels: {
      none: { en: "Not set", zh: "未设置" },
      short: { en: "Short", zh: "短" },
      medium: { en: "Medium", zh: "中" },
      long: { en: "Long", zh: "长" },
    },
  },
  watermark: VIDEO_SCHEMA.watermark,
};

const REQUEST_SCHEMA = {
  requests: {
    type: "number",
    unit: "count",
    description: { en: "Account resource request unit price", zh: "账户资源请求单价" },
  },
};

const VIDEO_EXAMPLES = [
  {
    label: "4.0 1080p 5s",
    facts: { output_seconds: 5, resolution: "1080p", audio: "always", dynamic_range: "sdr", input_video: "none", watermark: false },
  },
  {
    label: "4.0 4k HDR input video",
    facts: { output_seconds: 10, resolution: "4k", audio: "always", dynamic_range: "hdr", input_video: "video", watermark: false },
  },
  {
    label: "flash 720p 10s",
    facts: { output_seconds: 10, resolution: "720p", audio: "always", dynamic_range: "sdr", input_video: "none", watermark: true },
  },
  {
    label: "3.0 1080p native 5s",
    facts: { output_seconds: 5, resolution: "1080p", audio: "native", dynamic_range: "sdr", input_video: "none", watermark: false },
  },
];

const GENERATION_ROUTES = [
  ["text-to-video", "text_to_video"],
  ["image-to-video", "image_to_video"],
  ["omni-video", "omni_video"],
  ["motion-control", "motion_control"],
  ["image-generation", "image_generation"],
  ["long-image-generation", "long_image"],
];

const RESOURCE_PATHS = [
  ["GET", "/v1/general/advanced-custom-elements"],
  ["GET", "/v1/general/advanced-presets-elements"],
  ["POST", "/v1/general/delete-advanced-elements"],
  ["POST", "/v1/general/custom-voices"],
  ["GET", "/v1/general/custom-voices"],
  ["GET", "/v1/general/custom-voices/:id"],
  ["GET", "/v1/general/presets-voices"],
  ["POST", "/v1/general/delete-voices"],
];

function buildRoutes() {
  const routes = [];
  for (let i = 0; i < GENERATION_ROUTES.length; i++) {
    const segment = GENERATION_ROUTES[i][0];
    const action = GENERATION_ROUTES[i][1];
    routes.push({
      method: "POST",
      path: "/" + segment + "/:model",
      type: "submit",
      action: action,
      decode: "decodeSubmit",
      render: "taskCreated",
    });
    routes.push({
      method: "GET",
      path: "/" + segment + "/:model/:task_id",
      type: "query",
      render: "taskStatus",
    });
  }
  routes.push({
    method: "POST",
    path: "/v1/general/advanced-custom-elements",
    type: "submit",
    action: "element",
    decode: "decodeElement",
    render: "taskCreated",
  });
  routes.push({
    method: "GET",
    path: "/v1/general/advanced-custom-elements/:task_id",
    type: "query",
    render: "taskStatus",
  });
  for (let i = 0; i < RESOURCE_PATHS.length; i++) {
    routes.push({
      method: RESOURCE_PATHS[i][0],
      path: RESOURCE_PATHS[i][1],
      type: "submit",
      action: "resource",
      decode: "decodeResource",
      render: "taskCreated",
      retainResult: false,
    });
  }
  routes.push({
    method: "GET",
    path: "/tasks",
    type: "dynamic",
    decode: "decodeTaskQuery",
    render: "renderAuto",
    retainResult: false,
  });
  routes.push({
    method: "POST",
    path: "/tasks",
    type: "dynamic",
    decode: "decodeTaskPage",
    render: "renderAuto",
    retainResult: false,
  });
  return routes;
}

export const meta = {
  apiVersion: 1,
  key: "tokenmart-kling",
  name: "TokenMart Kling",
  icon: "Kling.Color",
  description: {
    en: "TokenMart Kling China API 2.0 for video, omni, motion, and image generation",
    zh: "TokenMart 可灵国内站 API 2.0，覆盖视频、全能参考、动作控制与图片生成",
  },
  version: "1.0.0",
  author: { name: "QuantumNous" },
  website: "https://console.service-inference.ai/docs/kling-cn-native",
  baseUrl: "https://model.service-inference.ai",
  requiredCapabilities: ["json-clone@1"],
  models: VIDEO_MODELS.concat([IMAGE_MODEL, ELEMENT_MODEL, RESOURCE_MODEL]),
  fetchMode: "per_task",
  upstreams: ["vendor", "new_api"],
  usageSchema: VIDEO_SCHEMA,
  usageExamples: VIDEO_EXAMPLES,
  usageProfiles: [
    {
      models: [IMAGE_MODEL],
      schema: IMAGE_SCHEMA,
      examples: [
        {
          label: "single 2k",
          facts: { image_count: 1, resolution: "2k", refer_images: 0, result_type: "single", skill: "none", length: "none", watermark: false },
        },
        {
          label: "series 4k 4",
          facts: { image_count: 4, resolution: "4k", refer_images: 1, result_type: "series", skill: "none", length: "none", watermark: false },
        },
        {
          label: "long ecommerce 2k",
          facts: { image_count: 1, resolution: "2k", refer_images: 1, result_type: "long", skill: "ecommerce", length: "medium", watermark: false },
        },
      ],
    },
    {
      models: [ELEMENT_MODEL],
      schema: REQUEST_SCHEMA,
      examples: [{ label: "create element", facts: { requests: 1 } }],
    },
    {
      models: [RESOURCE_MODEL],
      schema: REQUEST_SCHEMA,
      examples: [{ label: "resource call", facts: { requests: 1 } }],
    },
  ],
  protocols: [
    { name: "openai_responses", supports: ["stream", "sync", "background"] },
    { name: "openai_video", models: VIDEO_MODELS },
    { name: "openai_image", models: [IMAGE_MODEL] },
  ],
  routes: buildRoutes(),
};

function trimmed(value) {
  return String(value || "").trim();
}

function asObject(value) {
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  return value;
}

function finiteNumber(value) {
  if (typeof value === "number" && Number.isFinite(value)) return value;
  if (typeof value === "string" && value.trim() !== "") {
    const parsed = Number(value);
    if (Number.isFinite(parsed)) return parsed;
  }
  return null;
}

function validModelSegment(model) {
  return /^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$/.test(model);
}

function actionFromPath(path) {
  const value = String(path || "");
  if (value.indexOf("/text-to-video/") >= 0) return "text_to_video";
  if (value.indexOf("/image-to-video/") >= 0) return "image_to_video";
  if (value.indexOf("/omni-video/") >= 0) return "omni_video";
  if (value.indexOf("/motion-control/") >= 0) return "motion_control";
  if (value.indexOf("/long-image-generation/") >= 0) return "long_image";
  if (value.indexOf("/image-generation/") >= 0) return "image_generation";
  if (value.indexOf("/advanced-custom-elements") >= 0) return "element";
  return "";
}

function billingModelFor(action, pathModel) {
  const name = trimmed(pathModel);
  if (action === "image_generation" || action === "long_image") {
    if (name === "kling-4.0-cn" || name === IMAGE_MODEL) return IMAGE_MODEL;
    return name;
  }
  if (action === "element") return ELEMENT_MODEL;
  if (action === "resource") return RESOURCE_MODEL;
  return name;
}

function isOmniOnly(model) {
  return model === "kling-3.0-omni-cn" || model === "kling-o1-cn";
}

function isImageModel(model) {
  return model === IMAGE_MODEL;
}

function requireJSONObject(ctx) {
  if (!ctx.body || ctx.body.kind !== "json") throw new Error("JSON body required");
  const body = ctx.body.value;
  if (!body || typeof body !== "object" || Array.isArray(body)) throw new Error("request body must be an object");
  return body;
}

function queryString(query) {
  if (!query || typeof query !== "object") return "";
  const parts = [];
  const names = Object.keys(query);
  for (let i = 0; i < names.length; i++) {
    const values = query[names[i]];
    const list = Array.isArray(values) ? values : [values];
    for (let j = 0; j < list.length; j++) {
      if (list[j] === undefined || list[j] === null) continue;
      parts.push(encodeURIComponent(names[i]) + "=" + encodeURIComponent(String(list[j])));
    }
  }
  if (!parts.length) return "";
  return "?" + parts.join("&");
}

function gatewayPath(ctx) {
  let path = String((ctx && ctx.path) || "");
  const params = (ctx && ctx.params) || {};
  const names = Object.keys(params);
  for (let i = 0; i < names.length; i++) {
    path = path.split(":" + names[i]).join(encodeURIComponent(String(params[names[i]])));
  }
  if (!path || path.charAt(0) !== "/") throw new Error("resource path is invalid");
  return path;
}

function splitIDs(value) {
  return String(value || "")
    .split(",")
    .map(function (part) {
      return part.trim();
    })
    .filter(function (part) {
      return part;
    });
}

function normalizeIDList(value) {
  const ids = Array.isArray(value)
    ? value
        .map(function (part) {
          return trimmed(part);
        })
        .filter(function (part) {
          return part;
        })
    : splitIDs(value);
  if (ids.length > 500) throw new Error("at most 500 task ids can be queried");
  return ids;
}

function firstQuery(ctx, name) {
  const values = ((ctx && ctx.query) || {})[name] || [];
  if (Array.isArray(values)) return trimmed(values[0]);
  return trimmed(values);
}

function passthrough(method, path, query, body) {
  return {
    kind: "submit",
    model: RESOURCE_MODEL,
    action: "resource",
    requestBody: { passthrough: true, method: method, path: path, query: query || "", body: body },
  };
}

function root(ctx) {
  const base = trimmed(ctx && ctx.baseUrl).replace(/\/+$/, "");
  if (!base) throw new Error("base_url is empty");
  return base;
}

function authHeaders(ctx, json) {
  const key = trimmed(ctx && ctx.apiKey);
  if (!key) throw new Error("api_key is empty");
  const headers = { Accept: "application/json", Authorization: "Bearer " + key };
  if (json) headers["Content-Type"] = "application/json";
  return headers;
}

function outboundModel(ctx) {
  const req = (ctx && ctx.requestBody) || {};
  if (trimmed(req.pathModel)) return trimmed(req.pathModel);
  return trimmed((ctx && (ctx.upstreamModel || ctx.model)) || "");
}

function upstreamPath(ctx) {
  const req = (ctx && ctx.requestBody) || {};
  if (req.passthrough || (req.path && !trimmed(req.pathModel))) {
    if (typeof req.path !== "string" || req.path.charAt(0) !== "/") throw new Error("resource path is invalid");
    return req.path;
  }
  const action = ctx && ctx.action;
  if (action === "element") return "/v1/general/advanced-custom-elements";
  const segments = {
    text_to_video: "text-to-video",
    image_to_video: "image-to-video",
    omni_video: "omni-video",
    motion_control: "motion-control",
    image_generation: "image-generation",
    long_image: "long-image-generation",
  };
  const segment = segments[action];
  if (!segment) throw new Error("unsupported action");
  const model = outboundModel(ctx);
  if (!validModelSegment(model)) throw new Error("model is required");
  return "/" + segment + "/" + encodeURIComponent(model);
}

function officialBody(ctx) {
  const req = (ctx && ctx.requestBody) || {};
  const body = req.body;
  if (!body || typeof body !== "object" || Array.isArray(body)) throw new Error("request body must be an object");
  return body;
}

function upstreamFailed(body) {
  if (!body || typeof body !== "object" || Array.isArray(body) || !Object.prototype.hasOwnProperty.call(body, "code")) return "";
  if (body.code === 0 || body.code === "0") return "";
  return String(body.message || "upstream request failed");
}

function taskRecord(body, taskId) {
  const rootBody = asObject(body);
  if (!rootBody) return null;
  const data = rootBody.data !== undefined ? rootBody.data : null;
  if (Array.isArray(data)) {
    if (!data.length) return null;
    if (taskId) {
      for (let i = 0; i < data.length; i++) {
        const item = asObject(data[i]);
        if (item && String(item.id || "") === String(taskId)) return item;
      }
    }
    return asObject(data[0]);
  }
  const object = asObject(data);
  if (object && (object.id || object.status || object.task_status || object.outputs || object.create_time)) return object;
  if (rootBody.id || rootBody.status || rootBody.task_status || rootBody.outputs) return rootBody;
  return null;
}

function mapStatus(status) {
  switch (String(status || "").toLowerCase()) {
    case "submitted":
    case "queued":
      return "SUBMITTED";
    case "processing":
    case "running":
      return "IN_PROGRESS";
    case "succeeded":
    case "succeed":
    case "success":
      return "SUCCESS";
    case "failed":
    case "failure":
      return "FAILURE";
    default:
      return "";
  }
}

function officialStatus(status) {
  switch (String(status || "").toUpperCase()) {
    case "SUCCESS":
      return "succeeded";
    case "FAILURE":
      return "failed";
    case "IN_PROGRESS":
      return "processing";
    default:
      return "submitted";
  }
}

function mediaURL(output) {
  if (!output || typeof output !== "object") return "";
  return trimmed(output.url || output.mp3_url || output.wav_url || output.watermark_url);
}

function firstMediaURL(record) {
  const outputs = record && Array.isArray(record.outputs) ? record.outputs : [];
  for (let i = 0; i < outputs.length; i++) {
    const url = mediaURL(outputs[i]);
    if (url) return url;
  }
  return "";
}

function outputSeconds(model, settings) {
  const duration = finiteNumber(settings && settings.duration);
  if (duration != null && duration > 0) return duration;
  if (model === "kling-4.0-cn" || model === "kling-4.0-flash-cn") return 10;
  return 5;
}

function videoResolution(model, settings) {
  const raw = trimmed(settings && settings.resolution).toLowerCase();
  if (raw === "720p" || raw === "1080p" || raw === "4k") return raw;
  if (model === "kling-4.0-cn") return "1080p";
  return "720p";
}

function audioFact(model, settings) {
  const raw = trimmed(settings && settings.audio).toLowerCase();
  if (raw === "native" || raw === "off" || raw === "original" || raw === "always") return raw;
  if (model === "kling-4.0-cn" || model === "kling-4.0-flash-cn" || model === "kling-3.0-turbo-cn") return "always";
  if (model === "kling-o1-cn" || model === "kling-2.5-turbo-cn") return "off";
  return "off";
}

function dynamicRangeFact(settings) {
  return trimmed(settings && settings.dynamic_range).toLowerCase() === "hdr" ? "hdr" : "sdr";
}

function hasInputVideo(action, contents) {
  if (action === "motion_control") return true;
  if (!Array.isArray(contents)) return false;
  for (let i = 0; i < contents.length; i++) {
    const type = contents[i] && contents[i].type;
    if (type === "feature_video" || type === "base_video" || type === "video") return true;
  }
  return false;
}

function watermarkOn(options) {
  const info = asObject(options && options.watermark_info);
  return !!(info && info.enabled === true);
}

function videoFacts(model, action, body) {
  const settings = asObject(body && body.settings) || {};
  const options = asObject(body && body.options) || {};
  const contents = body && Array.isArray(body.contents) ? body.contents : [];
  return {
    output_seconds: outputSeconds(model, settings),
    resolution: videoResolution(model, settings),
    audio: audioFact(model, settings),
    dynamic_range: dynamicRangeFact(settings),
    input_video: hasInputVideo(action, contents) ? "video" : "none",
    watermark: watermarkOn(options),
  };
}

function referCount(contents) {
  if (!Array.isArray(contents)) return 0;
  let count = 0;
  for (let i = 0; i < contents.length; i++) {
    const type = contents[i] && contents[i].type;
    if (type === "refer_image" || type === "style_refer_image" || type === "subject_refer_image") count++;
  }
  return count;
}

function imageResolution(action, settings) {
  const raw = trimmed(settings && settings.resolution).toLowerCase();
  if (raw === "1k" || raw === "2k" || raw === "4k" || raw === "8k") return raw;
  return action === "long_image" ? "1k" : "2k";
}

function imageFacts(action, body) {
  const settings = asObject(body && body.settings) || {};
  const options = asObject(body && body.options) || {};
  const contents = body && Array.isArray(body.contents) ? body.contents : [];
  let resultType = "single";
  if (action === "long_image") resultType = "long";
  else if (settings.result_type === "series") resultType = "series";
  let imageCount = 1;
  if (resultType === "series") {
    if (settings.series_amount === "auto") imageCount = 4;
    else {
      const amount = finiteNumber(settings.series_amount);
      imageCount = amount != null && amount > 0 ? amount : 4;
    }
  } else if (resultType === "single") {
    const count = finiteNumber(settings.n);
    imageCount = count != null && count > 0 ? count : 1;
  }
  let skill = "none";
  if (settings.skill === "ecommerce" || settings.skill === "webpage") skill = settings.skill;
  let length = "none";
  if (skill === "ecommerce") length = settings.length === "short" || settings.length === "long" ? settings.length : "medium";
  else if (settings.length === "short" || settings.length === "medium" || settings.length === "long") length = settings.length;
  return {
    image_count: imageCount,
    resolution: imageResolution(action, settings),
    refer_images: referCount(contents),
    result_type: resultType,
    skill: skill,
    length: length,
    watermark: watermarkOn(options),
  };
}

function countImages(record) {
  const outputs = record && Array.isArray(record.outputs) ? record.outputs : [];
  let images = 0;
  let untyped = 0;
  for (let i = 0; i < outputs.length; i++) {
    const output = outputs[i];
    if (!output || typeof output !== "object") continue;
    if (output.type === "image") images++;
    else if (!output.type && mediaURL(output)) untyped++;
  }
  return images > 0 ? images : untyped;
}

function durationSum(record) {
  const outputs = record && Array.isArray(record.outputs) ? record.outputs : [];
  let sum = 0;
  let any = false;
  for (let i = 0; i < outputs.length; i++) {
    const duration = finiteNumber(outputs[i] && outputs[i].duration);
    if (duration != null && duration >= 0) {
      sum += duration;
      any = true;
    }
  }
  return any ? sum : null;
}

function billedSeconds(model, record) {
  const usage = asObject(record && record.usage) || {};
  const output = finiteNumber(usage.output_duration);
  const input = finiteNumber(usage.input_duration);
  const four = model === "kling-4.0-cn" || model === "kling-4.0-flash-cn";
  if (output != null && output >= 0) {
    if (four && input != null && input > 0) return Math.max(input + output, output * 2);
    if (four) return output;
  }
  const fromFile = durationSum(record);
  const raw = output != null && output >= 0 ? output : fromFile;
  if (raw == null) return null;
  if (four) return raw;
  return Math.floor(fromFile != null ? fromFile : raw);
}

function looksLikeTaskCreate(data) {
  if (!data) return false;
  if (data.status === "submitted" || data.status === "processing") return true;
  return data.create_time !== undefined && data.create_time !== null && (data.id || data.status);
}

function officialItem(task) {
  const record = taskRecord(task && task.data, "");
  const item = record ? utils.json.clone(record) : {};
  item.id = task.task_id;
  if (task && task.status === "SUCCESS") item.status = "succeeded";
  else if (task && task.status === "FAILURE") {
    item.status = "failed";
    if (!item.message) item.message = task.fail_reason || "";
  } else if (!item.status) item.status = officialStatus(task && task.status);
  return item;
}

function responsesInput(req) {
  const texts = [];
  const images = [];
  const input = req && req.input;
  if (typeof input === "string") texts.push(input);
  else if (Array.isArray(input)) {
    for (let i = 0; i < input.length; i++) {
      const item = input[i];
      if (typeof item === "string") {
        texts.push(item);
        continue;
      }
      if (!asObject(item)) continue;
      const content = item.content === undefined ? [item] : Array.isArray(item.content) ? item.content : [item.content];
      for (let j = 0; j < content.length; j++) {
        const part = content[j];
        if (typeof part === "string") {
          texts.push(part);
          continue;
        }
        if (!asObject(part)) continue;
        if ((part.type === "input_text" || part.type === "text") && typeof part.text === "string") texts.push(part.text);
        if (part.type === "input_image" || part.type === "image_url") {
          let image = part.image_url;
          if (image && typeof image === "object") image = image.url;
          if (trimmed(image)) images.push(trimmed(image));
        }
      }
    }
  }
  return {
    prompt: texts
      .filter(function (text) {
        return trimmed(text);
      })
      .join("\n"),
    images: images,
  };
}

function explicitDuration(req) {
  if (req && Object.prototype.hasOwnProperty.call(req, "seconds")) return req.seconds;
  if (req && Object.prototype.hasOwnProperty.call(req, "duration")) return req.duration;
  return undefined;
}

function aspectFromSize(size) {
  const ratios = { "1024x1024": "1:1", "1280x720": "16:9", "1920x1080": "16:9", "720x1280": "9:16", "1080x1920": "9:16" };
  return ratios[trimmed(size)] || "";
}

function takeOfficial(req) {
  const body = {};
  let any = false;
  function copyFrom(source) {
    if (!source) return;
    if (source.prompt !== undefined) {
      body.prompt = source.prompt;
      any = true;
    }
    if (source.contents !== undefined) {
      body.contents = source.contents;
      any = true;
    }
    if (source.settings !== undefined) {
      body.settings = source.settings;
      any = true;
    }
    if (source.options !== undefined) {
      body.options = source.options;
      any = true;
    }
  }
  copyFrom(req);
  copyFrom(asObject(req && req.metadata));
  return any ? body : null;
}

function protocolAction(model, body, hasImage) {
  if (isImageModel(model)) return "image_generation";
  if (isOmniOnly(model)) return "omni_video";
  if (hasInputVideo("", body && body.contents)) return "omni_video";
  if (hasImage) return "image_to_video";
  return "text_to_video";
}

function decodeGenerative(ctx, req) {
  const model = trimmed(ctx && ctx.model);
  if (!model) throw new Error("model is required");
  const upstream = trimmed((ctx && (ctx.upstreamModel || ctx.model)) || "");
  if (upstream === ELEMENT_MODEL || upstream === RESOURCE_MODEL || model === ELEMENT_MODEL || model === RESOURCE_MODEL) {
    throw new Error("model does not support this protocol");
  }
  const input = responsesInput(req);
  const images = [];
  const seeds = [req && req.image, req && req.input_reference].concat((req && req.images) || [], input.images);
  for (let i = 0; i < seeds.length; i++) {
    const image = trimmed(seeds[i]);
    if (image && images.indexOf(image) < 0) images.push(image);
  }
  let body = takeOfficial(req);
  const prompt = (body && trimmed(body.prompt)) || input.prompt || trimmed(req && req.prompt);
  if (!body) {
    body = {};
    if (isImageModel(upstream) || isImageModel(model)) {
      const contents = [];
      if (prompt) contents.push({ type: "prompt", text: prompt });
      for (let i = 0; i < images.length; i++) contents.push({ type: "refer_image", url: images[i], id: "image_" + String(i + 1) });
      body.contents = contents;
    } else if (images.length || isOmniOnly(upstream) || isOmniOnly(model)) {
      const contents = [];
      if (prompt) contents.push({ type: "prompt", text: prompt });
      const imageType = isOmniOnly(upstream) || isOmniOnly(model) ? "refer_image" : "first_frame";
      if (images[0]) contents.push({ type: imageType, url: images[0] });
      if (images[1] && imageType === "first_frame") contents.push({ type: "last_frame", url: images[1] });
      body.contents = contents;
    } else if (prompt) body.prompt = prompt;
  } else if (!body.prompt && !body.contents && prompt) body.prompt = prompt;
  const duration = explicitDuration(req);
  if (duration !== undefined) {
    const settings = Object.assign({}, body.settings || {});
    if (settings.duration === undefined) settings.duration = duration;
    body.settings = settings;
  }
  const ratio = aspectFromSize(req && req.size);
  if (ratio) {
    const settings = Object.assign({}, body.settings || {});
    if (settings.aspect_ratio === undefined) settings.aspect_ratio = ratio;
    body.settings = settings;
  }
  if (!prompt && images.length === 0 && !body.contents && !body.prompt) throw new Error("input is required");
  const declared = isImageModel(upstream) || isImageModel(model) ? IMAGE_MODEL : upstream || model;
  return {
    kind: "submit",
    model: ctx.model,
    action: protocolAction(declared, body, images.length > 0),
    requestBody: { body: body },
  };
}

function validateProtocolRequest(ctx) {
  if (!ctx.body || (ctx.body.kind !== "json" && ctx.body.kind !== "multipart")) throw new Error("JSON or multipart body required");
  if (ctx.body.kind === "json") {
    if (!ctx.body.value || Array.isArray(ctx.body.value)) throw new Error("JSON object required");
    const req = ctx.body.value;
    if (req.input !== undefined && typeof req.input !== "string" && !Array.isArray(req.input)) throw new Error("input must be a string or array");
    if (req.metadata !== undefined && !asObject(req.metadata)) throw new Error("metadata must be an object");
    return req;
  }
  const fields = ctx.body.fields || {};
  const req = {};
  const names = Object.keys(fields);
  for (let i = 0; i < names.length; i++) {
    const values = fields[names[i]] || [];
    if (values.length > 1) throw new Error(names[i] + " must be provided once");
    req[names[i]] = values[0];
  }
  if (req.metadata !== undefined) {
    let parsed;
    try {
      parsed = JSON.parse(req.metadata);
    } catch (err) {
      throw new Error("metadata must be a JSON object string");
    }
    if (!asObject(parsed)) throw new Error("metadata must be a JSON object string");
    req.metadata = parsed;
  }
  if (req.seconds !== undefined) req.seconds = Number(req.seconds);
  const files = ctx.body.files || [];
  if (files.length) {
    req.input_reference = { __fileRef: files[0].ref, encoding: "base64" };
  }
  return req;
}

function escapeAttr(value) {
  return trimmed(value).replace(/&/g, "&amp;").replace(/"/g, "&quot;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

function responsesMediaText(ctx) {
  const artifacts = (ctx && ctx.artifacts) || {};
  const model = trimmed(ctx && (ctx.upstreamModel || ctx.model));
  const imageModel = isImageModel(model);
  const parts = [];
  if (!imageModel && artifacts.video && trimmed(artifacts.video.url)) {
    parts.push('<video controls src="' + escapeAttr(artifacts.video.url) + '"></video>');
  }
  const keys = Object.keys(artifacts);
  for (let i = 0; i < keys.length; i++) {
    if (keys[i] === "video") continue;
    const item = artifacts[keys[i]];
    if (!item || !trimmed(item.url)) continue;
    if (!imageModel && item.type === "video") parts.push('<video controls src="' + escapeAttr(item.url) + '"></video>');
    if (item.type === "image") parts.push('<img src="' + escapeAttr(item.url) + '" alt="" />');
    if (item.type === "audio") parts.push('<audio controls src="' + escapeAttr(item.url) + '"></audio>');
  }
  if (parts.length) return parts.join("");
  throw new Error(imageModel ? "image artifact is unavailable" : "video artifact is unavailable");
}

function artifactType(type) {
  if (type === "video" || type === "image" || type === "audio") return type;
  return "file";
}

function artifactMime(type) {
  if (type === "video") return "video/mp4";
  if (type === "image") return "image/png";
  if (type === "audio") return "audio/mpeg";
  return "";
}

export const native = {
  decodeSubmit: function (ctx) {
    const body = requireJSONObject(ctx);
    const pathModel = trimmed((ctx.params || {}).model);
    if (!validModelSegment(pathModel)) throw new Error("model is required");
    const action = actionFromPath(ctx.path || "");
    if (!action) throw new Error("unsupported action");
    return {
      kind: "submit",
      model: billingModelFor(action, pathModel),
      action: action,
      requestBody: { pathModel: pathModel, body: utils.json.clone(body) },
    };
  },
  decodeElement: function (ctx) {
    const body = requireJSONObject(ctx);
    return {
      kind: "submit",
      model: ELEMENT_MODEL,
      action: "element",
      requestBody: { path: "/v1/general/advanced-custom-elements", body: utils.json.clone(body) },
    };
  },
  decodeResource: function (ctx) {
    const method = String((ctx && ctx.method) || "GET").toUpperCase();
    let body = null;
    if (method !== "GET" && method !== "HEAD") body = utils.json.clone(requireJSONObject(ctx));
    return passthrough(method, gatewayPath(ctx), queryString(ctx && ctx.query), body);
  },
  decodeTaskQuery: function (ctx) {
    const taskIDs = firstQuery(ctx, "task_ids");
    const externalIDs = firstQuery(ctx, "external_task_ids");
    if (taskIDs && externalIDs) throw new Error("task_ids and external_task_ids cannot be used together");
    if (!taskIDs && !externalIDs) throw new Error("task_ids or external_task_ids is required");
    if (taskIDs) {
      const ids = normalizeIDList(taskIDs);
      if (!ids.length) throw new Error("task_ids or external_task_ids is required");
      return { kind: "query", taskIds: ids };
    }
    return passthrough("GET", "/tasks", "?external_task_ids=" + encodeURIComponent(externalIDs), null);
  },
  decodeTaskPage: function (ctx) {
    const body = requireJSONObject(ctx);
    if (body.task_ids !== undefined && body.task_ids !== null && trimmed(Array.isArray(body.task_ids) ? body.task_ids.join(",") : body.task_ids)) {
      const ids = normalizeIDList(body.task_ids);
      if (!ids.length) throw new Error("task_ids or external_task_ids is required");
      return { kind: "query", taskIds: ids };
    }
    return passthrough("POST", "/tasks", "", utils.json.clone(body));
  },
  taskCreated: function (ctx, task) {
    const body = asObject(task && task.data);
    if (!body) return { code: 0, message: "SUCCEED", data: { id: task.task_id, status: "submitted" } };
    const data = asObject(body.data);
    if (!looksLikeTaskCreate(data)) return body;
    const copy = utils.json.clone(body);
    copy.data.id = task.task_id;
    return copy;
  },
  taskStatus: function (ctx, task) {
    return { code: 0, message: "SUCCEED", data: [officialItem(task)] };
  },
  taskList: function (ctx, tasks) {
    const list = Array.isArray(tasks) ? tasks : [tasks];
    const data = [];
    for (let i = 0; i < list.length; i++) data.push(officialItem(list[i]));
    return { code: 0, message: "SUCCEED", data: data };
  },
  renderAuto: function (ctx, taskOrTasks) {
    if (Array.isArray(taskOrTasks)) return native.taskList(ctx, taskOrTasks);
    return native.taskCreated(ctx, taskOrTasks);
  },
  error: function (ctx, error) {
    return { code: (error && error.httpStatus) || 5000, message: (error && error.message) || "" };
  },
};

export function buildSubmitRequest(ctx) {
  const req = (ctx && ctx.requestBody) || {};
  const path = upstreamPath(ctx);
  const query = typeof req.query === "string" ? req.query : "";
  const method = String(req.passthrough ? req.method || "POST" : "POST").toUpperCase();
  const descriptor = {
    url: root(ctx) + path + query,
    method: method,
    headers: authHeaders(ctx, method !== "GET" && method !== "HEAD"),
    action: (ctx && ctx.action) || (req.passthrough ? "resource" : ""),
  };
  if (method !== "GET" && method !== "HEAD") descriptor.body = req.passthrough ? req.body : officialBody(ctx);
  return descriptor;
}

export function parseSubmitResponse(ctx, resp) {
  const body = (resp && resp.body) || {};
  const failed = upstreamFailed(body);
  if (failed) throw new Error(failed);
  const req = (ctx && ctx.requestBody) || {};
  if (req.passthrough) {
    const taskData = body && typeof body === "object" ? body : { code: 0, message: "SUCCEED" };
    return { taskId: utils.uuid(), taskData: taskData, immediate: { status: "SUCCESS" } };
  }
  const data = asObject(body.data);
  const taskId = trimmed(data && (data.id || data.task_id));
  if (!taskId) throw new Error("missing task id");
  return { taskId: taskId, taskData: body };
}

export function buildQueryRequest(ctx) {
  const taskId = trimmed(ctx && ctx.taskId);
  if (!taskId) throw new Error("task id is empty");
  return {
    url: root(ctx) + "/tasks?task_ids=" + encodeURIComponent(taskId),
    method: "GET",
    headers: authHeaders(ctx, false),
  };
}

export function parseTaskResult(ctx, body) {
  const rootBody = asObject(body);
  const record = taskRecord(body, ctx && ctx.taskId);
  if (!record) {
    const failed = upstreamFailed(rootBody);
    if (failed) return { status: "FAILURE", reason: failed };
    return { status: "IN_PROGRESS", reason: "task is not in the query result yet" };
  }
  const status = mapStatus(record.status || record.task_status);
  if (!status) return { status: "UNKNOWN", reason: "unknown task status: " + String(record.status || record.task_status || "") };
  const result = { status: status, reason: String(record.message || record.task_status_msg || "") };
  if (record.id) result.taskId = String(record.id);
  const url = firstMediaURL(record);
  if (url) result.url = url;
  return result;
}

export function extractUsage(ctx) {
  if (ctx && ctx.usagePurpose === "billing_ratios") return null;
  const model = trimmed(ctx && (ctx.upstreamModel || ctx.model));
  const action = trimmed(ctx && ctx.action);
  const req = (ctx && ctx.requestBody) || {};
  const body = asObject(req.body) || {};
  if (action === "resource" || model === RESOURCE_MODEL) return { requests: 1 };
  if (action === "element" || model === ELEMENT_MODEL) return { requests: 1 };
  if (action === "image_generation" || action === "long_image" || isImageModel(model)) return imageFacts(action, body);
  return videoFacts(model, action, body);
}

export function extractUsageOnComplete(task, _result, body) {
  const model = trimmed(task && (task.upstreamModel || task.model));
  const action = trimmed(task && task.action);
  const record = taskRecord(body, task && task.taskId);
  if (!record) return null;
  if (action === "image_generation" || action === "long_image" || isImageModel(model)) {
    const count = countImages(record);
    if (count > 0) return { image_count: count };
    return null;
  }
  if (action === "element" || action === "resource" || model === ELEMENT_MODEL || model === RESOURCE_MODEL) return null;
  const seconds = billedSeconds(model, record);
  if (seconds == null) return null;
  return { output_seconds: seconds };
}

export function listArtifacts(task) {
  if (!task || task.status !== "SUCCESS") return [];
  const record = taskRecord(task.data, task.taskId);
  const outputs = record && Array.isArray(record.outputs) ? record.outputs : [];
  const artifacts = [];
  for (let i = 0; i < outputs.length; i++) {
    if (!mediaURL(outputs[i])) continue;
    const type = artifactType(outputs[i].type);
    const artifact = { key: "output-" + String(i + 1), type: type };
    const mime = artifactMime(type);
    if (mime) artifact.mimeType = mime;
    artifacts.push(artifact);
  }
  return artifacts;
}

export function buildContentRequest(ctx) {
  const record = taskRecord(ctx && ctx.data, ctx && ctx.taskId);
  const outputs = record && Array.isArray(record.outputs) ? record.outputs : [];
  let url = "";
  for (let i = 0; i < outputs.length; i++) {
    if ("output-" + String(i + 1) !== ctx.artifactKey) continue;
    url = mediaURL(outputs[i]);
    break;
  }
  if (!url) throw new Error("artifact_not_found");
  const method = ctx && ctx.clientRequest && ctx.clientRequest.method ? ctx.clientRequest.method : "GET";
  return { url: url, method: method, credentialless: true };
}

export const protocols = {
  openai_responses: {
    decodeRequest: function (ctx) {
      const req = validateProtocolRequest(ctx);
      return decodeGenerative(ctx, req);
    },
    renderEvents: function (ctx, task, previousState) {
      const status = String((task && task.status) || "UNKNOWN").toUpperCase();
      const value = Number(String((task && task.progress) || "").replace("%", ""));
      const progress = Number.isFinite(value) && value >= 0 && value <= 100 ? value : null;
      const state = { status: status, progress: progress };
      if (status === "SUCCESS") {
        const text = responsesMediaText(ctx);
        const events = previousState && previousState.status === status ? [] : [{ type: "output", data: text }];
        return { events: events, state: state, done: true };
      }
      if (status === "FAILURE") {
        return { events: [{ type: "error", code: "task_failed", message: (task && task.fail_reason) || "task failed" }], state: state, done: true };
      }
      if (previousState && previousState.status === status && previousState.progress === progress) return { events: [], state: state, done: false };
      const event = { type: "progress", message: status.toLowerCase() };
      if (progress !== null) event.progress = progress;
      return { events: [event], state: state, done: false };
    },
    renderFinal: function (ctx, _task) {
      return {
        output: [
          {
            type: "message",
            status: "completed",
            role: "assistant",
            content: [{ type: "output_text", text: responsesMediaText(ctx), annotations: [], logprobs: [] }],
          },
        ],
        metadata: { vendor: "tokenmart-kling" },
      };
    },
  },
  openai_video: {
    decodeRequest: function (ctx) {
      const req = validateProtocolRequest(ctx);
      if (req.input === undefined && req.prompt !== undefined) req.input = req.prompt;
      return decodeGenerative(ctx, req);
    },
    render: function (ctx, task) {
      const record = taskRecord(task && task.data, task && task.task_id) || {};
      const output = { id: task && task.task_id, object: "video", model: trimmed(ctx && ctx.model) };
      const url = firstMediaURL(record);
      if (url) output.url = url;
      const outputs = Array.isArray(record.outputs) ? record.outputs : [];
      const seconds = finiteNumber(outputs[0] && outputs[0].duration);
      if (seconds != null) output.seconds = String(seconds);
      return output;
    },
  },
  openai_image: {
    decodeRequest: function (ctx) {
      const req = validateProtocolRequest(ctx);
      if (req.input === undefined && req.prompt !== undefined) req.input = req.prompt;
      const decoded = decodeGenerative(ctx, req);
      decoded.action = "image_generation";
      return decoded;
    },
    render: function (_ctx, task) {
      const record = taskRecord(task && task.data, task && task.task_id) || {};
      const outputs = Array.isArray(record.outputs) ? record.outputs : [];
      const data = [];
      for (let i = 0; i < outputs.length; i++) {
        const url = mediaURL(outputs[i]);
        if (url) data.push({ url: url });
      }
      return { data: data };
    },
  },
};
