# M8 测试标准化差异审计报告

**审计时间**：2026-08-19  
**当前 HEAD**：`b198b4b545153db25b77c1753a168af03db8e99a`（M6 完成状态）  
**官方参考**：`upstream/main` @ `e2c7aa7b102c`  
**核心提交**：`e2c7aa7b1` test(web): standardize frontend tests on Vitest (#6569)

---

## 一、核心发现

### 1.1 官方改动

**提交 e2c7aa7b1**（2026-08-15）将所有前端测试从 `node:test` 迁移到 Vitest：

```
37 files changed, 1567 insertions(+), 2171 deletions(-)
净减少：604 行（测试代码更简洁）
```

**改动类型**：
1. ✅ 测试框架迁移：`node:test` → Vitest
2. ✅ 断言库迁移：`node:assert/strict` → Vitest `expect`
3. ✅ 组件测试标准化：统一使用 React Testing Library + jsdom
4. ✅ 测试设置文件：新增 `src/test-setup.ts` + `vitest.config.ts`
5. ✅ 依赖更新：移除 `happy-dom`，新增 `@testing-library/user-event`

### 1.2 当前状态

**部分迁移**：
- ✅ 13 个文件已迁移到 Vitest（在 M5 或更早期间移植）
- ❌ 12 个文件仍使用 `node:test`（Roadmap 提到的失败文件）
- ✅ Vitest 配置已存在（`vitest.config.ts` + `src/test-setup.ts`）

**Roadmap 原文**：
> 当前这 12 个文件（`bun run test` 全量下失败、其余 13 文件 64 项通过）为：`json-code-editor-utils`、`json-code-editor`、`dropdown-menu`、`oauth-callback-mode`、`channel-field-update`、`channel-table-row-id`、`flow-selection`、`flow`、`redemption-form`、`tool-price-validation`、`cost-display`、`tool-surcharge`。失败原因统一是 `Cannot bundle Node.js built-in "node:test"`，与 M5 无关。

---

## 二、迁移模式分析

### 2.1 导入语句迁移

**旧写法**（node:test）：
```typescript
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
```

**新写法**（Vitest）：
```typescript
import { describe, expect, test } from 'vitest'
```

### 2.2 断言迁移

| node:assert | Vitest expect | 说明 |
|------------|--------------|------|
| `assert.equal(a, b)` | `expect(a).toBe(b)` | 严格相等 |
| `assert.deepEqual(a, b)` | `expect(a).toEqual(b)` | 深度相等 |
| `assert.strictEqual(a, b)` | `expect(a).toBe(b)` | 严格相等（同上） |
| `assert.ok(x)` | `expect(x).toBeTruthy()` | 真值断言 |
| `assert.match(str, regex)` | `expect(str).toMatch(regex)` | 正则匹配 |
| `assert.throws(() => fn())` | `expect(() => fn()).toThrow()` | 异常断言 |

### 2.3 组件测试迁移

**旧写法**（直接 DOM 操作 + happy-dom）：
```typescript
import { describe, test } from 'node:test'
import assert from 'node:assert/strict'

test('renders component', () => {
  // 手工 DOM 操作
  const div = document.createElement('div')
  // ...
  assert.ok(div.innerHTML.includes('expected'))
})
```

**新写法**（React Testing Library + jsdom）：
```typescript
import { describe, expect, test } from 'vitest'
import { render, screen } from '@testing-library/react'

test('renders component', () => {
  render(<MyComponent />)
  expect(screen.getByText('expected')).toBeInTheDocument()
})
```

---

## 三、需要迁移的 12 个文件

### 3.1 文件清单

| # | 文件路径 | 测试类型 | 行数估算 | 优先级 |
|---|---------|---------|---------|--------|
| 1 | `src/components/json-code-editor/__tests__/json-code-editor-utils.test.ts` | 单元 | ~150 | 高 |
| 2 | `src/components/json-code-editor/__tests__/json-code-editor.test.tsx` | 组件 | ~250 | 高 |
| 3 | `src/components/ui/dropdown-menu.test.tsx` | 组件 | ~50 | 中 |
| 4 | `src/features/auth/lib/__tests__/oauth-callback-mode.test.ts` | 单元 | ~100 | 高 |
| 5 | `src/features/channels/lib/__tests__/channel-field-update.test.ts` | 单元 | ~80 | 中 |
| 6 | `src/features/channels/lib/__tests__/channel-table-row-id.test.ts` | 单元 | ~60 | 低 |
| 7 | `src/features/dashboard/lib/flow-selection.test.ts` | 单元 | ~120 | 中 |
| 8 | `src/features/dashboard/lib/flow.test.ts` | 单元 | ~600 | 高 |
| 9 | `src/features/redemption-codes/lib/redemption-form.test.ts` | 单元 | ~150 | 中 |
| 10 | `src/features/system-settings/models/__tests__/tool-price-validation.test.tsx` | 组件 | ~180 | 中 |
| 11 | `src/features/usage-logs/components/__tests__/cost-display.test.tsx` | 组件 | ~180 | 中 |
| 12 | `src/features/usage-logs/lib/__tests__/tool-surcharge.test.ts` | 单元 | ~70 | 低 |

**总计**：约 1990 行测试代码需要迁移。

### 3.2 优先级分级

**高优先级**（4 个，~1100 行）：
- `json-code-editor-utils`：JSON 编辑器核心逻辑
- `json-code-editor`：JSON 编辑器组件（M5 刚移植的 AutoGroups 编辑器依赖）
- `oauth-callback-mode`：OAuth 认证流程（安全相关）
- `flow`：Dashboard 流程逻辑（600 行大文件）

**中优先级**（6 个，~770 行）：
- `dropdown-menu`、`channel-field-update`、`flow-selection`、`redemption-form`、`tool-price-validation`、`cost-display`

**低优先级**（2 个，~130 行）：
- `channel-table-row-id`、`tool-surcharge`

---

## 四、依赖变更

### 4.1 需要添加的依赖

**package.json 改动**：
```diff
 "devDependencies": {
   ...
+  "@testing-library/user-event": "^14.6.1",
-  "happy-dom": "^20.11.1",
   ...
 }

 "overrides": {
   ...
-  "dompurify": "3.4.11",
+  "dompurify": "3.4.13",
   ...
 }
```

**说明**：
- `@testing-library/user-event`：模拟用户交互（点击、输入等）
- 移除 `happy-dom`：官方改用 `jsdom`（已存在）
- `dompurify` 升级：安全补丁

### 4.2 已存在的配置

✅ `vitest.config.ts`：
```typescript
export default defineConfig({
  resolve: {
    alias: { '@': path.resolve(__dirname, './src') },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test-setup.ts'],
    clearMocks: true,
    restoreMocks: true,
    include: ['src/**/*.{test,spec}.{ts,tsx}'],
  },
})
```

✅ `src/test-setup.ts`：
- i18next 初始化（避免测试中 t() 报错）
- jsdom 全局对象模拟（`matchMedia`、`ResizeObserver`、`scrollIntoView` 等）
- React Testing Library 自动清理（`afterEach(() => cleanup())`）

---

## 五、迁移策略

### 5.1 自动迁移 vs 手工迁移

**自动迁移可行性**：❌ 低

**原因**：
1. 断言风格多样（`assert.equal` / `assert.deepEqual` / `assert.ok` / `assert.match`）
2. 组件测试需要重写为 RTL 风格（不是简单替换）
3. 部分测试逻辑需要调整（例如异步断言、DOM 查询）

**推荐策略**：手工逐文件迁移，参考官方 diff

### 5.2 分批迁移计划

**批次 1**（高优先级，M8.1）：
- `json-code-editor-utils.test.ts`
- `json-code-editor.test.tsx`
- `oauth-callback-mode.test.ts`

**批次 2**（flow 大文件，M8.2）：
- `flow.test.ts`（600 行，单独处理）

**批次 3**（中优先级，M8.3）：
- 剩余 6 个中优先级文件

**批次 4**（低优先级，M8.4）：
- 剩余 2 个低优先级文件

---

## 六、官方改动示例

### 6.1 简单单元测试迁移

**文件**：`src/lib/server-error-message.test.ts`

**官方 diff 模式**：
```diff
-import assert from 'node:assert/strict'
-import { describe, test } from 'node:test'
+import { describe, expect, test } from 'vitest'

 describe('server error message mapping', () => {
   test('maps the active-session limit to recovery instructions', () => {
     const message = getServerErrorMessageKey({ code: 'AUTH_SESSION_LIMIT' })

-    assert.match(message ?? '', /Sign out other sessions/)
-    assert.match(message ?? '', /reset your password/)
+    expect(message ?? '').toMatch(/Sign out other sessions/)
+    expect(message ?? '').toMatch(/reset your password/)
   })
 })
```

### 6.2 组件测试迁移

**文件**：`src/components/ui/dropdown-menu.test.tsx`

**官方 diff 模式**：
```diff
-import { describe, test } from 'node:test'
-import assert from 'node:assert/strict'
+import { describe, expect, test } from 'vitest'
+import { render, screen } from '@testing-library/react'

 describe('dropdown menu', () => {
   test('renders trigger and content', () => {
-    // 手工 DOM 操作
-    const container = document.createElement('div')
-    // ...
-    assert.ok(container.innerHTML.includes('Menu'))
+    render(<DropdownMenu><DropdownMenuTrigger>Menu</DropdownMenuTrigger></DropdownMenu>)
+    expect(screen.getByText('Menu')).toBeInTheDocument()
   })
 })
```

---

## 七、验收标准

### 7.1 测试通过标准

```bash
cd /home/ubuntu/new-api/web/default

# 1. 安装依赖
bun install

# 2. 运行全部测试
bun run test

# 3. 验收通过条件
# ✅ 所有 25 个文件（13 已迁移 + 12 待迁移）全部通过
# ✅ 无 "Cannot bundle Node.js built-in" 错误
# ✅ 测试输出显示总数 > 64 项（当前已通过数）
```

### 7.2 代码质量标准

```bash
# 类型检查
bun run typecheck

# 格式检查
bun run format:check

# Lint 检查
bun run lint
```

---

## 八、风险评估

### 8.1 技术风险

| 风险项 | 等级 | 缓解措施 |
|-------|------|---------|
| 迁移后测试失败 | 🟡 中 | 逐文件对比官方 diff，保持测试覆盖不变 |
| 断言语义变化 | 🟢 低 | `assert.deepEqual` ≈ `expect().toEqual()`，语义一致 |
| 组件测试重写 | 🟡 中 | 参考官方 RTL 模式，测试行为不变 |
| 依赖冲突 | 🟢 低 | 仅添加 `@testing-library/user-event`，无冲突 |

### 8.2 业务影响

**影响范围**：✅ 仅前端测试，不影响生产代码

**回滚方案**：
- 测试文件可独立回滚
- 生产代码不受影响

---

## 九、与 RelayKit 的关系

**时间线**：
- RelayKit (86ac0f774)：2026-07-27
- M6 AutoGroups (0ab020206)：2026-08-??
- **测试标准化 (e2c7aa7b1)：2026-08-15**

**结论**：测试标准化 **晚于** RelayKit，**早于** M6 AutoGroups。

**依赖关系**：
- ❌ **不依赖** RelayKit（纯前端测试迁移）
- ❌ **不依赖** M6/M7 后端改动
- ✅ **独立** 进行，不阻塞其他单元

**M7 决策影响**：
- 无论是否引入 RelayKit，M8 测试迁移都可以独立进行
- 建议在 M7 计费/安全修复的同时，并行进行 M8 测试迁移

---

## 十、推荐执行计划

### 10.1 M8.1：高优先级测试迁移（1-2 天）

**任务**：
1. 安装 `@testing-library/user-event`
2. 更新 `dompurify` 到 3.4.13
3. 迁移 3 个高优先级文件：
   - `json-code-editor-utils.test.ts`
   - `json-code-editor.test.tsx`
   - `oauth-callback-mode.test.ts`

**验收**：
- 这 3 个文件测试通过
- 原有 13 个文件仍然通过

### 10.2 M8.2：flow 大文件迁移（1 天）

**任务**：
- 迁移 `flow.test.ts`（600 行，需仔细对比官方 diff）

**验收**：
- flow 测试通过
- 测试覆盖不减少

### 10.3 M8.3：中优先级迁移（1-2 天）

**任务**：
- 迁移剩余 6 个中优先级文件

**验收**：
- 全部测试通过（22/25 文件）

### 10.4 M8.4：低优先级迁移（半天）

**任务**：
- 迁移最后 2 个低优先级文件

**验收**：
- 全部 25 个测试文件通过
- `bun run test` 无错误

**总耗时**：3-4 天

---

## 十一、下一步行动

### 立即决策点

**问题 1**：M8 测试迁移的时机？
- **选项 A**：在 M7 计费/安全修复的同时并行进行（推荐）
- **选项 B**：等 M7 完成后再开始
- **选项 C**：推迟到 M9

**推荐**：选项 A（并行），理由：
- M8 是纯前端测试，不影响后端
- 不依赖 M7 后端改动
- 可以在等待后端测试时进行前端测试迁移
- 提前完成 M8，为 M7 后端改动提供前端测试保障

**问题 2**：是否移除 `happy-dom` 依赖？
- **推荐**：✅ 移除，官方已不再使用
- **风险**：🟢 低，当前已使用 jsdom

---

## 十二、总结

**M8 范围**：
- ✅ 测试框架标准化：`node:test` → Vitest
- ✅ 12 个文件迁移（~1990 行）
- ✅ 依赖更新：+`@testing-library/user-event`，-`happy-dom`
- ✅ 独立进行，不阻塞 M7

**工作量**：
- 预计 3-4 天
- 可与 M7 并行

**风险等级**：🟢 低风险
- 仅测试代码，不影响生产
- 回滚简单

**推荐策略**：
- 在 M7 后端审计/移植期间，并行进行 M8 前端测试迁移
- 分 4 批完成，逐步验收

---

**用户决策已落实**（2026-08-19）：
1. ✅ M8 与 M7 并行 → 实际是 M7 完成后立即进入 M8
2. ✅ 移除 `happy-dom` → 已移除，`bun install` 生效
3. ✅ 立即开始 M8.1 → **M8.1–M8.4 全部完成**

## M8 最终结果（2026-08-19）

**✅ 全部完成**：25/25 测试文件通过，135 项测试全绿（M8 前 13 通过 / 12 失败、64 项）。

**依赖**：`+ @testing-library/user-event@14.6.5`、`+ dompurify@3.4.13`（dependencies + overrides 两处）、`- happy-dom`。

**本文档第五节"自动迁移可行性：❌ 低 / 推荐手工逐文件迁移"的判断偏保守。** 实际 12 个文件里 **9 个可以直接采用官方版本全文覆盖**，只有 3 个需要真正动手：
- `json-code-editor-utils.test.ts`：手工替换断言（官方版也在，但我们先手工做完了）
- `json-code-editor.test.tsx`：真正重写为 RTL
- `redemption-form.test.ts`：**官方没有这个文件**（定制独有），只能手工迁移

有效的判定流程（先判定再动手，能省掉大量手工改写）：
```bash
f=src/features/dashboard/lib/flow.test.ts
git cat-file -e "e2c7aa7b1:web/$f"                    # 官方是否有
diff <(git show "e2c7aa7b1:web/$f" | grep -oE "(test|it)\('[^']*'" | sort) \
     <(grep -oE "(test|it)\('[^']*'" "$f" | sort)      # 用例名是否一致
diff <(git show "e2c7aa7b1:web/$f" | grep -E "^import") <(grep -E "^import" "$f")  # 源模块是否一致
diff <(git show "e2c7aa7b1:web/$f" | sed -n '1,18p') <(sed -n '1,18p' "$f")        # 版权头是否一致
```

⚠️ **采用官方组件测试前先核对组件契约**：官方改用 `getByRole('img')` / `getByRole('spinbutton')` 这类语义化查询，依赖组件真的设了对应 role。本次已 grep 确认 `log-cost-display.tsx:58` 有 `role='img'`、`tool-price-settings.tsx:331` 是 `type='number'` 且 `:389` 按钮文案为 `Save tool prices`。若定制版改过组件，应该改测试而不是改组件。

**行数实测更正**：`flow.test.ts` 是 815 行（本文档原估 ~600）；12 个文件合计 2118 行（原估 ~1990）。

## 🐞 M8 顺带修复的生产缺陷

`web/default/src/lib/format.ts` 的 `getEditableQuotaStep()` 原本是 `10 ** -getCurrencyFractionDigits(0)`。`digitsSmall` 默认 4，而 **V8（Chrome/Node）算 `10 ** -4` 得 `0.00009999999999999999`**，`10 ** -5` 同样错；Bun 的 JSC 算出精确的 `0.0001`，所以本地用 bun 跑业务代码时完全看不出来。

该值直接作为兑换码金额输入框的 `step` 属性。浏览器按 `step` 校验输入，step 不精确时正常的 4 位小数金额可能被判 invalid。生产跑 Chrome，属真实用户可见缺陷。

已改为 `1 / 10 ** getCurrencyFractionDigits(0)`（0–10 位小数在 V8 上全部精确），并加注释说明为何不能写负指数。

**为什么以前没发现**：`redemption-form.test.ts` 本来就有 `assert.equal(getEditableQuotaStep(), 0.0001)`，但该文件因 `node:test` 无法打包，**从未真正执行过**。迁到 Vitest（跑在 Node/V8）后立刻失败。

→ **通用规则：那批"因 node:test 而失败"的文件不是纯框架噪音，里面可能藏着从未执行的真实断言。迁移后第一次失败，先当成真 bug 查源码。**

同类写法 `src/lib/currency.ts:292`、`src/features/dashboard/lib/charts.ts:62` 也用 `Math.pow(10, -digits)`，但两处值都随即经 `.toFixed(digits)` 收敛回 `0.0001`，无可观测缺陷，**故意未改**（避免扩大改动面）。

## M8 验收证据

```
bun run test          → 25 passed (25)，135 项全部通过
grep -rn "node:test\|node:assert\|happy-dom" src/  → 无输出
bun run typecheck     → 通过
bun run format:check  → 通过（1070 文件）
bun run lint          → 375 error；stash 掉 M8 全部改动后同为 375 → 零新增
bun run build         → 通过（改了生产文件 format.ts，必须跑构建）
```

改动统计：`15 files changed, 512 insertions(+), 800 deletions(-)` —— 12 个测试文件 + `package.json` + `web/bun.lock` + **`src/lib/format.ts`（唯一生产代码改动，6+/1-）**。

全程只在本机工作树和本地 bun 内操作，未触碰生产容器、数据库或子站。
