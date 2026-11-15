DH燃源榜 dh-v1.1.1 修复与恢复版本。

- Windows 包补齐 toktrail-background.exe，安装器验证并复制全部三个程序；原生 CI 实际安装、查询和移除测试任务。
- 已确认的批次不再删除，保持不可变归档和增量信任链；只上传 pending 批次。Codex / DeepSeek 的第二次扫描与新增记录已补回归测试。
- 新增 status、repair、已有身份复用 setup；历史恢复只接受服务器确认且严格核对的快照，不重传历史或清空游标。
- 同一账号的多设备身份、凭据撤销/安全重绑、同步失败状态；严格拒绝历史篡改、未来时间和高频请求，异常 Token 隔离。客户端自报无法绝对防伪。
- 网站新增修复配置说明、设备管理与 GitHub 头像同步。
- macOS / Windows / Linux 的 amd64 与 arm64 六平台，附安装脚本和 SHA256SUMS。
