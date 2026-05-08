# legendary-bot 继续开发记录

记录日期：2026-05-07

## 当前状态

本轮最后成功推送到 GitHub 的提交：

```text
7de860e
```

服务器已手动拉取、编译并启动成功。最后一次确认结果是：

- `go build -o wxapi .` 不再报错
- `pm2 restart wxapi` 后程序正常开始运行
- 因测试微信号暂时无法重新登录，目前只能验证到服务启动和接口链路恢复这一层

本地工作区还有一个未提交文件：

```text
models/Msg/cmdchat.go
```

下次继续前，先检查这个文件的 diff，确认里面是否还有需要保留的指令改动或编码污染。

## 今天解决的问题

### 1. 服务器部署目录异常

服务器原来的 `/www/wwwroot/wxapi` 目录不是正常 Git 工作副本，表现为：

```text
On branch master
No commits yet
```

后来处理方式：

- 备份旧目录
- 重新 `git clone https://github.com/lxx9321/legendary-bot.git wxapi`
- 在新目录里编译 `go build -o wxapi .`
- 用 `pm2 restart wxapi` 启动

现在服务器目录已经是正常 Git 仓库，可以 `git pull origin main`。

### 2. 公司电脑 Git 推送方式

HTTPS 在当前网络环境里不稳定，后来切换到 SSH。

本地仓库远程地址已改成：

```text
git@github.com:lxx9321/legendary-bot.git
```

以后正常提交和推送仍然使用：

```bash
git add .
git commit -m "message"
git push origin main
```

不需要每次手写 SSH 地址。

### 3. 绑定主人无响应的真实原因

一开始以为是指令解析问题，后来通过日志确认：

- 手动调用 `/api/Login/AutoHeartBeat` 后，手机发指令才会进入日志
- 不调用时，手机发消息日志完全不跳

因此根因不是简单的“绑定主人命令没识别”，而是：

```text
登录/唤醒成功后，没有自动重新挂上 AutoHeartBeat 和 Msg.Sync 轮询
```

消息链路是：

```text
LoginAwaken / LoginTwiceAutoAuth
  -> AutoHeartBeat
  -> HeartBeat
  -> userService.AddUser
  -> Msg.Sync 轮询
  -> ProcessCmdChatAddMsgs
```

缺少 `AutoHeartBeat` 时，手机消息不会进入指令处理。

## 已提交的代码改动

### 47903e9

提交名：

```text
Auto restore heartbeat sync after login
```

主要改动：

- `LoginTwiceAutoAuth` 成功后自动补自动心跳
- `LoginAwaken` 成功后自动补自动心跳
- `/AutoHeartBeat` 逻辑收敛成统一入口
- 程序启动时自动恢复短链 `Msg.Sync` 轮询
- 关闭自动心跳时同时停止短链同步

这个提交最初有循环依赖问题，服务器编译报：

```text
import cycle not allowed
```

### 203a876

提交名：

```text
Fix heartbeat auto-restore import cycle
```

处理了 `models/Login` 和 `srv/wxcore` 之间的循环依赖。

这个提交仍有一个漏掉的 import，服务器编译报：

```text
controllers/Login.go:43:16: undefined: srv
```

### 7de860e

提交名：

```text
Fix auto heartbeat controller import
```

补回 `controllers/Login.go` 里需要的：

```go
import "wechatdll/srv"
```

服务器在这个版本上编译通过并正常启动。

## 服务器部署命令

下次需要手动部署时，在服务器执行：

```bash
cd /www/wwwroot/wxapi
git pull origin main
git rev-parse --short HEAD
go build -o wxapi .
pm2 restart wxapi
pm2 logs wxapi --lines 100
```

确认版本：

```bash
git rev-parse --short HEAD
```

确认端口：

```bash
ss -lntp | grep -E '8062|8063|8888'
```

确认日志：

```bash
tail -f /root/.pm2/logs/wxapi-out.log
tail -f /root/.pm2/logs/wxapi-error.log
```

## PM2 运行信息

当前 PM2 进程名：

```text
wxapi
```

常用命令：

```bash
pm2 show wxapi
pm2 restart wxapi
pm2 logs wxapi --lines 100
pm2 status
```

之前确认过 PM2 指向：

```text
script path: /www/wwwroot/wxapi/wxapi
exec cwd: /www/wwwroot/wxapi
```

## 下次需要继续验证

由于当前测试号被腾讯限制，暂时无法完整验证自动回复。下次号能上来后，按这个顺序测：

1. 不手动调用 `/AutoHeartBeat`
2. 走正常网页登录/唤醒流程
3. 手机给机器人发：

```text
#在吗
#绑定主人
绑定主人
claim
```

4. 看日志是否出现：

```text
[cmdchat]
robot=...
skip duplicate
回执发送失败
```

如果有 `[cmdchat]`，说明消息同步链路已经自动恢复。

如果没有 `[cmdchat]`，优先查：

- `LoginAwaken` 是否成功
- `LoginTwiceAutoAuth` 是否成功
- 自动心跳是否启动
- `userService.AddUser` 是否真的开始 `Msg.Sync` 轮询

## 风控风险判断

测试号被踢时，日志里出现了长连心跳失败、连续重试、二次登录后重拉长连等信息。

当前判断：

- 不是单纯“心跳频率太快”
- 更像是长连异常后恢复策略太激进
- 连续重试加二次登录可能放大了腾讯风控判断

下一阶段建议单独处理：

1. 长连心跳失败后使用退避重试
2. 二次登录增加冷却时间
3. 避免短时间内反复 `LoginSecautoauth`
4. 长连异常时降级短链，不要两边同时高频跑
5. 超过阈值后暂停该号自动恢复，并在日志里明确提示

相关文件：

```text
srv/wxcore/wxconnect.go
srv/wxcore/wxmodels.go
conf/app.conf
```

## 注意事项

- 本地 Windows 机器没有安装 Go，不能本地编译验证
- 服务器有 Go，可以在服务器编译
- 服务器 Git hook 没有自动拉到最新提交，本轮确认过 `git rev-parse --short HEAD` 还停在旧版本，所以部署不要完全依赖 hook
- 下次可以继续排查 hook 为什么没有触发或没有完成部署
## 2026-05-07 本地 Go 环境处理

本地 Windows 机器已经确认安装了 Go：

```text
C:\Program Files\Go\bin\go.exe
go version go1.26.2 windows/amd64
```

之前 `go` 命令不可用，是因为当前 VS/Codex 进程没有刷新 PATH。已经把 Go bin 目录追加到用户 PATH：

```text
C:\Program Files\Go\bin
```

新开的 VS Code 终端里应可直接执行：

```bash
go version
```

由于默认 `proxy.golang.org` 在当前网络下连接失败，已经把 Go 模块代理改成国内镜像：

```bash
go env -w GOPROXY=https://goproxy.cn,direct
go env -w GOSUMDB=sum.golang.google.cn
```

当前确认值：

```text
GOPROXY=https://goproxy.cn,direct
GOSUMDB=sum.golang.google.cn
```

本地已经成功编译过主程序：

```bash
go build -o wxapi_local.exe .
```

编译时发现 `models/Msg/cmdchat.go` 里有两处历史断字符串，已经正式修复并提交：

```text
5ee3bcd Fix cmdchat owner bind strings
```

这次修复把之前服务器手动 `sed` 修过的两行正式入库，后续服务器不再需要手动补：

```go
return "不能使用机器人号自身绑定。"
return "绑定失败：" + err.Error()
```
