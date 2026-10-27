# 原生状态页与降智检测：实现接口契约

本文件是 `status-page-quality-detection.md` 的实施约定，字段按下列契约对齐。所有时间戳为 Unix 秒，耗时为毫秒，成功率为 0–100，速度为 token/秒。缺失指标使用 null。Rocket Loader 不在兼容范围。

普通接口统一返回 `{ success: boolean, message?: string, data: T }`；错误使用实际 HTTP 状态码。管理员前缀 `/api/quality-detection`，公共前缀 `/api/status-page`。权限与输入校验必须在服务端执行。

## 管理员接口

- `GET /settings` / `PUT /settings`：`QualitySettings`。PUT 带现有 version，发生并发修改返回409。
- `GET /channels`：`QualityChannel[]`；`PUT /channels/:id` 接收 `QualityProfile`。
- `GET /prompts`：`QualityPrompt[]`；`POST /prompts` 接收 `{ key?, name, content, rule }` 创建预设/新版本（同key递增版本）。`POST /prompts/:id/archive` 接收 `{archived:boolean}`。
- `POST /batches`：`{channel_ids?:number[],group_names?:string[]}`，必带 `Idempotency-Key`；空选择代表所有已启用目标。返回202和`QualityBatch`；重复同键同参数返回已有批次，其他运行冲突返回409。
- `GET /batches?page=1&page_size=20`：`{items:QualityBatch[],total:number,page:number,page_size:number}`。
- `GET /batches/:id`：`{batch:QualityBatch,runs:QualityRun[]}`。
- `POST /batches/:id/cancel`：取消请求，返回data为null。
- `GET /runs/:id`：`QualityRun`；`GET /runs/:id/output?kind=raw|html`：纯文本响应（可下载附件），不以内联HTML响应。`GET /runs/:id/thumbnail`：受鉴权PNG。
- `POST /runs/:id/review`：`{result:'accepted'|'rejected',note:string}`；`POST /runs/:id/baseline`：设基线，返回data为null。

`QualitySettings`：

- version:number、enabled:boolean、interval_minutes:number（5–10080，默认60）、concurrency:number（1–10，默认3）、timeout_seconds:number（60–1800，默认600）。
- max_output_tokens:number（默认8192）、daily_call_limit:number（默认500）、daily_token_limit:number（默认5000000）、retention_days:number（默认14）。
- status_page_enabled:boolean（默认false）、public_groups:Array<{name:string,alias:string}>（默认空）。
- renderer_available:boolean 是只读响应字段，不在数据库设置中保存连接密钥。

`QualityProfile`：channel_id:number、enabled:boolean、model:string、protocol:'chat'|'responses'|'claude'、prompt_version_id:number、reasoning_effort:string、timeout_seconds:number（0继承全局）、public_alias:string、publish_sample:boolean、version:number。

`QualityChannel`：id:number、name:string、groups:string[]、models:string[]、status:number、supported:boolean、unsupported_reason:string、profile:QualityProfile。

`QualityPrompt`：id:number、key:string、version:number、name:string、content:string、rule:'visual'|'interactive'、archived:boolean、created_at:number。

`QualityBatch`：id:number、task_id:string、trigger:'manual'|'scheduled'、status:string、total:number、completed:number、failed:number、cancel_requested:boolean、created_at:number、completed_at:number、summary:string。

`QualityRun`：id:number、public_id:string、batch_id:number、channel_id:number、channel_name:string、model:string、mapped_model:string、prompt_version_id:number、prompt_name:string、status:'queued'|'running'|'completed'|'failed'|'cancelled'|'interrupted'|'skipped'、quality_result:'not_evaluated'|'rules_passed'|'needs_review'|'suspected_degradation'、created_at:number、started_at:number、completed_at:number、first_text_ms:number|null、generation_ms:number|null、input_tokens:number|null、output_tokens:number|null、error_code:string、has_output:boolean、has_html:boolean、has_thumbnail:boolean、render_status:string、review_status:string、review_note:string、baseline_id:number。

不得向前端返回存储路径、凭据、未经脱敏的上游错误。数据库模型可含额外私有字段；HTTP DTO应遵循以上契约。

## 公共状态页

`GET /api/status-page?hours=24&model=&group=&state=&page=1&page_size=20&sort=name` 返回：

- enabled:boolean、paused:boolean、generated_at:number、metrics_data_until:number、last_probe_completed_at:number、window_start:number、window_end:number、bucket_seconds:number。
- total:number、page:number、page_size:number、models:string[]。
- summary:{groups:number,attention:number,channels:number}，摘要基于筛选后全部组而非单页，channels全站去重。
- groups:StatusGroup[]。

`StatusGroup`：public_id:string、name:string（公开别名）、channel_count:number、configured_count:number、fresh_count:number、state:'passed'|'review'|'partial'|'failed'|'unknown'|'stale'|'paused'、success_rate:number|null、avg_latency_ms:number|null、tokens_per_second:number|null、series:Array<{ts:number,value:number|null}>、latest_sample:StatusSample|null。

`GET /api/status-page/groups/:public_id?hours=24&model=&page=1&page_size=20` 返回 `{group:StatusGroup,samples:StatusSample[],total:number,page:number,page_size:number}`。

- `samples` 是所选分组仍保留的历史终态运行（completed/failed/cancelled/interrupted/skipped），包含同一渠道的多次检测；按 `completed_at DESC, id DESC` 稳定排序。`id` 仅用于服务端排序，不公开输出。
- `page` 默认1，范围1–100000；`page_size` 默认20，范围1–100。`total` 是应用模型与公开权限筛选后的历史记录总数；超出末页返回 `samples:[]`，保留实际总数与请求分页参数。无效分页返回400。数据库执行计数、LIMIT和OFFSET，仅详情请求读取所选分组的历史。
- `hours` 仅限制 `group` 的业务统计窗口，不截断检测历史；`group.latest_sample`、状态与当前计数仍使用原有每渠道最新记录聚合，历史分页不会将旧记录变成当前状态。列表接口仍只加载每渠道最新记录，不读取全量历史。
- `model` 精确筛选样本模型；检测配置的代表模型变更后，旧模型记录在渠道仍暴露该模型时可继续显示，但不作为新代表模型的当前证据。未生成或不可用的截图仍保留对应记录，`thumbnail_url:null`，公共查询不会触发模型调用、渲染或自动重新生成。
- 每次读取都要求状态页当前开启、分组当前已配置公开、渠道存在且 `publish_sample` 为真并有非空公开别名，以及渠道当前分组/模型元数据和该分组的模型能力记录匹配。运行还必须具有同一分组的历史 `QualityProbeRunGroup` 关联；移动渠道不会将旧记录带入新分组。渠道临时禁用或检测暂停本身不删除已发布历史，当前状态仍按既有规则标记陈旧或暂停。

`StatusSample`：public_id:string、channel_name:string（仅公开别名）、model:string、status:string、quality_result:string、completed_at:number、stale_after:number、first_text_ms:number|null、generation_ms:number|null、thumbnail_url:string|null、render_status:string。无原始HTML、提示词、内部渠道ID或原始错误。

`GET /api/status-page/artifacts/:public_id/thumbnail` 返回PNG；public_id对应终态运行公开标识，同一运行在多个公开分组中共享此标识和缩略图URL。每次读取均重新检查当前状态页、渠道发布规则、当前分组/模型能力和同组历史归属，代表模型变更不自动撤回仍暴露模型的历史截图。移除模型、能力、公开别名或公开权限后立即拒绝，不使用绕过权限的永久公开存储URL。

## 执行和渲染的内部契约

- `service/quality_detection_executor.go` 定义注入的 `QualityExecutor`；由controller适配层实现并在任务注册前注入。service不导入relay。
- 执行必须调用emit顺序追加可见文本；有上限、不重试、无普通用户扣费和生产路由状态副作用。需要可信完成事件，部分输出不等于完成。
- `service.RenderQualityHTML(ctx context.Context, html []byte) ([]byte,error)` 只返回PNG；`service.QualityRendererAvailable() bool` 表示是否配置。环境变量 `QUALITY_RENDERER_URL` 和 `QUALITY_RENDERER_TOKEN` 不在公开接口输出。渲染请求POST `/render`，JSON `{html:string}`，Bearer认证，服务端视口固定1200×800。
- 渲染不可用不影响模型生成记录；原始文本、HTML和PNG存不可变外部产物。`QUALITY_ARTIFACT_DIR`指定共享产物目录，单机默认`./data/quality`。多实例要求共享挂载。
- 状态页统计使用数据库闭合时间桶；无数据null，不合成曲线，不混合业务成功率与质量结论。
