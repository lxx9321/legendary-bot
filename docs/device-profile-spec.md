# 设备档案规范设计

本文档只描述项目内部的设备档案规范、缓存边界和状态管理设计，不包含任何规避风控或绕过平台检测方案。

## 1. 当前问题总结

当前登录体系里，设备信息分散在多个字段和多条登录链路中维护，存在以下问题：

- 同一个账号在不同登录类型下可能复用同一个 Redis 登录缓存，导致 iPad、Car、Android 等字段混在一起。
- Car 登录链路可能保留旧缓存中的 iPad 设备名、Android 机型或 Apple 品牌等字段。
- 顶层 `Deviceid_str`、`Imei` 与 `DeviceInfo.deviceid`、`DeviceInfo.imei` 来源不同，容易形成同一份档案内的身份字段不一致。
- 部分内部请求在字段为空时回退到硬编码 `iPad` 或 `iPhone`，没有严格沿用登录成功时保存的设备档案。
- 二维码临时缓存、登录成功持久缓存、`devId:{deviceId}` 映射之间缺少登录类型维度，容易把不同登录类型的档案串起来。
- 设备失效或登录态不可用时，目前缺少明确的设备状态字段，后续流程难以判断应该继续使用、重新登录，还是停止自动刷新。

## 2. 设备档案的目标

设备档案的目标是让每一种登录类型拥有一份稳定、可复用、可验证的设备上下文。

- 同一个账号在同一种登录类型下，复用同一份设备档案。
- iPad 登录链路只能使用 iPad 设备档案。
- Car 登录链路只能使用 Car 设备档案。
- 登录成功后的内部请求，只能使用登录成功时保存的设备字段和登录态字段。
- 不允许内部请求临时补硬编码设备类型或设备名称。
- 不允许临时生成新的 `DeviceID` 覆盖已有设备档案。
- 当服务端返回设备失效、登录态失效、需要重新验证等结果时，设备档案应标记为 `stale` 或 `invalid`，交给前端重新走正常登录流程。

## 3. 设备字段分类

### 3.1 设备身份字段

这些字段决定“这是一台什么设备”，应该在设备档案创建时统一生成，并长期保持稳定。

- `ProfileType`
- `Deviceid_str`
- `Deviceid_byte`
- `Imei`
- `DeviceType`
- `DeviceName`
- `ClientVersion`
- `SoftType`
- `OsVersion`
- `RomModel`
- `DeviceInfo`

### 3.2 登录态字段

这些字段来自登录成功或登录态刷新结果，可以在同一设备档案下更新，但不能跨登录类型迁移。

- `Wxid`
- `Uin`
- `Sessionkey`
- `Sessionkey_2`
- `Autoauthkey`
- `Autoauthkeylen`
- `Clientsessionkey`
- `Serversessionkey`
- `Loginecdhkey`
- `Cooike`
- `DeviceToken`
- `SyncKey`
- `RsaPublicKey`
- `RsaPrivateKey`
- `MmtlsKey`
- `LoginDate`
- `RefreshTokenDate`

### 3.3 网络环境字段

这些字段描述当前网络连接和代理环境，可以随运行环境变化，但更新时不能破坏设备身份字段。

- `Proxy`
- `Mmtlsip`
- `ShortHost`
- `LongHost`
- `Dns`

### 3.4 临时二维码字段

这些字段只属于一次二维码登录流程，应该有过期时间，不应作为长期设备身份。

- `Uuid`
- `Aeskey`
- `NotifyKey`
- 二维码返回内容
- 二维码过期时间

## 4. iPad 登录档案应包含哪些字段

iPad 档案必须明确标记为 iPad 类型，例如：

- `ProfileType = "ipad"`
- `Deviceid_str`
- `Deviceid_byte`
- `Imei`
- `DeviceType = Algorithm.IPadDeviceType`
- `DeviceName = Algorithm.IPadDeviceName` 或用户首次创建档案时传入的 iPad 名称
- `ClientVersion = Algorithm.IPadVersion`
- `SoftType` 使用 iPad 档案对应来源生成
- `OsVersion = Algorithm.IPadOsVersion`
- `RomModel = Algorithm.IPadModel`
- `DeviceInfo` 中的 `deviceid`、`imei`、`devicename`、`ostype`、`ostypenumber`、`iphonever` 必须与顶层字段同源
- 登录成功后保存同一份档案下的 `Sessionkey`、`Autoauthkey`、`Clientsessionkey`、`MmtlsKey`、`DeviceToken`

## 5. Car 登录档案应包含哪些字段

Car 档案必须明确标记为 Car 类型，例如：

- `ProfileType = "car"`
- `Deviceid_str`
- `Deviceid_byte`
- `Imei`
- `DeviceType = Algorithm.CarDeviceType`
- `DeviceName = Algorithm.CarDeviceName` 或用户首次创建档案时传入的 Car 名称
- `ClientVersion = Algorithm.CarVersion`
- `SoftType` 使用 Car 档案对应来源生成
- `OsVersion = Algorithm.CarOsVersion`
- `RomModel = Algorithm.CarModel`
- `DeviceInfo` 中的 `deviceid`、`imei`、`devicename`、`ostype`、`ostypenumber`、`iphonever` 必须与顶层字段同源
- 登录成功后保存同一份档案下的 `Sessionkey`、`Autoauthkey`、`Clientsessionkey`、`MmtlsKey`、`DeviceToken`

Car 档案中不应该出现 iPad 设备名、iPad 版本、Android 机型等与 Car 档案类型冲突的字段。

## 6. 哪些字段必须同源

以下字段必须从同一份设备档案生成或派生：

- `Deviceid_str` 与 `Deviceid_byte`
- `Deviceid_str` 与顶层 `Imei`
- 顶层 `Deviceid_str` 与 `DeviceInfo.deviceid`
- 顶层 `Imei` 与 `DeviceInfo.imei`
- 顶层 `DeviceName` 与 `DeviceInfo.devicename`
- 顶层 `DeviceType` 与 `DeviceInfo.ostype`
- 顶层 `OsVersion` 与 `DeviceInfo.ostypenumber`
- 顶层 `RomModel` 与 `DeviceInfo.iphonever`
- `ClientVersion` 与 `DeviceType`
- `SoftType` 与 `Deviceid_str`、`OsVersion`、`RomModel`
- `DeviceToken` 与当前设备档案

如果任意一个同源字段无法对齐，应拒绝继续内部请求，并把设备档案标记为 `stale`，等待前端重新登录。

## 7. 哪些字段允许登录成功后刷新

以下字段可以在同一 `Wxid + ProfileType` 档案下刷新：

- `Sessionkey`
- `Sessionkey_2`
- `Autoauthkey`
- `Autoauthkeylen`
- `Clientsessionkey`
- `Serversessionkey`
- `Loginecdhkey`
- `Cooike`
- `DeviceToken`
- `SyncKey`
- `RsaPublicKey`
- `RsaPrivateKey`
- `MmtlsKey`
- `ShortHost`
- `LongHost`
- `Mmtlsip`
- `LoginDate`
- `RefreshTokenDate`

刷新这些字段时，不应重建或覆盖设备身份字段。

## 8. 哪些字段禁止跨登录类型复用

以下字段禁止在 iPad、Car、Win、Mac、Android 等登录类型之间复用：

- `Deviceid_str`
- `Deviceid_byte`
- `Imei`
- `DeviceType`
- `DeviceName`
- `ClientVersion`
- `SoftType`
- `OsVersion`
- `RomModel`
- `DeviceInfo`
- `DeviceToken`
- `Autoauthkey`
- `Sessionkey`
- `Clientsessionkey`
- `Serversessionkey`
- `MmtlsKey`

如果历史缓存里已经存在跨类型混用，读取时应优先判定为 `stale`，不要在内部请求里自动纠正为另一个类型。

## 9. Redis key 建议

建议把设备档案、登录态、二维码临时态拆开管理，并显式加入登录类型。

- 设备档案：`device_profile:{wxid}:{profileType}`
- 当前激活档案：`active_profile:{wxid}:{profileType}`
- 设备 ID 映射：`devId:{profileType}:{deviceId}`
- 二维码临时缓存：`qr_login:{profileType}:{uuid}`
- 旧版兼容读取：保留读取旧 key 的能力，但读取后必须校验 `ProfileType` 与字段一致性。

示例：

```text
device_profile:wxid_xxx:ipad
device_profile:wxid_xxx:car
active_profile:wxid_xxx:ipad
active_profile:wxid_xxx:car
devId:ipad:49xxxxxxxx
devId:car:49xxxxxxxx
qr_login:car:uuid_xxx
```

不建议继续只用 `devId:{deviceId}` 映射到 `wxid`，因为它无法区分同一账号下的不同登录类型。

## 10. 设备状态设计

### pending

表示二维码已创建，但尚未完成登录。

- 可保存 `Uuid`、`NotifyKey`、`Aeskey`、临时 `MmtlsKey`
- 必须带 `ProfileType`
- 必须设置过期时间
- 不允许作为内部业务请求的登录态使用

### active

表示登录成功，设备档案和登录态可用于内部请求。

- 已保存 `Wxid`
- 已保存登录态字段
- 设备身份字段通过一致性校验
- 内部请求只能使用 `active` 档案

### stale

表示设备档案或登录态可能过期，需要前端重新走正常登录流程。

触发场景：

- 服务端返回登录态失效
- 服务端返回需要重新验证
- 内部校验发现设备字段不同源
- 内部请求发现必要登录态字段缺失
- 旧缓存中存在 iPad / Car / Android 混合字段

`stale` 状态下不应自动硬重试，不应自动生成新设备 ID。

### invalid

表示设备档案不可继续使用。

触发场景：

- 设备身份字段严重缺失
- `ProfileType` 与 `DeviceType`、`ClientVersion` 明显冲突
- `Deviceid_str` 无法解析为 `Deviceid_byte`
- 历史缓存无法迁移且无法确认归属类型

`invalid` 状态下应要求重新创建同类型设备档案。

## 11. 后续代码改造顺序

建议按小步改造，避免一次性改动太多链路。

1. 新增设备档案结构和状态字段，只定义模型，不接入业务。
2. 新增设备档案一致性校验函数，只做只读校验和日志输出。
3. 为 iPad QR 登录接入 `ProfileType = "ipad"`，保持旧 Redis 读取兼容。
4. 为 Car QR 登录接入 `ProfileType = "car"`，禁止复用 iPad 档案。
5. 调整 `devId` 映射，增加登录类型维度。
6. 调整二维码临时缓存 key，增加登录类型维度。
7. 登录成功后只刷新登录态字段，不覆盖设备身份字段。
8. 内部请求统一从 active 设备档案读取字段，移除空值时的 `iPad` / `iPhone` 回退。
9. 设备失效、登录态失效、字段不一致时标记 `stale`，交给前端重新登录。
10. 旧缓存迁移：读取旧缓存后做类型和同源校验，能确认类型则迁移，不能确认则标记 `stale` 或 `invalid`。

## 12. 每一步改造的验证方式

### 第 1 步：新增结构

验证方式：

- 编译通过。
- 不改变现有登录行为。
- 新结构字段能表达 `profileType`、`status`、设备身份字段、登录态字段。

### 第 2 步：新增校验函数

验证方式：

- 对现有缓存只输出校验结果，不阻断业务。
- 能识别顶层 `Deviceid_str/Imei` 与 `DeviceInfo.deviceid/imei` 不一致。
- 能识别 Car 档案中出现 iPad 名称或 Android 机型。

### 第 3 步：iPad QR 接入档案

验证方式：

- iPad 取码成功。
- Redis 中 iPad 档案 key 带 `ipad` 类型。
- `DeviceType`、`ClientVersion`、`DeviceName`、`RomModel`、`OsVersion` 都是 iPad 来源。

### 第 4 步：Car QR 接入档案

验证方式：

- Car 取码成功。
- Redis 中 Car 档案 key 带 `car` 类型。
- Car 档案中不出现 iPad 设备名和 Android 机型。
- 顶层字段与 `DeviceInfo` 字段同源。

### 第 5 步：调整 devId 映射

验证方式：

- `devId:ipad:{deviceId}` 只能找到 iPad 档案。
- `devId:car:{deviceId}` 只能找到 Car 档案。
- 同一个账号的 iPad 和 Car 不互相覆盖。

### 第 6 步：调整二维码临时缓存

验证方式：

- 二维码缓存有过期时间。
- QR 临时缓存带 `profileType`。
- iPad UUID 不会读取到 Car 临时缓存。

### 第 7 步：登录成功刷新登录态

验证方式：

- 登录成功后 `Sessionkey`、`Autoauthkey`、`Clientsessionkey` 正常更新。
- `Deviceid_str`、`Imei`、`DeviceType`、`DeviceInfo` 不被跨类型覆盖。

### 第 8 步：内部请求读取 active 档案

验证方式：

- `sync`、新设备确认等内部请求读取 active 档案字段。
- 字段缺失时返回明确错误或标记 `stale`。
- 不再出现空值回退 `"iPad"` 或 `"iPhone"` 的行为。

### 第 9 步：设备失效状态处理

验证方式：

- 服务端返回登录态失效时，档案状态变为 `stale`。
- 不自动生成新设备 ID。
- 不自动硬重试二次登录。
- 前端能看到需要重新登录的状态。

### 第 10 步：旧缓存迁移

验证方式：

- 纯 iPad 旧缓存能迁移为 iPad 档案。
- 纯 Car 旧缓存能迁移为 Car 档案。
- 混合缓存不自动猜测修复，标记为 `stale` 或 `invalid`。
- 迁移过程不删除原始旧缓存，方便回滚和排查。
