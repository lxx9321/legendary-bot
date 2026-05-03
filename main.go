package main

import (
	"fmt"
	"runtime"
	"wechatdll/TcpPoll"
	"wechatdll/comm"
	_ "wechatdll/routers"
	"wechatdll/srv/wxcore"

	"github.com/astaxie/beego"
	"github.com/lunny/log"
)

func main() {
	longLinkEnabled, _ := beego.AppConfig.Bool("longlinkenabled")

	comm.RedisInitialize()
	_, err := comm.RedisClient.Ping().Result()
	if err != nil {
		panic(fmt.Sprintf("【Redis】连接失败，ERROR：%v", err.Error()))
	}

	comm.RabbitMQSetup()

	sysType := runtime.GOOS

	if sysType == "linux" && longLinkEnabled {
		// LINUX系统
		tcpManager, err := TcpPoll.GetTcpManager()
		if err != nil {
			log.Errorf("TCP启动失败.")
		}
		go tcpManager.RunEventLoop()
	}

	// bee generate docs 生成文档

	// 初始化自动心跳包
	go wxcore.GetWXConnectMgr().InitAutoHeartBeat()

	// 初始化验证地址
	comm.InitProxy("8888")
	//
	// 启动
	if err := comm.GetProxy().Start(); err != nil {
		log.Fatal(err)
	}

	beego.BConfig.WebConfig.DirectoryIndex = true
	beego.BConfig.WebConfig.StaticDir["/"] = "swagger"

	beego.SetLogFuncCall(false)
	//beego.InsertFilter("/*", beego.BeforeRouter, middleware.BaseAuthLog, false)
	beego.Run()
	// 生成 swagger 文档 bee generate docs
}
//func GetA16Data(DevicelId string) string {
//	return "A" + DevicelId[1:16]
//}
//
//func main() {
//	fmt.Println(GetA16Data(("492c578eae9a79442e1adb4abadfb507")))
//}

