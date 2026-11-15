DH燃源榜 dh-v1.1.2：让 Agent 安装可恢复、可诊断、可验收。

- 兑换一次性接入码之前，先检查授权参数、状态可写、任务配置及 Windows 三个程序齐全；兑换成功的设备凭据在后续联网前原子保存，瞬断后可直接复用。
- 支持 DH_TICKET / --ticket-file，避免默认把接入码放进进程参数。
- Windows 下载增加 curl → PowerShell → Python 备用通道，macOS/Linux 增加 wget/Python；始终校验 TLS 和 SHA-256。
- 发布 latest.json：六平台带版本固定 URL、大小、SHA-256；站点提供缓存接口 /api/latest.json，带旧缓存标识，不依赖 GitHub API 小时配额。
- 新增 diagnose --json（doctor 别名）及 privacy；命令帮助不执行操作，未知参数非零退出。
- 网络错误区分 DNS/TCP/超时/代理/TLS，保留脱敏底层错误链；setup 与上传使用不同恢复提示。
- Windows status 明确 scheduled、background、launcherPresent、schedulerState，未注册任务返回空 artifacts，查询失败标为 unknown。
- 接入说明增加权限范围、逐项验收、真实后台任务验证、失败处理矩阵与无交互会话限制。
- 保留 v1.1.1 的完整 Windows 后台启动器、不可变历史归档、多设备去重和安全恢复。
