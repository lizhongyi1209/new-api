# 服务器带宽测试说明

本文用于记录服务器公网带宽的测试方法、实测结果和采购新服务器时的对比口径。带宽测试必须在业务低峰和高峰分别执行，并同时关注延迟、下载、上传、抖动、丢包和持续传输能力；单次测速不能代表长期可用带宽。

## 一、本服务器实测记录

### 1. 测试环境

| 项目 | 值 |
|---|---|
| 记录时间 | 2026-08-28 08:07 UTC |
| 测试协议 | HTTPS / IPv6 / TLS 1.3 |
| 测速目标 | Cloudflare Speed，边缘节点 `PDX`（美国） |
| 延迟样本 | 5 次空响应 HTTPS 请求 |
| 下载样本 | 5 次，每次 25,000,000 字节 |
| 上传样本 | 5 次，每次 25,000,000 字节 |
| 换算方式 | `Mbps = bytes_per_second × 8 ÷ 1,000,000` |

为避免泄露服务器信息，记录中不保存公网 IP。`PDX` 是本次请求命中的 Cloudflare 边缘节点，不等同于服务器物理机房位置。

### 2. 汇总结果

| 指标 | 中位数 | 平均值 | 样本范围 |
|---|---:|---:|---:|
| HTTPS 空响应总耗时 | 48.71 ms | 48.16 ms | 40.22～54.13 ms |
| TCP 建连耗时 | 1.04 ms | 1.04 ms | 0.96～1.14 ms |
| TLS 握手完成耗时 | 17.93 ms | 17.83 ms | 16.19～19.13 ms |
| 下载速度 | **2,265.49 Mbps** | 2,102.57 Mbps | 1,399.95～2,378.89 Mbps |
| 上传速度 | **849.84 Mbps** | 773.66 Mbps | 275.81～1,129.04 Mbps |

中位数比单次峰值更适合作为横向比较值。本次 25 MB 样本传输时间很短，所以下载结果反映的是到 Cloudflare `PDX` 边缘节点的短时突发能力，不能直接当作长期保底带宽；上传样本波动明显，也不应按 1.13 Gbps 峰值规划容量。

### 3. 原始样本

| 轮次 | 下载耗时 | 下载速度 | 上传耗时 | 上传速度 |
|---:|---:|---:|---:|---:|
| 1 | 0.088281 s | 2,265.49 Mbps | 0.177141 s | 1,129.04 Mbps |
| 2 | 0.092500 s | 2,162.16 Mbps | 0.235338 s | 849.84 Mbps |
| 3 | 0.086716 s | 2,306.38 Mbps | 0.299676 s | 667.39 Mbps |
| 4 | 0.084073 s | 2,378.89 Mbps | 0.725141 s | 275.81 Mbps |
| 5 | 0.142862 s | 1,399.95 Mbps | 0.211372 s | 946.20 Mbps |

## 二、结果解读

本次测试可以确认：当前服务器到 Cloudflare `PDX` 边缘节点的短时下载能力超过 2 Gbps，短时上传中位数约 850 Mbps。它适合用作后续候选服务器的同口径基线，但不能单独证明以下事项：

- 服务器到中国大陆、欧洲或其他地区用户的实际速度。
- 服务器到模型上游、对象存储或其他云厂商的线路质量。
- 晚高峰的保底带宽、长时间满载能力和丢包率。
- 机房端口标称速率、共享带宽争用情况或服务商限速策略。
- CDN 缓存命中后，Cloudflare 边缘节点到最终用户的下载速度。

如果新服务器只有 1 Gbps 端口，它的单机峰值下载将低于本次到 Cloudflare 边缘节点测得的约 2.27 Gbps 中位数。不过，是否影响真实业务仍取决于并发量、文件大小、CDN 缓存率、跨地区线路和服务商承诺的持续带宽。

## 三、采购容量估算

先根据真实业务计算所需吞吐量：

```text
所需带宽（Mbps）
  = 并发传输数 × 单次平均文件大小（MiB）× 8.388608 ÷ 目标传输时间（秒）
  × 1.2～1.5 安全系数
```

示例：20 个并发请求都需要在 2 秒内传完 10 MiB 文件，理论带宽约为 `839 Mbps`；乘以 1.3 的安全系数后约为 `1.09 Gbps`。这类场景不宜采购只有 1 Gbps 共享端口且没有保底承诺的实例。

采购时应向服务商确认以下指标，不要只比较“端口峰值”：

| 指标 | 需要确认的内容 |
|---|---|
| 端口速率 | 1/2.5/5/10 Gbps，是否共享，是否允许持续跑满 |
| 保底带宽 | 服务等级协议中的承诺值，而非突发峰值 |
| 月流量 | 上行、下行如何计费，超额后的价格或限速规则 |
| 线路质量 | 到主要用户地区、模型上游、Cloudflare、对象存储的延迟和丢包 |
| 高峰表现 | 当地业务高峰时段的中位数和最差样本 |
| 限制策略 | 单连接限速、连接数限制、DDoS 清洗后限速、流量整形 |

## 四、复测命令

以下命令使用 Cloudflare 公共测速接口。测试会实际消耗公网流量；正式复测会分别上传、下载约 125 MB，总流量约 250 MB。

### 1. 检查命中的边缘节点

```bash
curl -fsS --max-time 15 \
  https://speed.cloudflare.com/cdn-cgi/trace \
  | sed -E '/^(ip|warp)=/d'
```

输出中的 `colo` 是 Cloudflare 边缘节点，`loc` 是国家或地区。记录时继续过滤 `ip` 和 `warp` 字段。

### 2. 延迟

```bash
for test_run in 1 2 3 4 5; do
  curl -fsS -o /dev/null \
    --connect-timeout 10 --max-time 20 \
    -w "sample=${test_run} connect=%{time_connect}s tls=%{time_appconnect}s total=%{time_total}s\n" \
    'https://speed.cloudflare.com/__down?bytes=0'
done
```

这里的 `total` 是空响应 HTTPS 请求的完整耗时，不等同于 ICMP `ping` 往返延迟；采购对比时必须使用相同方法。

### 3. 下载

```bash
for test_run in 1 2 3 4 5; do
  curl -fsS -A 'Mozilla/5.0' \
    -H 'Accept-Encoding: identity' \
    -o /dev/null --connect-timeout 10 --max-time 60 \
    -w "sample=${test_run} status=%{http_code} bytes=%{size_download} seconds=%{time_total} bytes_per_second=%{speed_download}\n" \
    "https://speed.cloudflare.com/__down?bytes=25000000&run=${test_run}"
done
```

### 4. 上传

```bash
for test_run in 1 2 3 4 5; do
  head -c 25000000 /dev/zero \
    | curl -fsS -A 'Mozilla/5.0' \
      -H 'Content-Type: application/octet-stream' \
      -o /dev/null --connect-timeout 10 --max-time 60 \
      -X POST --data-binary @- \
      -w "sample=${test_run} status=%{http_code} bytes=%{size_upload} seconds=%{time_total} bytes_per_second=%{speed_upload}\n" \
      "https://speed.cloudflare.com/__up?run=${test_run}"
done
```

## 五、候选服务器验收流程

为了让采购比较有效，当前服务器和每台候选服务器必须使用相同的目标、文件大小、并发数和统计方式：

1. 在业务低峰和目标用户地区的晚高峰各测试一次。
2. 每组至少执行 5 轮，记录中位数、最小值和最大值，不只记录最快一轮。
3. 除 Cloudflare 外，再选择业务真实依赖的模型上游、对象存储和一个主要用户地区进行测试。
4. 使用自有 `iperf3` 节点做 5～10 分钟的单连接和多连接持续测试，以验证保底带宽与丢包；公共测速节点只适合初筛。
5. 用真实大小的图片、视频或请求体进行端到端测试，并结合 Nginx、应用分阶段日志判断瓶颈位于入口、上游还是响应下载。
6. 连续观察至少 24 小时；如果候选服务器只在非高峰跑得快，不应把峰值作为采购容量依据。

建议为每台候选服务器复制“本服务器实测记录”表，并追加测试时间、机房、运营商、端口、月流量、测速目标和原始样本。只有同口径数据才适合做采购判断。
