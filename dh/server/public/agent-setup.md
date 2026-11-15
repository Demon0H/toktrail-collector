# DH燃源榜 Agent 自动接入说明

服务地址固定为 https://tokenrank.demonh.site 。本 fork 专用程序是 `dh-tokenrank`，不得使用原版 `toktrail connect/bind/sync/service` 接入其他站点。

## 一句话接入流程

用户先在网页通过 GitHub 登录，再填写昵称，城市由服务端依据访问者 IP 自动查询（定位不保证精确，失败显示未知，不阻碍接入）。点击“复制给 Agent”会使用 GitHub 身份生成 15 分钟有效、仅可兑换一次的接入码。接入码只用于绑定新的采集设备，不是长期设备密钥。不要在日志中输出设备密钥。GitHub 登录后默认公开昵称、IP 归属城市和用量；网页会明确提示，并允许撤回。Agent 不修改账号公开状态。

1. 确认本机系统与架构：macOS=darwin，Windows=windows，Linux=linux；x86_64=amd64，Apple Silicon/aarch64=arm64。下载 https://github.com/Demon0H/toktrail-collector/releases/latest 中对应的 `dh-tokenrank-OS-ARCH.zip`（Windows）或 `.tar.gz`。同时下载同一版本 SHA256SUMS 并校验，再解压到稳定用户目录，不需要管理员权限。包内含 dh-tokenrank 与用于离线诊断的上游 toktrail。
2. 可先读取并运行同一 Release 的 install.ps1 或 install.sh。脚本只安装二进制，不读取会话、不绑定设备、不创建定时任务。
3. **必须使用 dh-v1.1.1 或更新版本。** 运行 `dh-tokenrank version` 检查，运行 `dh-tokenrank discover` 获取本机全部受支持的本地工具及默认目录，并向用户展示。discover 只检查目录存在，不读对话正文。不要因为当前使用 Codex 就只配置 Codex。
4. 用户复制的说明已选择“本机全部受支持 AI 工具及历史”；使用 `dh-tokenrank setup --ticket 一次性接入码 --all --install`。setup 将兑换设备专用密钥，存入 `~/.dh-tokenrank/dh-config.json`，不会把长期管理密钥放进 Agent 提示词。每 30 分钟运行一次用户级任务；macOS 使用 LaunchAgent，Windows 使用任务计划程序，Linux 使用 systemd 用户 timer。Linux 没有用户 systemd 时如实报告，不伪称已安装。
5. `--all` 表示授权当前用户下所有受支持本地工具的默认数据目录，包括全部有效历史；每次同步重新发现，后续新增工具也能纳入。自定义数据目录可额外追加 `--grant source=绝对路径`。不扫描其他用户目录。若用户不接受完整历史/全部来源，就停止 all 流程，改用其明确选择的 --grant；目前没有历史时间窗口开关。
6. 如果已有配置，保留状态，先运行 `dh-tokenrank status`；有 missingArchives 时先运行 `dh-tokenrank repair`。用新接入码执行 `dh-tokenrank setup --ticket 接入码 --reconnect --all --install` 可核对同一账号并重绑同一设备，不增加设备或重复计数；跨账号会拒绝，请另用 --state-dir。有效本地密钥也可直接执行 `dh-tokenrank setup --all --install` 复用配置，不必设置 DH_TOKEN。切换身份使用独立 --state-dir；接入码已过期/使用时请用户在网站重新生成。已有配置的 setup 支持用 DH_TOKEN 环境变量复用现有密钥，但不要把密钥放进命令参数、历史记录或聊天。
7. 执行 `dh-tokenrank sync`，检查退出码和上传结果。某个来源失败会记录来源名称并继续处理其他工具，不能把部分失败报告成全部成功。网络或 ACK 失败保留队列，下次先重试；服务端按参与者和 recordKey 去重。
8. 向用户报告已发现并授权的工具、首次同步结果和任务状态，留下安装路径与 `dh-tokenrank uninstall`。卸载仅移除定时任务，保留状态。私密身份不上公开榜单是预期行为。网页 GitHub 登录会话可以撤回公开或永久删除身份，设备密钥只有查询自身与上传权限，不能删除/公开。

## 覆盖范围

支持 Codex、Claude Code、DeepSeek Harness、WorkBuddy、Qwen Code、Kimi CLI、Kimi Code、ZCode、Hermes Agent、OpenClaw、Pi、Oh My Pi、CodeBuddy IDE、OpenCode 的本地用量。默认目录直接复用上游 registry（如 Codex ~/.codex/sessions、Claude ~/.claude/projects、Kimi ~/.kimi/sessions），包括注册的候选布局。

这是“本机所有受支持本地工具”，不等于所有软件。Cursor 的用量需要账户网络授权，没有本地计数适配器，暂不纳入 DH 的 --all；不得假装采集成功，也不得替用户读取登录凭据。自定义路径、其他账号、WSL 或容器内日志需要在相应环境单独配置或显式授权。

仅上传隐私过滤后的计数、时间、工具和模型，不上传提示词、正文或本地路径。公网 HTTPS 在独立 Nginx 站点终止，加密回源到内部 8081。专用 API 是 /api/enroll、/api/me、/api/import。

## 安装失败后的补救

Windows 安装目录必须同时包含 dh-tokenrank.exe、toktrail.exe 和 toktrail-background.exe。后台任务调用 launcher，再启动 dh-tokenrank periodic；不要手工安装 legacy 任务绕过缺文件。

旧版 dh-v1.0.0 / dh-v1.1.0 曾误删已同步批次。先备份整个 ~/.dh-tokenrank，升级完整发行包，再运行 status 和 repair。repair 只处理已确认的历史：从账号认证接口下载服务器实际接收的记录，Codex/DeepSeek 与当前本地标准化数据逐项核对，数量或用量不符即停止；Claude 的服务器历史快照独立保留，message-progress.json 不变。所有来源都核对后才原子更新账本，并备份旧账本。它不会上传历史、清空游标或伪造原始批次；新快照明确标记为 verified-server-recovery。缺少未确认批次时必须恢复原始备份，不能假定已上传。

然后 setup --all --install（复用原密钥）与 sync，应均为退出码0。Windows status 应显示 scheduled:true、background:true、missingArchives:0。必要时在网站重新登录，再点“配置失败？重新生成修复说明”，用一次性码 --reconnect；服务器保留设备身份，即使旧凭据刚被轮换/撤销，也可在同一账号下再次补救。

## 多设备与数据校验

同一 GitHub 账号可绑定最多32台活跃设备，独立密钥与同步状态，历史按账号+recordKey 去重。重复键的时间、工具、模型、计数发生冲突时整批拒绝，其他已成功批次保留。每设备每分钟最多120批；未来超过5分钟及2000年前的时间拒绝。单条已知 Token 总数超过10亿会隔离，仍保存在账号历史，但不进入公开榜单。用户可在设备区查看最近成功同步、失败来源、异常条数并撤销设备。旧版无法追溯来源的记录明确归为未区分设备，不冒认归属。

这些措施保护账号、重试与数据一致性，不能证明用户本地日志未被人为修改；榜单明确属于客户端自报数据。
