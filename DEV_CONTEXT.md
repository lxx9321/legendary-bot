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


## 最新进度：Car 独立设备档案复用已完成

已完成：
1. Car 登录成功后保存 `device_profile:car:{wxid}`。
2. `LoginGetQRCar` 在前端不传 `DeviceID` 时，会通过 `CallerID` 查 `last_device_profile:{CallerID}:car -> wxid`。
3. 找到 wxid 后，优先读取 `device_profile:car:{wxid}`。
4. 如果 Car 独立档案不存在或不可用，再回退通用 `wxid -> LoginData`。
5. 已验证：
   - `device_profile:car:{wxid}` 中的 `Deviceid_str`
   - 新取码返回的 `DeviceId`
   二者一致。
6. 手机端验证为常用设备登录，没有新增设备。

当前结论：
Car 登录链路已经实现基于 API Key / CallerID 的后端自动设备复用，并且已具备 Car 独立档案，不再主要依赖通用 wxid 缓存。
---

# 2026-05-09 最新交接补充：Car 登录稳定性、在线链路门禁、重复轮询兜底

## 当前总状态

今天的主线已经从“Car 设备档案复用”继续推进到“登录后内部在线链路安全兜底”。

当前最重要结论：

1. Car 登录设备档案复用链路已经跑通，并且具备独立 Car 档案。
2. `Sync` 旧快照覆盖通用登录缓存的问题已经修复。
3. `SendNewMsg / Statusnotify` 已加 Car 设备一致性门禁。
4. 在线链路（心跳、长连接、Sync、服务启动恢复）已加 Car 设备一致性门禁。
5. 发现新的关键问题：前端 `LoginCheckQR` 成功后没有停止轮询，会导致同一个 `uuid` 重复进入登录成功处理。
6. 后端已补 `uuid consumed` 兜底，防止同一个二维码成功分支被重复处理。
7. 由于当前没有更多测试账号，最新 `uuid consumed` 兜底尚未完成真实登录验证。

当前不要继续用高价值账号测试。下次优先做低风险验证和前端轮询修复。

## 已完成代码修改：Sync 写回修复

问题：

`models/Msg/sync.go` 原逻辑是：

1. 读取通用 `wxid -> LoginData` 得到旧 `D`
2. 发起 `Sync` 网络请求
3. 请求成功后只更新 `D.SyncKey`
4. 把旧 `D` 整份写回 Redis

风险：

如果 `Sync` 网络请求期间，其他链路更新了 `Sessionkey / Cookie / MmtlsKey / DeviceType / Deviceid_str / DeviceInfo` 等字段，`Sync` 结束后会用旧快照整份覆盖新缓存。

已修复：

`models/Msg/sync.go` 改为：

1. `Sync` 请求前仍使用原来的 `D` 发请求
2. 请求成功后，不再用旧 `D` 写回
3. 写回前重新读取最新 `LoginData`
4. 只把新的 `SyncKey` 合并到最新 `LoginData`
5. 再写回最新对象
6. 如果重新读取失败或为空，跳过本次 `SyncKey` 持久化，不用旧 `D` 覆盖

已验证：

1. 手动调用 `/api/Msg/Sync` 成功
2. 返回 `Code=0 / Success=true / 当前未有新消息`
3. `Deviceid_str / DeviceType / ClientVersion / DeviceInfo.deviceid` 没有被改坏
4. `device_profile:car:{wxid}` 与通用 `wxid -> LoginData` 保持一致

## 已完成代码修改：SendNewMsg / Statusnotify 发送前门禁

新增：

`comm/car_send_guard.go`

核心函数：

```go
ValidateCarSendProfile(wxid string, D *LoginData) error
```

规则：

1. 先查 `device_profile:car:{wxid}` 是否存在
2. 不存在则放行，兼容非 Car 账号和旧账号
3. 存在则读取 Car 独立档案
4. Car 独立档案无效则拒绝发送
5. 与当前通用 `wxid -> LoginData` 对比：
   - `DeviceType`
   - `ClientVersion`
   - `Deviceid_str`
   - `Imei`
   - `DeviceInfo.deviceid`
6. 不一致则返回 mismatch 字段名
7. 日志只允许输出 `wxid` 和 mismatch 字段名，不输出 API Key、SessionKey、AutoauthKey、MmtlsKey、Cookie 等敏感字段

接入点：

1. `models/Msg/SendNewMsg.go`
   - 读取通用缓存后、`Statusnotify` 前先校验
   - `Statusnotify` 后、真正 `newsendmsg` 前重新读取最新 D 再校验一次
2. `models/Report/statusnotify.go`
   - 自己也独立校验，避免被其他入口直接调用时绕过门禁

已验证：

正常一致时：

1. 手工 `SendTxt` 成功
2. 英文发送成功
3. 中文用 UTF-8 body 发送成功
4. 日志未出现 `[send_guard] / [statusnotify_guard]`
5. 发送后通用缓存设备字段未变化

反向验证：

1. 曾经在线状态下临时改坏通用 `wxid -> LoginData` 的 `Deviceid_str`
2. `SendTxt` 返回：
   - `Code=-8`
   - `Success=false`
   - `Message=发送前设备档案校验失败`
3. 说明发送门禁有效

重要警告：

这次反向验证暴露出风险：虽然 `SendTxt` 被门禁拦住，但在线心跳 / 长连接当时仍可能读取被改坏的通用缓存并继续发 238 心跳，随后账号出现强制退出/模拟器提示。

以后禁止在服务运行中直接篡改通用登录缓存做反向验证。

## 已完成代码修改：在线链路 Car 设备一致性门禁

新增：

`comm/car_online_guard.go`

核心函数：

```go
ValidateCarOnlineProfile(wxid string, D *LoginData) error
```

规则与发送门禁类似：

1. `device_profile:car:{wxid}` 不存在则放行
2. 存在则校验 Car 独立档案有效性
3. 对比当前通用缓存中的：
   - `DeviceType`
   - `ClientVersion`
   - `Deviceid_str`
   - `Imei`
   - `DeviceInfo.deviceid`
4. 不一致则返回 mismatch 字段名
5. 不输出敏感字段

已接入文件和位置：

1. `controllers/Login.go`
   - `ensureAutoHeartBeat(wxid)` 中，`GetLoginata` 成功后、`Start/SendHeartBeat` 前校验
2. `srv/wxcore/wxconnect.go`
   - `SendHeartBeat()` 每次周期性心跳前重新读取最新通用缓存并校验
   - 失败时清理 `AutoHeartBeatList:{wxid}` 并 `wxconn.Stop()`
3. `srv/wxcore/wxmodels.go`
   - `loginHeartBeatLongOnce()` 中，`GetClient / MmtlsSend 238` 前校验
   - `MsgListen` 触发 `Msg.Sync` 前校验
4. `models/Msg/sync.go`
   - `Sync()` 开头读取 D 后、发 Sync 请求前校验
   - 失败时不发 Sync、不写回 SyncKey
5. `srv/wxcore/wxconnectmgr.go`
   - `InitAutoHeartBeat()` 服务启动恢复在线链前校验
6. `models/Login/HeartBeat.go`
   - `InitAutoSyncPolling()` 服务启动恢复短链轮询前校验

目的：

如果通用 `wxid -> LoginData` 与 `device_profile:car:{wxid}` 不一致，则：

1. 不启动在线链
2. 不发心跳
3. 不建长连接
4. 不发 238
5. 不 Sync
6. 不重连
7. 清理 `AutoHeartBeatList:{wxid}`
8. 交给人工处理

已验证的部分：

1. 清理 `AutoHeartBeatList:*` 后启动服务，不再自动恢复旧账号在线链
2. 服务启动只看到 API 正常启动，没有自动 238 心跳刷屏
3. `online_guard` 曾经能拦住无效 wxid，说明门禁接入生效

尚未完整验证：

1. 正常一致时重新登录后，在线链路是否只启动一次并稳定运行
2. 不一致时服务停止状态下制造异常，再启动服务是否能拒绝在线链

注意：第二项以后要用测试号，并且只能在服务停止状态下准备异常缓存，不能在线改 Redis。

## 已完成代码修改：同一个 uuid 登录成功只处理一次

发现的新问题：

前端 `LoginCheckQR` 在扫码成功后没有及时停止轮询。

导致风险：

1. 同一个 `uuid` 被重复调用 `LoginCheckQR`
2. 反复进入 `CheckUuid status == 2`
3. 反复进入 `CheckSecManualAuth`
4. 反复打印“登入数据”
5. 反复写登录缓存
6. 反复触发 `ensureAutoHeartBeat`
7. 反复创建/启动长连接和 238 心跳

日志现象：

1. 多次出现 `LoginCheckQR`
2. 多次打印完整“登入数据”
3. 多个 `TcpClient` 握手
4. 多次 238 心跳
5. `Epoll total number of connections: 0` 反复出现

后端兜底已完成：

修改文件：

1. `comm/Redis.go`
2. `models/Login/CheckUuid.go`

新增：

```go
TryConsumeLoginUUID(uuid string, ttlSeconds int64) (bool, error)
```

逻辑：

1. Redis key：`login_uuid_consumed:{uuid}`
2. 使用 `SETNX + TTL`
3. TTL 当前为 1800 秒
4. 第一次消费成功返回 `true`
5. 重复请求返回 `false`
6. Redis 异常时返回 error

`CheckUuid.go` 中：

1. 只在 `notifydataRsp.GetStatus() == 2` 成功确认登录分支调用
2. `consumed == true` 才允许进入 `CheckSecManualAuth`
3. `consumed == false` 直接返回：
   - `Code=-8`
   - `Success=false`
   - `Message=登录已完成，请停止轮询`
4. Redis 异常时保守返回失败，不继续成功登录处理

部署状态：

这刀已经部署成功。

尚未验证：

因为没有更多测试账号，尚未完成真实登录验证。

下次重点验证：

1. 第一次 `LoginCheckQR` 成功后只打印一次“登入数据”
2. 重复轮询同一个 `uuid` 时，返回“登录已完成，请停止轮询”
3. 不再重复 `CheckSecManualAuth`
4. 不再重复 `ensureAutoHeartBeat`
5. 不再出现多个 TcpClient 快速握手
6. Redis 中能看到 `login_uuid_consumed:{uuid}`，且 TTL 正常

## 当前已知风险和禁止操作

当前禁止：

1. 不要恢复 `cmdchat_enabled = true`
2. 不要用已被提示风险的账号继续高频测试
3. 不要在线篡改通用 `wxid -> LoginData`
4. 不要做反向破坏性验证
5. 不要继续 Win 登录测试
6. 不要恢复自动二次登录
7. 不要恢复心跳失败自动恢复登录
8. 不要手动频繁调用 `Secautoauth / AwakenLogin`
9. 不要多浏览器、多页面同时轮询同一个二维码
10. 不要在没有修复前端轮询的情况下继续真实账号扫码测试

当前必须保留：

1. `cmdchat_enabled = false`
2. `AutoHeartBeatList:*` 在测试前应为空
3. Redis DB2 中 API Key 白名单仍使用 `wxapi:api:keys`
4. Car 独立档案：`device_profile:car:{wxid}`
5. Caller 映射：`last_device_profile:{CallerID}:car -> wxid`
6. 发送门禁和在线门禁
7. uuid consumed 兜底

## 当前建议下次第一件事

下次不要直接登录。

第一步先确认环境：

```bash
pm2 status
cd /www/wwwroot/wxapi
grep -n "cmdchat_enabled" conf/app.conf
redis-cli -n 2 --scan --pattern "AutoHeartBeatList:*"
```

要求：

1. `cmdchat_enabled = false`
2. `AutoHeartBeatList:*` 无输出，或者先备份后清理
3. 服务日志没有自动恢复旧账号心跳

然后确认 Redis 缓存一致：

```bash
WXID="实际测试 wxid"
redis-cli -n 2 GET "$WXID" | grep -o '"Deviceid_str":"[^"]*"'
redis-cli -n 2 GET "device_profile:car:$WXID" | grep -o '"Deviceid_str":"[^"]*"'
redis-cli -n 2 GET "$WXID" | grep -o '"DeviceType":"[^"]*"'
redis-cli -n 2 GET "device_profile:car:$WXID" | grep -o '"DeviceType":"[^"]*"'
redis-cli -n 2 GET "$WXID" | grep -o '"ClientVersion":[0-9]*'
redis-cli -n 2 GET "device_profile:car:$WXID" | grep -o '"ClientVersion":[0-9]*'
redis-cli -n 2 GET "$WXID" | grep -o '"deviceid":"[^"]*"'
redis-cli -n 2 GET "device_profile:car:$WXID" | grep -o '"deviceid":"[^"]*"'
```

确认一致后，下一步优先修前端。

## 下一步优先任务：修前端轮询停止逻辑

必须修复前端：

1. 新取码前，先停止旧轮询 timer
2. 登录成功后，立即停止 `LoginCheckQR` 轮询
3. 收到后端 `登录已完成，请停止轮询` 时，也停止轮询
4. 二维码过期、取消、失败时停止轮询
5. 页面刷新/关闭前停止轮询
6. 禁止同一个页面同时存在多个轮询 timer

前端修复后再验证后端 consumed 兜底。

## 下次给 Codex 的建议提示词

```text
请先不要修改业务主链路。

先读取 DEV_CONTEXT.md，重点看最后的 2026-05-09 最新交接补充。

当前状态：
1. Car 设备档案复用已完成。
2. Car 独立档案 `device_profile:car:{wxid}` 已完成。
3. Sync 旧快照覆盖修复已完成。
4. SendNewMsg / Statusnotify 发送门禁已完成并验证。
5. 在线链路门禁已完成并部分验证。
6. 后端 `login_uuid_consumed:{uuid}` 一次性消费兜底已部署，但尚未真实验证。
7. 当前最大未完成问题是前端 `LoginCheckQR` 成功后没有停止轮询。

请先只做只读分析，不要改代码。

请重点查看前端扫码页相关代码和接口调用逻辑，找出：
1. LoginGetQRCar 取码后在哪里启动轮询
2. LoginCheckQR 轮询 timer 保存在哪里
3. 登录成功后是否 clearInterval / clearTimeout
4. 重新取码前是否清理旧 timer
5. 二维码过期/取消/失败时是否停止轮询
6. 页面卸载时是否清理轮询
7. 如何避免多个轮询同时存在

然后给出最小修改计划：
1. 修改文件
2. 每个文件改什么
3. 如何判断登录成功终态
4. 如何判断“登录已完成，请停止轮询”终态
5. 如何验证同一个 uuid 不再重复请求 LoginCheckQR
6. 推荐 commit 信息

限制：
1. 不要修改后端登录复用链路
2. 不要修改在线门禁
3. 不要修改发送门禁
4. 不要恢复 cmdchat
5. 不要继续 Win
6. 不要提供规避风控或绕过验证方案
目标只是修复项目内部重复轮询导致的重复登录成功处理。
```

