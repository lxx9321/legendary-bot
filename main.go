package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"wechatdll/TcpPoll"
	"wechatdll/comm"
	loginModel "wechatdll/models/Login"
	_ "wechatdll/routers"
	"wechatdll/srv/wxcore"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/context"
	"github.com/lunny/log"
)

// 主服务 HTTP 端口（app.conf httpport），供扫码独立端口页 /env.json 回填 API 地址。
func mainHTTPListenPort() int {
	p, err := beego.AppConfig.Int("httpport")
	if err != nil || p <= 0 {
		return 8062
	}
	return p
}

// 扫码静态页独立端口（app.conf scanloginhttpport）。填 0 则改回挂到主服务 /scanlogin/。
func scanloginStandalonePort() int {
	s := strings.TrimSpace(beego.AppConfig.String("scanloginhttpport"))
	if s == "" {
		return 8063
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 8063
	}
	return v
}

func startScanloginHTTPServer(scanPort, apiPort int) {
	const dir = "static/scanlogin"
	addr := fmt.Sprintf("0.0.0.0:%d", scanPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		msg := fmt.Sprintf("扫码页端口 %d 绑定失败: %v（外网会 ERR_EMPTY_RESPONSE）。请放行防火墙/安全组 TCP %d，或把 app.conf 里 scanloginhttpport 改为 0 改用 http://IP:%d/scanlogin/", scanPort, err, scanPort, apiPort)
		log.Errorf(msg)
		_, _ = fmt.Fprintln(os.Stderr, msg)
		return
	}
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/env.json" {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			_ = json.NewEncoder(w).Encode(map[string]int{"apiHttpPort": apiPort})
			return
		}
		http.FileServer(http.Dir(dir)).ServeHTTP(w, r)
	})
	boot := fmt.Sprintf("扫码页已监听 %s（浏览器: http://服务器公网IP:%d/ ；API/Swagger 仍在 %d；日志里 :8888 是验证代理）", ln.Addr().String(), scanPort, apiPort)
	log.Infof(boot)
	_, _ = fmt.Fprintln(os.Stderr, boot)
	go func() {
		if err := http.Serve(ln, h); err != nil {
			log.Errorf("扫码页独立服务退出: %v", err)
		}
	}()
}

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
	go loginModel.InitAutoSyncPolling()

	// 初始化验证地址
	comm.InitProxy("8888")
	//
	// 启动
	if err := comm.GetProxy().Start(); err != nil {
		log.Fatal(err)
	}

	beego.BConfig.WebConfig.DirectoryIndex = true
	beego.BConfig.WebConfig.StaticDir["/swagger"] = "swagger"
	apiPort := mainHTTPListenPort()
	switch sp := scanloginStandalonePort(); {
	case sp == 0:
		beego.BConfig.WebConfig.StaticDir["/scanlogin"] = "static/scanlogin"
		log.Infof("扫码页与主服务同端口: /scanlogin/（主端口 %d）", apiPort)
	default:
		startScanloginHTTPServer(sp, apiPort)
	}
	beego.Get("/", func(ctx *context.Context) {
		http.Redirect(ctx.ResponseWriter, ctx.Request, "/swagger/", http.StatusFound)
	})

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

