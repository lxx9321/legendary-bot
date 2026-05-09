# 半成品项目交接摘要

## 当前目标

当前目标已升级为：规范化所有登录接口的设备档案管理。

现在重点不是继续单点修复某个接口，而是解决整个登录体系里的设备指纹/设备档案不一致问题。

要求：

1. 同一个账号在同一种登录类型下，必须复用同一份设备档案。
2. iPad 登录链路只能使用 iPad 设备档案。
3. Car 登录链路只能使用 Car 设备档案。
4. 所有内部请求必须沿用登录成功时保存的设备信息。
5. 不允许内部请求重新硬编码 iPhone / iPad。
6. 不允许临时生成新的 DeviceID 破坏已有设备档案。
7. 如果服务端提示设备失效或需要重新验证，不自动硬重试，应标记设备档案为 stale，交给前端重新走正常登录流程。
8. 本项目只做内部一致性和稳定性修复，不做规避平台风控或绕过验证相关方案。

## 当前问题

已通过 `/login/GetCacheInfo` 确认：Car 登录缓存中存在明显设备信息混搭。

当前观察到的问题：

1. `DeviceType = car-31`
2. `DeviceName = iPad`
3. `SoftType` 带 iPad 风格
4. `RomModel = Xiaomi-M2012K11AC`
5. `DeviceInfo.devicebrand = Apple`
6. `DeviceInfo.devicename = iPad`
7. `DeviceInfo.ostype = car-31`
8. 顶层 `Deviceid_str / Imei` 与 `DeviceInfo.deviceid / DeviceInfo.imei` 不一致

当前判断：

这说明 Car 登录链路中，设备档案不是单一来源，而是混入了 Car / iPad / Android / Apple 多套字段。

## 当前怀疑的核心根因

1. `DeviceInfo` 内部的 `deviceid / imei` 是随机生成的，不等于顶层 `Deviceid_str / Imei`
2. `UpdateCarLoginData()` 可能只在字段为空时补 Car 默认值，旧缓存里已有的 iPad / Android 字段不会被强制修正
3. 普通 `GetQRCODE(LoginType=2)` 可能用 Car 的 `DeviceType` 发请求，但缓存主体仍是 iPad 初始化链路
4. `DeviceType / DeviceName / ClientVersion / SoftType / RomModel / DeviceInfo` 没有统一设备档案来源
5. Redis 目前更像是保存登录态，不像是严格区分 `wxid + loginType` 的设备档案管理

## 已确认无需重复分析的区域

以下问题已经处理完成，暂时不要重新展开：

- 自动二次登录
- 心跳失败后自动恢复登录
- RefreshTokenTimer 自动消费
- `sync.go` 硬编码 iPhone
- `Secautoauth` 设备信息强制重写
- 自动长连恢复逻辑

## 已完成修改

提交 `19c0b6f`：`Disable automatic relogin during heartbeat`

已完成：

- 关闭自动二次登录
- 关闭心跳失败后的自动恢复登录
- 保留前端手动二次登录 / 唤醒接口
- 同步消息请求改为优先使用当前登录数据里的设备类型

提交 `b5893c0`：`Start heartbeat after QR login success`

已完成：

- 二维码登录成功后自动提取 `wxid`
- 登录成功后启动心跳
- 失败时写调试信息

提交 `3845cfa`：`Use cached device name in ext device login confirm get`

已完成：

- `ExtDeviceLoginConfirmGet.go` 不再直接硬编码 `DeviceName: "iPhone"`
- 改为优先使用 Redis 登录态中的 `D.DeviceName`
- 空值时再兜底
- 已推送并在服务器重新部署

## 当前代码状态

当前状态：

- 自动心跳保留
- 自动二次登录已关闭
- 心跳失败后不再自动恢复登录
- 前端手动二次登录、唤醒登录接口保留
- `ExtDeviceLoginConfirmGet` 的 `DeviceName` 已改为优先读取缓存
- 当前主要问题已升级为：登录接口设备档案不一致
- Car 登录链路中仍存在 Car / iPad / Android / Apple 字段混用，需要继续规范

## Git 和部署状态

GitHub 推送已成功。  
`origin` 已改为 SSH。

服务器部署注意事项：

- 服务器部署目录 `/www/wwwroot/wxapi` 曾经存在本地未提交改动
- 已通过 `git reset --hard` 和 `git clean -fd` 清理
- 已重新 `git pull origin main`
- 已重新执行 `go build -buildvcs=false -o wxapi .`
- 已通过 `pm2 restart wxapi` 重启服务
- 部署目录自身旧 `.git` 状态不一定可靠，确认版本时优先看实际源码内容、GitHub commit、PM2 日志

## 当前未完成任务

当前未完成任务不是继续单点修 `ExtDeviceLoginConfirmGet / Ok`。

当前未完成任务是：

1. 建立设备字段审计记录
2. 建立设备档案规范文档
3. 明确哪些字段属于设备身份
4. 明确哪些字段属于登录态
5. 明确哪些字段允许刷新
6. 明确哪些字段禁止跨登录类型复用
7. 明确 Redis 设备档案 key 设计
8. 明确设备失效时的 stale / invalid 状态处理
9. 按规范分阶段改造 Car / iPad 登录链路

## 当前优先文档

建议新增：

`docs/device-field-audit-2026-05-09.md`

用途：

- 保存 Codex 对当前设备字段来源的盘点结果
- 记录当前 Car 缓存中字段混搭现状
- 作为后续规范化依据

建议新增：

`docs/device-profile-spec.md`

用途：

- 定义设备档案规范
- 定义 iPad / Car 登录档案字段
- 定义 Redis key 设计
- 定义设备状态
- 定义后续代码改造顺序

## 当前优先读取的文件

下一步优先读取：

`DEV_CONTEXT.md`  
`docs/device-field-audit-2026-05-09.md`  
`docs/device-profile-spec.md`

如果文档还没创建，则先读取：

`models/Login/GetQRCodeCar.go`  
`models/Login/GetQRCode.go`  
`models/Login/CheckSecManualAuth.go`  
`models/Login/InitData.go`  
`models/Login/Util.go`  
`models/Login/Secautoauth.go`  
`models/Login/ExtDeviceLoginConfirmGet.go`  
`models/Login/ExtDeviceLoginConfirmOk.go`  
`models/Msg/sync.go`  
`srv/wxcore/wxconnect.go`  
`comm/Redis.go`

## 当前给 Codex/Cursor 的提示词

请先不要修改业务代码。

当前任务不是继续修单个接口，而是整理设备档案规范。

请根据当前字段盘点结果，生成：

`docs/device-profile-spec.md`

文档需要包含：

1. 当前问题总结
2. 设备档案目标
3. 设备字段分类
   - 设备身份字段
   - 登录态字段
   - 网络环境字段
   - 临时二维码字段
4. iPad 登录档案应包含哪些字段
5. Car 登录档案应包含哪些字段
6. 哪些字段必须同源
7. 哪些字段允许登录成功后刷新
8. 哪些字段禁止跨登录类型复用
9. Redis key 建议
10. 设备状态设计
    - pending
    - active
    - stale
    - invalid
11. 后续代码改造顺序
12. 每一步改造的验证方式

注意：

- 不要提供规避平台风控或绕过验证方案
- 目标只是修复项目内部设备信息不一致、缓存混用和状态管理混乱问题
- 先写规范文档，不要改代码

## 当前禁止操作

在没有明确确认之前，暂时不要：

1. 重新启用自动二次登录
2. 重新启用心跳失败自动恢复登录
3. 新增自动设备切换逻辑
4. 同时修改 Car 和 iPad 两条登录链路
5. 扩大扫描范围到全项目
6. 批量重构 Redis 登录缓存结构
7. 在没有规范文档前直接修改 `CheckSecManualAuth`
8. 在没有确认字段来源前直接改 `DeviceInfo`
9. 把旧 iPad / Android 字段强行塞进 Car 档案
10. 让内部请求自己临时生成新的 DeviceID