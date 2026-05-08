# DEV_CONTEXT.md

## 项目定位

这是一个已经开发到一半的 Go 项目，不是学习项目。  
当前目标是继续修复原有项目问题，同时把项目开发上下文和个人学习路线分开管理。

## 当前核心问题

账号成功登录模拟器后：

- 不调用自动心跳：风险低，但 API 捕捉不到手机消息。
- 调用自动心跳：可以正常获取消息，但不久后容易被腾讯判断为模拟器/异常客户端操作，导致封禁风险。

## 已完成的重要修改

本次已提交：

`19c0b6f Disable automatic relogin during heartbeat`

核心改动目标：

把“自动心跳”和“二次登录/恢复登录”拆开。

### 已修改文件

1. `srv/wxcore/wxconnect.go`

已处理：

- 自动心跳循环里不再监听 `RefreshTokenTimer`
- 启动心跳后，不再自动安排 1 分钟后的二次登录
- 长连心跳失败后，不再自动调用二次登录恢复连接
- `RefreshToken` 只保留给前端手动二次登录接口使用
- `SendRefreshTokenWaitingMinutes` 保留旧入口，但不再真正安排自动二次登录

2. `models/Login/Secautoauth.go`

已处理：

- 二次登录时不再硬编码新的 iPad 设备信息
- 优先使用登录缓存里的 `Imei / SoftType / DeviceType`
- 缓存为空时才按当前账号类型兜底

3. `models/Msg/sync.go`

已处理：

- 消息同步请求不再硬编码 `DeviceType: "iPhone"`
- 改为使用当前账号缓存里的 `D.DeviceType`
- 空值时兜底为 `iPad`

4. `conf/app.conf`

已处理：

- `longlink_recover_before_stop` 改为 `false`
- 注释说明：当前代码不再使用自动二次登录救活长连

## 当前行为状态

现在自动路径应该是：

- 自动心跳：保留
- 自动二次登录：关闭
- 心跳失败后自动恢复登录：关闭
- 前端手动二次登录接口：保留
- 前端唤醒/手动操作入口：保留
- 设备信息：尽量沿用登录时缓存，减少混乱

## Git / 部署状态

GitHub 已通过 SSH 推送成功。

origin 已改成 SSH：

`git@github.com:lxx9321/legendary-bot.git`

以后推送使用：

```bash
git push origin main