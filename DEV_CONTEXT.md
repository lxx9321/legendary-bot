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

## 当前禁止操作

在没有明确确认之前，暂时不要：

1. 重新启用自动二次登录
2. 重新启用心跳失败自动恢复登录
3. 新增自动设备切换逻辑
4. 同时修改 car 和 iPad 两条登录链路
5. 扩大扫描范围到全项目
6. 批量重构 Redis 登录缓存结构