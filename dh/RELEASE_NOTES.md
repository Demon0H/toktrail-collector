DH燃元榜 dh-v1.1.4：修复 DeepSeek Harness 源在「空 assistant/attempt」后整轮作废导致的用量丢失。

- 修复 deepseek-harness 适配器：同一 step 里先出现一条不携带任何用量样本的 `assistant/attempt`（DSH 在图像卸载、上下文重建时会发），随后才用 `assistant/message` 真正结算时，不再把整个 turn 判为 invalid。空 attempt 不消耗该 step 唯一的结算名额，该 turn 的全部用量正常导出。
- 防重复计数语义不变：同一 step 两条都带用量样本的结算仍然作废整轮；带非法样本（负数、超出 2^53、字符串计数）的 attempt 仍然 fail closed。
- 升级后首次同步会自动回填历史：受影响会话在 known-records 里为空，重跑 `sync` 即可补传，不需要 `repair`。
- 新增 4 个回归用例：7 行合成 fixture 正向、连续多个空 attempt、重复结算负向、空 attempt 后直接 step/end。
- 采集口径、recordKey/groupKey、隐私白名单、状态目录格式均未变化；其它适配器未改动。

DH燃元榜 dh-v1.1.2：让 Agent 安装可恢复、可诊断、可验收。

- 兑换一次性接入码之前，先检查授权参数、状态可写、任务配置及 Windows 三个程序齐全；兑换成功的设备凭据在后续联网前原子保存，瞬断后可直接复用。
- 支持 DH_TICKET / --ticket-file，避免默认把接入码放进进程参数。
- Windows 下载增加 curl → PowerShell → Python 备用通道，macOS/Linux 增加 wget/Python；始终校验 TLS 和 SHA-256。
- 发布 latest.json：六平台带版本固定 URL、大小、SHA-256；站点提供缓存接口 /api/latest.json，带旧缓存标识，不依赖 GitHub API 小时配额。
- 新增 diagnose --json（doctor 别名）及 privacy；命令帮助不执行操作，未知参数非零退出。
- 网络错误区分 DNS/TCP/超时/代理/TLS，保留脱敏底层错误链；setup 与上传使用不同恢复提示。
- Windows status 明确 scheduled、background、launcherPresent、schedulerState，未注册任务返回空 artifacts，查询失败标为 unknown。
- 接入说明增加权限范围、逐项验收、真实后台任务验证、失败处理矩阵与无交互会话限制。
- 保留 v1.1.1 的完整 Windows 后台启动器、不可变历史归档、多设备去重和安全恢复。
