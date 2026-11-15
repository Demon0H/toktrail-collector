# DH燃元榜 Agent 自动接入说明

固定站点：https://tokenrank.demonh.site 。专用命令是 `dh-tokenrank`，不要使用上游 `toktrail connect/bind/sync/service` 连接其他站点。

## 授权范围与凭据

网页先要求 GitHub 登录、填写昵称，再生成15分钟有效的一次性接入码。城市由服务端按访问者 IP 查询，失败显示未知。登录默认公开昵称、城市和用量，用户可在网站撤回公开；Agent 不修改公开设置。只读取公开 GitHub 身份，不申请仓库权限。

用户复制的说明授权：读当前用户所有受支持工具的本地用量及历史，写 `~/.dh-tokenrank` 和稳定的用户级安装目录（Windows `%LOCALAPPDATA%\DH-TokenRank`；macOS/Linux `~/.local/share/dh-tokenrank`），联网访问本站及官方 GitHub Release，创建每30分钟的用户级任务。不得扫描其他系统用户或读取未授权账号。若 Agent 沙箱阻止这些路径/联网/任务操作，按所在环境的权限流程申请所需权限；未获授权就报告受阻，不能绕过限制或谎报完成。

接入码用于兑换设备专用密钥，长期密钥仅保存在私有状态目录。优先以 `DH_TICKET` 环境变量或私有 UTF-8 文件 `--ticket-file FILE` 传码，不要默认用 `--ticket` 把码放进进程参数。执行后清除临时环境变量/临时码文件，不输出或提交任何凭据。`DH_TOKEN` 用于已有设备身份，不要把它当成一次性接入码。

## 下载与本地预检

1. 识别系统/架构：macOS=darwin，Windows=windows，Linux=linux；x86_64=amd64，Apple Silicon/aarch64=arm64。
2. 获取 https://tokenrank.demonh.site/api/latest.json ：包含 `version`、`minVersion`、六平台 `assets[OS-ARCH].url/sha256/bytes`、安装脚本及其哈希。URL 必须属于 `https://github.com/Demon0H/toktrail-collector/releases/download/`。推荐 dh-v1.1.2 或更新，最低安全归档版本 dh-v1.1.1；清单 `stale:true` 表示上次验证的缓存，不是刚检查的新版本。
3. 下载匹配资产并核对 SHA-256，再解压到稳定用户目录；不要在会被清理的临时目录注册任务。清单不可用时访问 https://github.com/Demon0H/toktrail-collector/releases/latest ，下载同版归档和 SHA256SUMS。Windows 包必须同时有 `dh-tokenrank.exe`、`toktrail.exe`、`toktrail-background.exe`；其他平台有前两个二进制。
4. 可先下载、检查再运行同一 Release 的 `install.ps1` / `install.sh`，它们仅安装二进制，不读会话、不兑换码、不注册任务。Windows 下载依次尝试 curl.exe → PowerShell → Python urllib；macOS/Linux 为 curl → wget → Python。所有通道都验证 TLS。若均失败，请用户用浏览器下载归档和 SHA256SUMS，校验后解压；不得禁用证书验证。MSYS 不一定有 unzip，Windows 可用 `Expand-Archive`、`tar -xf` 或 Python zipfile；带空格路径必须引用。
5. 运行 `dh-tokenrank version`、`dh-tokenrank discover --json`，展示发现的来源与目录。discover 只检查目录存在，不读正文。不要只配置正在执行安装的那一种 Agent。

## 接入或恢复

- 新安装：把码注入 `DH_TICKET` 后执行 `dh-tokenrank setup --all --install`。程序在兑换码前检查授权参数、路径、状态目录可写、任务配置，以及 Windows 三个程序是否齐全。后续系统任务注册仍可能被系统策略拒绝；如已兑换，设备凭据先保存，直接复用配置重试即可。
- 已有配置：先 `dh-tokenrank status --json`，保留状态。密钥有效时，不带接入码执行 `dh-tokenrank setup --all --install`；接入码只在需要核对账号/重绑时注入 `DH_TICKET`，再执行 `dh-tokenrank setup --reconnect --all --install`，服务器只允许原账号原设备。接入码已使用/过期时，请用户重新登录网站并点“配置失败？重新生成修复说明”；不要反复重放旧码。跨账号请使用独立 `--state-dir`。
- `--all` 读取当前用户下全部受支持默认目录及有效历史，每次同步重新发现新增工具。自定义目录额外用 `--grant source=绝对路径`。若用户不授权全部历史/全部工具，停止 all 流程并使用明确选定的 grant；目前没有历史时间窗口参数。自定义状态目录必须在后续 status/sync/repair/diagnose/uninstall 都一致传入。
- 若 `missingArchives>0`，先备份整个状态目录，再执行 `dh-tokenrank repair`。旧 dh-v1.0.0 / dh-v1.1.0 曾误删已确认批次。repair 从服务器取本账号已接收记录，与本地 Codex/DeepSeek 标准化数据严格核对后创建有标记的恢复快照，Claude 保留独立服务器历史快照且不动 message-progress。仅全部核对通过才原子更新账本并备份旧账本，不上传历史、不清空游标。未确认批次丢失须恢复原始备份；核对失败就停止并报告，不能伪造原批次。
- 运行 `dh-tokenrank sync`。部分来源失败会继续其他来源并以非零退出，不能宣称全部成功。已接收用量保留，未确认批次保留供重试；服务端按账号和 recordKey 去重。首次没有任何本地用量时可同步成功但 syncedBatches 为0，应如实说明。

## 必须逐项验收并回报

1. 版本、稳定安装路径、发布归档 SHA-256（与清单/同版本 SHA256SUMS 一致）；Windows 另报三个 exe 的本地 SHA-256。归档清单没有单独 exe 哈希，不要声称它们也与清单逐项匹配。
2. `discover --json` 的来源/目录及实际授权范围；不要回传会话正文、长期密钥或完整诊断中的私人路径。
3. `sync` 退出码0且输出 `DH sync complete. Destination: https://tokenrank.demonh.site`。回报实际输出的 accepted 计数；如果本次无新批次，明确写“本次无新批次”，不得编造非零数量。
4. `status --json` 的 `pendingBatches:0`、`missingArchives:0`、`syncedBatches` 实际值、`scheduled:true`、`schedulerState:"registered"`。Windows 还应有 `background:true`、`launcherPresent:true`。launcherPresent 只证明文件存在，不能证明任务存在；查询失败的 unknown 不是安装成功。
5. 验证后台真实执行。Windows 从 schedulerArtifacts 得到任务名（去掉 `schtasks:`），用 `schtasks /Run /TN "任务名"`，等待任务完成后检查最近一次 LastTaskResult=0，及状态目录 `logs` 中本次 `collector finished exit=0`，再复查队列。不要把旧日志/旧 Last Result 当作本次成功。任务使用 InteractiveToken，CI/无交互桌面可能能注册却无法运行，此时报告“已注册，尚未验证后台执行”。macOS 检查用户 LaunchAgent，Linux 检查 systemd 用户 timer；无可用用户服务管理器时如实报告。
6. 任何一项失败，标出“已完成 / 未完成 / 下一步”，不能只说安装成功。提供 `dh-tokenrank uninstall`（只删所拥有任务，保留状态）及复测方法。

## 排障矩阵

先运行 `dh-tokenrank diagnose --json`（`doctor` 是别名）：离线输出版本、状态可写性、授权来源文件数、队列、任务及脱敏后的最近错误，不上传数据。输出可能含本地目录，分享前删去私人路径。`privacy` 显示采集政策；`COMMAND --help` 只显示该命令说明，不执行采集或创建任务，未知命令/参数以非零退出。

| 错误/状态 | 下一步 |
| --- | --- |
| background launcher is unavailable / incomplete Windows release | 升级并解压完整三程序包，再 setup --all --install；不要手装 legacy 任务绕过 |
| read history batch / missingArchives>0 | 备份状态，升级，repair；核对失败停止，未确认文件从备份恢复 |
| DNS / TCP / timeout / proxy / TLS certificate | 保留确切底层原因，手动 sync 重试；瞬断不用重装，证书错误不得跳过验证 |
| setup code used or expired / HTTP401 | 网站重新登录生成新码；已有有效配置可无 ticket 复用，勿删除状态 |
| state is in use by another process | 等待当前同步结束；不要删除锁文件或并发重装 |
| participation key differs from existing identity | 不覆盖身份；跨账号独立 --state-dir，同账号使用 --reconnect |
| schedulerState unknown / 注册后运行未验证 | 查看 schedulerError/本次任务日志，报告未验证，勿把 launcher 文件存在当成功 |

网络错误按阶段区分：只有上传错误提到 pending batch retained，setup 不暗示存在待上传批次。错误中隐藏参与密钥、一次性码和旧凭据。重新登录生成说明不清零排行榜；失败会影响尚未采集/上传的增量，已有成功数据不回滚。

## 覆盖范围与校验边界

支持 Codex、Claude Code、DeepSeek Harness、WorkBuddy、Qwen Code、Kimi CLI、Kimi Code、ZCode、Hermes Agent、OpenClaw、Pi、Oh My Pi、CodeBuddy IDE、OpenCode 的本地计数；默认候选目录复用上游 registry。Cursor 需要账号网络授权，暂不属于 DH --all，不得读取其凭据假装已支持。WSL/容器、其他账号需在相应环境单独配置或明确授权。

仅上传计数、时间、工具和模型，不上传提示词、正文或本地路径。同一 GitHub 账号最多32台活跃设备，独立密钥/状态，按账号+recordKey 去重；重复键内容冲突整批拒绝。每设备每分钟最多120批，超过未来5分钟/2000年前时间拒绝，单条已知用量超过10亿隔离、不进公开榜单。网站可撤回公开、撤销设备或删除身份，设备密钥不能修改这些管理设置。校验不能证明本地日志未被篡改，榜单属于客户端自报数据。
