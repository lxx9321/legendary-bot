交接摘要



已完成的修改

只改过 models/Login/GetQRCodeCar.go

LoginGetQRCar 入口现在固定使用 Algorithm.CarDeviceName

不再信任前端传入的 DeviceName

只改过 models/Login/InitData.go

GenCarLoginData 改为使用 Car 专用 SoftType 和 Car 专用 DeviceInfo

UpdateCarLoginData 改为强制回正 Car 核心字段，不再保留旧 iPad / Android 残留

新增 createCarSoftType

新增 createCarDeviceInfo

新增 carDeviceBrand

修复了 createCarSoftType 里 k33 的乱码问题，使用：

wechatName := "\\u5fae\\u4fe1"

再通过 fmt.Sprintf(...<k33>%s</k33>...) 写入

只改过 models/Login/CheckSecManualAuth.go

Car 链路命中旧 wxid 缓存时，不再回灌旧 DeviceInfo

只保留 DeviceToken 复用

只要 Data.DeviceType == Algorithm.CarDeviceType，就强制：

Data.DeviceInfo = createCarDeviceInfo(Data)

非 Car 链路保持原逻辑不变

已验证成功的结果

用户已经重新部署并通过 LoginGetQRCar + /login/GetCacheInfo 验证，当前 Car 登录缓存内部字段已经基本自洽：



DeviceType = car-31

DeviceName = Xiaomi-M2012K11AC

SoftType 里的 <k9> 已不再是 iPad，而是 Xiaomi-M2012K11AC

DeviceInfo.deviceid / imei 已与顶层 Deviceid\_str / Imei 同源

DeviceInfo.devicename / devicebrand / ostype 已与当前 Car 登录数据一致

当前剩余问题

同一个账号再次使用 LoginGetQRCar 登录时，Deviceid\_str 仍然变化

也就是：Car 设备档案还没有复用

手机端仍然可能把它识别成“新设备”

当前根因分析结论：

LoginGetQRCar 取码前只会按 DeviceID 查：

comm.GetLoginataByDevId(Data.DeviceID)

如果前端没传 DeviceID，后端只能新建设备档案

当前没有按 wxid + car 读取旧设备档案的机制

devId:{DeviceID} -> wxid 映射是在登录成功后才建立

LoginCheckQR / CheckUuid 在扫码确认阶段 status == 2 时就能拿到 wxid

当前阶段目标

当前阶段目标已经切换为：



不依赖前端手动填写 DeviceID

实现后端自动复用 Car 设备档案

候选设计方向：

用 API Key / 调用者身份识别调用者

登录成功后保存：

last\_device\_profile:{apiKey}:car -> wxid

device\_profile:car:{wxid} -> 当前 Car 设备档案

下次 LoginGetQRCar 时：

如果前端没传 DeviceID，也没传 wxid

后端通过 apiKey + car 找到上次登录的 wxid

再读取 device\_profile:car:{wxid} 复用旧 Car 设备档案

下一步只读分析任务

新对话只做只读分析，不要改代码，重点回答这些问题：



当前项目是否已有 API Key / token / auth 鉴权机制

API Key 在哪里校验

LoginGetQRCar 的 controller 层能不能拿到 API Key

model 层 GetQRCodeCar.go 能不能拿到 API Key

如果 model 层拿不到，是否要从 controller 传 callerId

登录成功后的 CheckSecManualAuth 能不能知道当前调用者 API Key

现在是否有地方能安全保存：

last\_device\_profile:{apiKey}:car -> wxid

如果一个 API Key 对应多个 wxid，方案有什么风险

最小后端复用方案怎么设计

第一刀最小修改点在哪里

必须优先阅读的文件

必须优先阅读这些文件：



controllers/Login.go

models/Login/GetQRCodeCar.go

models/Login/GetQRCode.go

models/Login/CheckSecManualAuth.go

models/Login/CheckUuid.go

comm/Redis.go

middleware/apikey.go

routers/router.go

controllers/Base.go

如果还要补充上下文，再看：



DEV\_CONTEXT.md

禁止重复分析和禁止修改的内容

不要重复分析这些已经有明确结论的点：



Car 登录缓存字段混搭问题

DeviceName=iPad 前端污染问题

DeviceInfo 被旧缓存回灌的问题

createCarSoftType 的乱码问题

Car 设备字段同源问题

这些已经完成或已确认结论：



LoginGetQRCar 入口应固定使用 Car 设备名

Car DeviceInfo 必须和顶层字段同源

CheckSecManualAuth 下 Car 链路不能回灌旧 DeviceInfo

当前禁止修改的内容：



不要改 iPad 登录链路

不要改 Redis key 结构

不要改心跳

不要改 Sync

不要改 Secautoauth

不要提供规避风控或绕过验证方案

新对话第一步只做只读分析，不要直接改代码

