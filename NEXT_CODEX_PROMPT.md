\# 明天继续任务提示词



请先不要修改代码。



请先阅读：



1\. `DEV\_CONTEXT.md`

2\. `models/Login/ExtDeviceLoginConfirmGet.go`

3\. `models/Login/ExtDeviceLoginConfirmOk.go`



当前任务不是学习项目，而是继续修复一个开发到一半的 Go 项目。



我的要求：



1\. 不要重新扫描整个项目，先只看和这两个接口相关的最少文件。

2\. 不要做绕过平台验证、隐藏风控、规避检测相关方案。

3\. 只分析合法扫码确认流程里的设备信息一致性和代码稳定性。

4\. 重点检查：

&#x20;  - 是否硬编码 `DeviceName: "iPhone"`

&#x20;  - 是否应该改成沿用登录缓存里的设备信息

&#x20;  - `strings.Replace(Data.Url, "https", "http", -1)` 是否有误伤风险

&#x20;  - 是否只应该处理 URL scheme

5\. 修改代码前，先告诉我：

&#x20;  - 准备改哪些文件

&#x20;  - 为什么改

&#x20;  - 风险是什么

&#x20;  - 如何测试

6\. 每次只改一个小点，不要大改。

7\. 修改完成后告诉我：

&#x20;  - 改了哪些文件

&#x20;  - 怎么测试

&#x20;  - git status 怎么看

&#x20;  - 推荐 commit 信息

