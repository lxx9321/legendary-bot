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

补充发现：
浏览器 Network 已确认，LoginGetQRCar 请求中即使前端 DeviceName 输入框清空，Payload 仍会带 DeviceName="iPad"。这会污染 Car 登录缓存。下一步需要修复 Car 接口不信任前端 DeviceName，并排查前端默认值来源。

验证结果：
修复 `CheckSecManualAuth.go` 后，Car 登录缓存中的 `DeviceInfo` 已不再回灌旧 iPad/Apple 数据。
当前 `/login/GetCacheInfo` 显示：顶层 `Deviceid_str/Imei` 与 `DeviceInfo.deviceid/imei` 已同源，`DeviceInfo.devicename`、`devicebrand`、`ostype` 也与当前 Car 登录数据一致。

剩余问题：
尚未确认同账号同 Car 登录类型是否能稳定复用同一份 `Deviceid_str/Imei`。下一步需要重复执行 LoginGetQRCar 登录，对比两次缓存中的设备 ID 是否保持一致。

最新验证：
Car 登录缓存内部字段已自洽，`DeviceInfo.deviceid/imei` 已与顶层 `Deviceid_str/Imei` 同源，`DeviceInfo.devicename/devicebrand/ostype` 也已与 Car 登录数据一致。

剩余问题：
同账号再次使用 `LoginGetQRCar` 登录时，`Deviceid_str` 仍会变化，手机端显示为新设备。下一步需要分析 `LoginGetQRCar` 取码阶段是否每次重新生成 DeviceID，以及是否可以通过 `wxid + loginType=car` 复用旧设备档案。

## API Key / callerId 传递验证

已完成第一刀：API Key 鉴权通过后，后端会生成 `CallerID = sha256(apiKey)`，并写入 Car 登录链路。

验证结果：
- `uuid` 临时缓存中已出现 `CallerID`
- 登录成功后的 `wxid` 持久缓存中也保留了同一个 `CallerID`
- 说明 `API Key -> callerId -> uuid临时缓存 -> CheckUuid -> CheckSecManualAuth -> wxid持久缓存` 链路已打通

注意：
当前只是打通 callerId 传递，还没有实现设备自动复用。
下一步目标是登录成功后保存：
`last_device_profile:{callerId}:car -> wxid`
然后下次 `LoginGetQRCar` 没传 DeviceID 时，通过 callerId 自动找到旧 wxid 并复用 Car 设备档案。


## 最新进度：API Key / CallerID 链路已打通

已完成：
1. API Key 鉴权已启用。
2. Redis API Key 白名单实际在 DB2。
3. `apikeyenforce = true`
4. `apikeysrediskey = wxapi:api:keys`
5. `CallerID = sha256(apiKey)` 已能写入 Car 登录链路。
6. 已验证：
   - uuid 临时缓存中存在 CallerID
   - wxid 持久缓存中也存在同一个 CallerID

当前尚未完成：
1. 还没有实现设备自动复用。
2. 下一步目标是在登录成功后保存：
   `last_device_profile:{CallerID}:car -> wxid`
3. 再下一步才是在 `LoginGetQRCar` 取码前读取这个映射，自动复用旧 Car 设备档案。

注意：
当前只是打通 CallerID 传递链路，不要重新分析 DeviceInfo 回灌问题。

## 最新进度：Car 设备档案后端自动复用已跑通

已完成：
1. `API Key -> CallerID` 链路已打通。
2. 登录成功后已保存：
   `last_device_profile:{CallerID}:car -> wxid`
3. `LoginGetQRCar` 在前端不填写 `DeviceID` 时，已能通过 `CallerID` 找回上次 Car 登录的 `wxid`。
4. 后端已能读取该 `wxid` 的持久缓存，并复用旧 `Deviceid_str`。
5. 已验证：
   - 旧缓存 `Deviceid_str`
   - 新取码返回 `DeviceId`
   二者一致。

当前结果：
Car 取码阶段已实现基于 API Key / CallerID 的后端自动设备复用，不再依赖前端手动填写 DeviceID。

下一步：
完成扫码登录后继续确认：
- 登录成功后的 `Deviceid_str` 是否仍保持一致
- `DeviceInfo.deviceid` 是否仍与顶层 `Deviceid_str` 同源
- 手机端是否不再新增设备


## 最新进度：Car 设备档案后端自动复用已验证成功

已完成：
1. `API Key -> CallerID` 链路已打通。
2. 登录成功后已保存：
   `last_device_profile:{CallerID}:car -> wxid`
3. `LoginGetQRCar` 在前端不填写 `DeviceID` 时，已能通过 `CallerID` 找回上次 Car 登录的 `wxid`。
4. 后端已能读取该 `wxid` 的持久缓存，并复用旧 Car 设备档案。
5. 已验证：
   - 新取码返回的 `DeviceId`
   - 登录成功后的 `Deviceid_str`
   - `DeviceInfo.deviceid`
   三者完全一致。
6. 手机端已验证为常用设备登录，没有新增设备。

当前结论：
Car 登录链路已经实现基于 API Key / CallerID 的后端自动设备复用，不再依赖前端手动填写 DeviceID。

下一阶段再考虑：
1. 账号转移 / 重新绑定功能。
2. last_device_profile 映射失效时的清理逻辑。
3. iPad / Mac / Windows 是否按同样模式扩展。