# 将这个 Agent 接入 DH燃源榜

专用服务地址：https://tokenrank.demonh.site 。不得接入 tt.bjxdql.cn 或任何原版服务，也不得调用上游 connect / bind / sync / service 安装流程。

1. 先阅读本文件，判断本机 OS 与 CPU 架构：macOS 对应 darwin，Windows 对应 windows，Linux 对应 linux；x86_64 对应 amd64，aarch64 / Apple Silicon 对应 arm64。
2. 在 DH 网站创建参与身份，复制保存参与密钥。默认私密，公开榜单为用户单独选择，不得替用户公开。不要在聊天、日志、Git 或 URL 中复述密钥。
3. 从 https://github.com/Demon0H/toktrail-collector/releases/latest 下载 `dh-tokenrank-OS-ARCH.zip`（Windows）或 `.tar.gz`（其它系统），以及同一 Release 的 SHA256SUMS。校验 SHA-256 后解压到稳定的个人用户目录（例如 ~/.local/share/dh-tokenrank 或 %LOCALAPPDATA%/DH-TokenRank），不要从临时目录安装定时任务。不需要管理员权限。
4. 可使用仓库 `dh/install/install.sh` 或 `install.ps1` 完成第三步；先阅读脚本再执行。安装器只安装二进制，不读取会话、不配置密钥，也不创建定时任务。
5. 向用户列出当前可用的工具与候选目录，请用户确认允许采集的来源及路径、历史范围。只授权选中的实际目录，不扫描其它用户目录。常见候选如下：
   - Codex: `codex-jsonl=~/.codex/sessions`
   - Claude Code: `claude-code-jsonl=~/.claude/projects`
   - Qwen Code: `qwen-jsonl=~/.qwen/projects`
   - Kimi CLI: `kimi-cli-jsonl=~/.kimi/wire`
   其它来源以 `toktrail help` 和上游 README 为准。必须展开 ~ 为本机绝对路径；不存在的路径跳过。首版专用命令采集被授权目录中的全部有效历史；用户不接受全部历史时不要执行 setup，先告知限制。
6. 将密钥通过当前进程的 `DH_TOKEN` 环境变量传递（不要出现在命令参数或 shell 历史中）。执行：

   `dh-tokenrank setup --grant codex-jsonl=绝对路径 --grant claude-code-jsonl=绝对路径 --install`

   grant 参数按用户确认的来源增减。setup 会验证 HTTPS 和密钥，保存到用户的独立 `~/.dh-tokenrank/dh-config.json`，安装每 30 分钟运行一次的用户级任务。macOS 使用 LaunchAgent，Windows 使用任务计划程序，Linux 使用 systemd 用户 timer。Linux 无用户 systemd 时，报告失败并按用户实际环境处理，不得伪称已安装。
7. 清除当前进程 `DH_TOKEN`。运行 `dh-tokenrank sync`，仅在成功后报告已连接；验证网站真实榜单，私密用户不上榜属于预期行为，不要擅自公开。首次同步可能上传大量历史，请清楚说明。退出码非零时报告具体失败步骤，不要输出凭据或原始会话内容。
8. 给用户留下稳定安装目录、已授权目录、定时任务检查方式、密钥保存位置和卸载命令 `dh-tokenrank uninstall`。卸载只删除任务，保留本地状态；删除服务器数据可在网站输入参与密钥后执行。

## 数据与恢复

dh-tokenrank 复用上游隐私过滤、校验与增量导出事务。只发送用量元数据，不发送提示词、正文或本地路径。网络失败保留队列，下次先重试旧队列；服务端按参与者和 recordKey 去重。不要在同步失败时删除状态目录。密钥被撤销/身份删除时预检失败，停止读取新会话和上传。

专用协议是 `/api/me` 和 `/api/import`，不声称实现上游 `/v1` 设备协议。原始 toktrail 可用于离线 scan / preview / export；其原版在线功能并非 DH 接入入口。
