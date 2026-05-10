# 半成品项目交接摘要

## 当前问题

登录成功后，不启动自动心跳时风险较低，但 API 捕捉不到手机消息；启动自动心跳后能获取消息，但存在被平台判定异常设备操作的风险。当前怀疑重点是：自动恢复登录逻辑曾经混乱、car/iPad 登录路径设备信息不一致、设备指纹复用不稳定，导致登录设备表现不固定。

当前怀疑的核心根因：
1. car 登录路径与 iPad 登录路径设备信息来源不一致
2. DeviceType / DeviceName / ClientVersion 可能存在混用
3. Redis 中 devId / DeviceID 映射可能没有稳定复用
4. 登录、同步、恢复登录链路可能使用了不同设备画像
5. 某些接口仍可能存在硬编码 iPhone/iPad

## 已确认无需重复分析的区域

以下问题已经处理完成，暂时不要重新展开：

- 自动二次登录
- 心跳失败后自动恢复登录
- RefreshTokenTimer 自动消费
- sync.go 硬编码 iPhone
- Secautoauth 设备信息强制重写
- 自动长连恢复逻辑

## 已完成修改

提交 `19c0b6f`：`Disable automatic relogin during heartbeat`  
关闭自动二次登录和心跳失败后的自动恢复登录；保留前端手动二次登录/唤醒接口；同步消息请求改为优先使用当前登录数据里的设备类型。

提交 `b5893c0`：`Start heartbeat after QR login success`  
二维码登录成功后自动提取 `wxid` 并启动心跳；失败时写调试信息。

## 当前代码状态

自动心跳保留。  
自动二次登录已关闭。  
心跳失败后不再自动恢复登录。  
前端手动二次登录、唤醒登录接口保留。  
设备信息已做部分统一，但 car 登录路径、iPad/car 混合登录路径、Redis 设备映射仍需继续核对。

## Git 和部署状态

GitHub 推送已成功。  
`origin` 已改为 SSH。  
服务器裸仓库 `/var/repo/wxapi.git` 已收到最新提交。  
部署 hook 会 checkout 到 `/www/wwwroot/wxapi`，重新 build，并用 PM2 重启。  
PM2 此前确认已正常重启。  
注意：部署目录自身有旧 `.git`，不能直接用它的 `git log` 判断部署版本，应以裸仓库提交、实际源码和运行日志为准。

## 当前未完成任务

继续检查 `/Login/ExtDeviceLoginConfirmGet` 和 `/Login/ExtDeviceLoginConfirmOk`。  
它们用于新设备扫码确认登录相关流程，可能处理 URL、wxid 和确认动作，但不能保证通过新设备验证，也不能替代稳定的设备信息管理。当前重点是确认这两个接口里的 URL 解析、设备信息来源、Redis 缓存读写是否一致稳定。

## 明天优先读取的文件

`controllers/Login.go`  
`models/Login/GetQRCodeCar.go`  
`models/Login/GetQRCode.go`  
`models/Login/CheckSecManualAuth.go`  
`models/Login/ExtDeviceLoginConfirm*.go`  
`comm/Redis.go`



## 明天给 Codex/Cursor 的提示词

请先阅读交接摘要，不要全项目扫描，不要先改代码。只查看登录、car 取码、新设备确认、Redis 登录数据缓存相关文件。先说明 `/Login/ExtDeviceLoginConfirmGet` 和 `/Login/ExtDeviceLoginConfirmOk` 的实际调用流程、设备信息来源、URL 处理方式和缓存关系。重点检查 car 登录与 iPad/car 混合登录路径的设备信息是否一致稳定。修改前先说明计划，每次只改一个小点，不提供规避风控或绕过检测方案。
---

# 2026-05-10 最新交接补充：新账号 Car 登录验证通过，前端轮询误判已修复

## 今日已完成

1. **本地与服务器配置继续保持自动回复关闭**
   - `cmdchat_enabled = false`
   - 注意：如果本地 `conf/app.conf` 是 `true`，每次部署都会覆盖服务器配置，所以后续提交/部署前必须先确认本地也是 `false`。

2. **新 API Key 已生成并加入 Redis DB2 白名单**
   - 白名单集合：`wxapi:api:keys`
   - 验证方式：
     ```bash
     redis-cli -n 2 SISMEMBER wxapi:api:keys "<new api key>"
     ```
   - 返回 `1` 表示可用。
   - 不要在日志、截图、提交记录里暴露真实 API Key。

3. **前端 LoginCheckQR 轮询误判已修复**
   - 旧问题：前端把 `Code=0 && Success=true` 误判成登录成功。
   - 实际含义：
     - `Code=0 / Success=true`：只代表 `LoginCheckQR` 接口调用成功。
     - `Data.status=0`：等待扫码。
     - `Data.status=1`：已扫码，等待手机确认。
     - `Data.status=2`：手机已确认，才算真正登录成功。
   - 已修复文件：
     ```text
     static/scanlogin/index.html
     ```
   - 修复规则：
     - `status=0` 继续轮询
     - `status=1` 继续轮询
     - `status=2` 停止轮询
     - 不再用 `Code=0 / Success=true` 单独判断登录成功
     - `status` 已统一 `Number(rawStatus)` 后再判断
   - 已验证：
     - 未扫码时返回 `status=0`
     - 前端不再误判登录成功
     - 轮询继续正常进行

4. **新账号 Car 登录成功**
   - 新账号 wxid：
     ```text
     wxid_nq0qtjquyiq212
     ```
   - 登录 UUID：
     ```text
     wcM9RSI_3hT8-gAAAAAA
     ```
   - 日志观察：
     - `LoginGetQRCar` 正常取码
     - `LoginCheckQR` 正常轮询
     - 登录成功后只看到一次完整“登入数据”
     - 未见昨天那种短时间连续多次“登入数据”
     - 心跳启动后出现：
       ```text
       总链接数量: 1
       发送心跳成功
       ```

5. **后端 uuid consumed 兜底已真实验证**
   - 查询命令：
     ```bash
     redis-cli -n 2 GET "login_uuid_consumed:wcM9RSI_3hT8-gAAAAAA"
     redis-cli -n 2 TTL "login_uuid_consumed:wcM9RSI_3hT8-gAAAAAA"
     ```
   - 实际结果：
     ```text
     GET = 1
     TTL = 142
     ```
   - 结论：
     - 同一个二维码成功分支已被标记为已消费
     - 后续重复轮询同一个 UUID 不应再次进入 `CheckSecManualAuth`
     - 后端防重复成功处理兜底已通过新账号真实验证

6. **AutoHeartBeatList 当前只绑定新账号**
   - 查询命令：
     ```bash
     redis-cli -n 2 --scan --pattern "AutoHeartBeatList:*"
     ```
   - 当前结果：
     ```text
     AutoHeartBeatList:wxid_nq0qtjquyiq212
     ```
   - 结论：
     - 当前自动心跳记录是新登录账号
     - 没有一堆旧账号残留
     - 这是正常状态

7. **新账号通用登录缓存设备字段自洽**
   - 查询结果：
     ```text
     Deviceid_str = 494b95c2e66d1d38eb434eeb01c40
     DeviceType = car-31
     ClientVersion = 553650459
     DeviceInfo.deviceid = 494b95c2e66d1d38eb434eeb01c40
     ```
   - 结论：
     - 顶层 `Deviceid_str` 与 `DeviceInfo.deviceid` 一致
     - `DeviceType` 是 Car 类型
     - `ClientVersion` 正常
     - 通用缓存当前自洽

## 当前尚未完成 / 下次优先验证

### 1. 补查 Car 独立设备档案是否与通用缓存一致

下次第一步建议执行：

```bash
WXID="wxid_nq0qtjquyiq212"

redis-cli -n 2 EXISTS "device_profile:car:$WXID"

redis-cli -n 2 GET "device_profile:car:$WXID" | grep -o '"Deviceid_str":"[^"]*"'
redis-cli -n 2 GET "device_profile:car:$WXID" | grep -o '"DeviceType":"[^"]*"'
redis-cli -n 2 GET "device_profile:car:$WXID" | grep -o '"ClientVersion":[0-9]*'
redis-cli -n 2 GET "device_profile:car:$WXID" | grep -o '"deviceid":"[^"]*"'
```

预期：

```text
EXISTS = 1
Deviceid_str = 494b95c2e66d1d38eb434eeb01c40
DeviceType = car-31
ClientVersion = 553650459
DeviceInfo.deviceid = 494b95c2e66d1d38eb434eeb01c40
```

### 2. 挂机观察心跳稳定性

建议观察 10 到 15 分钟：

```bash
pm2 logs wxapi --lines 200
```

重点看是否出现：

```text
[online_guard]
[send_guard]
重复“登入数据”
多个 TcpClient 快速握手
Epoll connections 一直为 0
心跳多次失败
```

通过标准：

```text
登入数据只出现一次
心跳按正常间隔执行
没有 online_guard mismatch
没有重复启动多个 TcpClient
```

### 3. 暂时不要恢复自动回复

继续保持：

```ini
cmdchat_enabled = false
```

不要做：

```text
开自动回复
发消息压力测试
测 Win
反向改 Redis
手动频繁 Secautoauth / AwakenLogin
```

### 4. 后续再做第二次 Car 取码复用验证

等当前账号稳定后，后续可以验证：

1. 使用同一个 API Key 再次 `LoginGetQRCar`
2. 前端 `DeviceID` 留空
3. 观察新取码返回的 `DeviceId`
4. 确认是否仍等于：
   ```text
   494b95c2e66d1d38eb434eeb01c40
   ```

这一步是验证新账号上的完整链路：

```text
last_device_profile:{CallerID}:car -> wxid
device_profile:car:{wxid}
```

## 当前阶段结论

截至 2026-05-10，本项目 Car 登录链路已经完成并验证了以下关键闭环：

```text
1. API Key 鉴权与 CallerID 传递
2. CallerID 到上次 Car wxid 映射
3. Car 独立设备档案保存
4. Car 取码阶段自动复用设备档案
5. 通用缓存设备字段自洽
6. Sync 不再用旧快照覆盖整份登录缓存
7. SendTxt / Statusnotify 发送前设备一致性门禁
8. 在线心跳 / 长连接 / Sync 设备一致性门禁
9. LoginCheckQR 前端自动轮询
10. 前端 status=0 不再误判登录成功
11. 后端 uuid consumed 防重复成功处理
12. 新账号 Car 登录成功
13. 新账号 uuid consumed 真实验证通过
14. 新账号自动心跳只绑定当前 wxid
```

当前最适合的下一步不是继续大改，而是：

```text
先补查 device_profile:car:{新 wxid}
再挂机观察心跳
再做同 API Key 第二次 Car 取码复用验证
```

## 下次给 Codex 的建议提示词

```text
请先读取 DEV_CONTEXT.md，重点看最后的 2026-05-10 最新交接补充。

当前状态：
1. Car 设备档案复用链路已完成。
2. 前端 LoginCheckQR 自动轮询已修复。
3. 前端不再把 Code=0 / Success=true 误判为登录成功。
4. 后端 login_uuid_consumed:{uuid} 已通过新账号真实验证。
5. 新账号 wxid_nq0qtjquyiq212 登录成功，通用缓存设备字段自洽。
6. 当前自动心跳列表只包含 AutoHeartBeatList:wxid_nq0qtjquyiq212。
7. 当前 cmdchat_enabled 必须保持 false。

请先不要修改代码。

下一步只做验证计划：
1. 补查 device_profile:car:wxid_nq0qtjquyiq212 是否存在，并与通用缓存字段一致。
2. 观察 10 到 15 分钟心跳日志，确认没有 online_guard mismatch、重复登入数据、多个 TcpClient 快速握手。
3. 稳定后，再设计同 API Key 第二次 LoginGetQRCar 复用验证。
4. 不要恢复 cmdchat。
5. 不要继续 Win。
6. 不要反向篡改 Redis。
7. 不要修改后端登录主链路。

请输出：
1. 当前应执行的验证命令
2. 每条命令的作用
3. 通过标准
4. 如果失败，应该停止哪些服务或清理哪些 key
5. 下一步是否可以做第二次取码复用验证
```

## 敏感日志提醒

以后不要整段发送完整“登入数据”。

完整登录数据里包含：

```text
Sessionkey
Autoauthkey
Clientsessionkey
Serversessionkey
MmtlsKey
Proxy
DeviceToken
Pwd
NotifyKey
```

这些都属于敏感在线态字段。

以后只发关键摘要：

```text
wxid
uuid
Deviceid_str
DeviceType
ClientVersion
DeviceInfo.deviceid
AutoHeartBeatList
login_uuid_consumed
是否出现 online_guard / send_guard
是否重复“登入数据”
是否多个 TcpClient 快速握手
```
当前结论

Car 链路目前可以认为进入稳定阶段：

Car 登录成功 ✅
Car 独立档案保存 ✅
同 API Key 二次取码复用 ✅
前端轮询不误判 ✅
后端 uuid consumed 生效 ✅
心跳稳定 2 天 ✅