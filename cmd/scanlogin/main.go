// 扫码登录小工具：固定 DeviceID、轮询 LoginCheckQR；可选 SOCKS5 代理、可选 Redis 存设备档案。
//
// 直接改代码后运行（不先 go build）：
//
//	go run ./cmd/scanlogin -base http://127.0.0.1:8062
//
// 编译：
//
//	go build -o scanlogin ./cmd/scanlogin
//
// 代理（与 wxapi 一致，走 SOCKS5；ProxyIp 填 ip:port，账号密码分开）示例：
//
//	go run ./cmd/scanlogin -base http://127.0.0.1:8062 \
//	  -proxy-ip "IP:端口" -proxy-user "账号" -proxy-pass "密码"
//
// 默认 Redis 见代码里 defaultEmbed*；要改端口/密码/key 可改常量，或用 -redis-addr 等覆盖。
//
// 档案里会保存 device_id / data62 / wxid，以及上次使用的代理 proxy_ip 等（代理不是「设备指纹」字段，但建议一起存，出口 IP 尽量稳定）。
//
// 默认已启用本机 Redis 存档案（见下方 defaultEmbed*），平时只需：
//
//	go run ./cmd/scanlogin -base http://127.0.0.1:8062 -proxy-ip "ip:port" -proxy-user "u" -proxy-pass "p"
//
// 切换取码端点（同形参的 LoginGetQR* 系列），例如安卓 Pad：
//
//	go run ./cmd/scanlogin -base http://127.0.0.1:8062 -endpoint LoginGetQRPad
//
// 不用 Redis 时加：-redis-off
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-redis/redis"
)

// defaultEmbed* 可按服务器环境改一次，避免每次命令行带 -redis-addr / -redis-key。
// 多账号建议改 defaultEmbedRedisKey，或仍用命令行 -redis-key 覆盖。
const (
	defaultEmbedRedisAddr = "127.0.0.1:6379"
	defaultEmbedRedisPass = ""
	defaultEmbedRedisDB   = 0
	defaultEmbedRedisKey  = "scanlogin:profile:default"
)

var knownQREndpoints = map[string]struct{}{
	"LoginGetQR":            {},
	"LoginGetQRx":           {},
	"LoginGetQRPad":         {},
	"LoginGetQRPadx":        {},
	"LoginGetQRWin":         {},
	"LoginGetQRWinUnified":  {},
	"LoginGetQRWinUwp":      {},
	"LoginGetQRMac":         {},
	"LoginGetQRCar":         {},
	"LoginGetQRNotCode":     {},
	"LoginGetQRNotCodePush": {},
}

// 设备指纹（给微信侧看的）主要是 device_id、data62、机型等；不包含「代理」本身。
// 代理单独记在档案里：便于下次用同一出口 IP，减少「今天代理 A、明天服务器直连」这种跳变触发风控。
type profile struct {
	DeviceID string `json:"device_id"`
	Data62   string `json:"data62,omitempty"`
	Wxid     string `json:"wxid,omitempty"`
	// 上次使用的 SOCKS5（与 LoginGetQR 的 Proxy 一致）；密码敏感，Redis/文件权限请自行收紧。
	ProxyIP   string `json:"proxy_ip,omitempty"`
	ProxyUser string `json:"proxy_user,omitempty"`
	ProxyPass string `json:"proxy_pass,omitempty"`
}

type proxyInfo struct {
	ProxyIp       string `json:"ProxyIp"`
	ProxyUser     string `json:"ProxyUser"`
	ProxyPassword string `json:"ProxyPassword"`
}

type getQRReq struct {
	Proxy      proxyInfo `json:"Proxy"`
	DeviceID   string    `json:"DeviceID"`
	DeviceName string    `json:"DeviceName"`
	LoginType  string    `json:"LoginType"`
}

type apiEnvelope struct {
	Code     int64           `json:"Code"`
	Success  bool            `json:"Success"`
	Message  string          `json:"Message"`
	Data     json.RawMessage `json:"Data"`
	Data62   string          `json:"Data62"`
	DeviceId string          `json:"DeviceId"`
}

type qrPayload struct {
	Uuid     string `json:"Uuid"`
	QrUrl    string `json:"QrUrl"`
	QrBase64 string `json:"QrBase64"`
}

type storeConfig struct {
	redisClient *redis.Client
	redisKey    string
	filePath    string // 空表示不写文件
}

func (s *storeConfig) load() (profile, error) {
	var p profile
	var lastErr error

	if s.redisClient != nil && s.redisKey != "" {
		val, err := s.redisClient.Get(s.redisKey).Result()
		if err == nil && val != "" {
			if err := json.Unmarshal([]byte(val), &p); err != nil {
				return p, err
			}
			if p.DeviceID != "" || p.Wxid != "" || p.ProxyIP != "" {
				return p, nil
			}
		}
		if err != nil && err != redis.Nil {
			lastErr = err
		}
	}

	if s.filePath != "" {
		b, err := os.ReadFile(s.filePath)
		if err != nil {
			if os.IsNotExist(err) {
				return p, lastErr
			}
			return p, err
		}
		if err := json.Unmarshal(b, &p); err != nil {
			return p, err
		}
	}
	return p, lastErr
}

func (s *storeConfig) save(p profile) error {
	raw, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}

	if s.redisClient != nil && s.redisKey != "" {
		if err := s.redisClient.Set(s.redisKey, string(raw), 0).Err(); err != nil {
			return fmt.Errorf("redis SET: %w", err)
		}
		fmt.Println("已写入 Redis key:", s.redisKey)
	}

	if s.filePath != "" {
		if err := os.WriteFile(s.filePath, raw, 0600); err != nil {
			return fmt.Errorf("写文件: %w", err)
		}
		fmt.Println("已写入文件:", s.filePath)
	}
	return nil
}

func postJSON(client *http.Client, url string, body any) ([]byte, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func main() {
	base := flag.String("base", "http://127.0.0.1:8062", "wxapi 根地址")
	profPath := flag.String("profile", "scanlogin_profile.json", "设备档案 JSON 路径；仅用 Redis 时加 -nofile")
	interval := flag.Duration("interval", 3*time.Second, "轮询 LoginCheckQR 间隔")
	deviceName := flag.String("device", "iPad", "DeviceName")

	proxyIP := flag.String("proxy-ip", "", "SOCKS5 代理 host:port，如 1.2.3.4:29677（wxapi 要求 SOCKS）")
	proxyUser := flag.String("proxy-user", "", "代理账号")
	proxyPass := flag.String("proxy-pass", "", "代理密码")

	redisOff := flag.Bool("redis-off", false, "关闭 Redis，仅用本地 -profile 文件")
	redisAddr := flag.String("redis-addr", defaultEmbedRedisAddr, "Redis 地址（覆盖代码默认值）")
	redisPass := flag.String("redis-pass", defaultEmbedRedisPass, "Redis 密码")
	redisDB := flag.Int("redis-db", defaultEmbedRedisDB, "Redis DB")
	redisKey := flag.String("redis-key", defaultEmbedRedisKey, "Redis 档案键（覆盖代码默认值）")

	noFile := flag.Bool("nofile", false, "为 true 时不读写本地 profile 文件（仅 Redis 时与默认 Redis 配合）")

	endpoint := flag.String("endpoint", "LoginGetQR", "取码端点名（不含路径前缀），如 LoginGetQR、LoginGetQRPad、LoginGetQRWinUnified 等")

	flag.Parse()

	*base = strings.TrimSuffix(strings.TrimSpace(*base), "/")
	apiLogin := *base + "/api/Login"

	ep := strings.TrimSpace(*endpoint)
	ep = strings.TrimPrefix(ep, "/")
	ep = strings.TrimPrefix(ep, "api/Login/")
	ep = strings.TrimPrefix(ep, "/api/Login/")
	if ep == "" {
		fmt.Fprintln(os.Stderr, "-endpoint 不能为空")
		os.Exit(1)
	}
	if _, ok := knownQREndpoints[ep]; !ok {
		fmt.Fprintf(os.Stderr, "警告: -endpoint=%s 不在已知白名单内，将按原样调用 /api/Login/%s\n", ep, ep)
	}

	var st storeConfig
	st.filePath = *profPath
	if *noFile {
		st.filePath = ""
	}
	useRedis := !*redisOff && strings.TrimSpace(*redisAddr) != ""
	if useRedis {
		rc := redis.NewClient(&redis.Options{
			Addr:     strings.TrimSpace(*redisAddr),
			Password: *redisPass,
			DB:       *redisDB,
		})
		if _, err := rc.Ping().Result(); err != nil {
			_ = rc.Close()
			if st.filePath == "" {
				fmt.Fprintf(os.Stderr, "Redis 连接失败且无本地 -profile：%v\n", err)
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "警告: Redis 不可用(%v)，已改为仅使用本地文件 %s\n", err, st.filePath)
		} else {
			st.redisClient = rc
			st.redisKey = strings.TrimSpace(*redisKey)
			if st.redisKey == "" {
				_ = rc.Close()
				fmt.Fprintln(os.Stderr, "-redis-key 不能为空")
				os.Exit(1)
			}
			fmt.Println("已连接 Redis:", strings.TrimSpace(*redisAddr), "key:", st.redisKey)
			defer func() { _ = rc.Close() }()
		}
	}

	if st.filePath == "" && st.redisClient == nil {
		fmt.Fprintln(os.Stderr, "请至少保留本地 -profile，或启用 Redis（默认已启用，除非 -redis-off）")
		os.Exit(1)
	}

	client := &http.Client{Timeout: 120 * time.Second}

	p, err := st.load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取档案警告/错误: %v\n", err)
	}

	px := proxyInfo{
		ProxyIp:       strings.TrimSpace(*proxyIP),
		ProxyUser:     strings.TrimSpace(*proxyUser),
		ProxyPassword: strings.TrimSpace(*proxyPass),
	}
	// 命令行未指定时，用档案里上次保存的代理（方便长期同一静态 IP）
	if px.ProxyIp == "" && p.ProxyIP != "" {
		px.ProxyIp = p.ProxyIP
		px.ProxyUser = p.ProxyUser
		px.ProxyPassword = p.ProxyPass
		fmt.Printf("使用档案中的 SOCKS5 代理: %s 用户: %q\n", px.ProxyIp, px.ProxyUser)
	}
	if px.ProxyIp != "" {
		fmt.Printf("本次请求 SOCKS5: %s 用户: %q\n", px.ProxyIp, px.ProxyUser)
	}

	reqBody := getQRReq{
		Proxy:      px,
		DeviceID:   p.DeviceID,
		DeviceName: *deviceName,
		LoginType:  "",
	}

	qrURL := apiLogin + "/" + ep
	fmt.Printf("==> POST /api/Login/%s\n", ep)
	body, err := postJSON(client, qrURL, reqBody)
	if err != nil {
		fmt.Fprintf(os.Stderr, "请求失败: %v\n", err)
		os.Exit(1)
	}

	var env apiEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		fmt.Fprintf(os.Stderr, "解析 JSON 失败: %v\n原始: %s\n", err, string(body))
		os.Exit(1)
	}
	if !env.Success || env.Code != 1 {
		fmt.Fprintf(os.Stderr, "取码未成功 Code=%d Success=%v Message=%s\n原始: %s\n", env.Code, env.Success, env.Message, string(body))
		os.Exit(1)
	}

	var qd qrPayload
	if len(env.Data) > 0 && string(env.Data) != "null" {
		_ = json.Unmarshal(env.Data, &qd)
	}
	if qd.Uuid == "" {
		fmt.Fprintf(os.Stderr, "响应里没有 Uuid，原始: %s\n", string(body))
		os.Exit(1)
	}

	if env.DeviceId != "" {
		p.DeviceID = env.DeviceId
	}
	if env.Data62 != "" {
		p.Data62 = env.Data62
	}
	if px.ProxyIp != "" {
		p.ProxyIP = px.ProxyIp
		p.ProxyUser = px.ProxyUser
		p.ProxyPass = px.ProxyPassword
	}
	if err := st.save(p); err != nil {
		fmt.Fprintf(os.Stderr, "保存档案失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n--- 请用手机微信扫码 ---")
	if qd.QrUrl != "" {
		fmt.Println("QrUrl:", qd.QrUrl)
		_ = os.WriteFile("scanlogin_qr.url.txt", []byte(qd.QrUrl+"\n"), 0644)
		fmt.Println("（已保存到 scanlogin_qr.url.txt）")
	}
	if qd.QrBase64 != "" {
		const maxShow = 120
		s := qd.QrBase64
		if len(s) > maxShow {
			s = s[:maxShow] + "..."
		}
		fmt.Println("QrBase64 前缀:", s)
	}
	fmt.Println("\nUUID:", qd.Uuid)
	fmt.Printf("\n==> 轮询 LoginCheckQR 每 %v（Ctrl+C 结束）\n", *interval)

	checkURL := fmt.Sprintf("%s/LoginCheckQR?uuid=%s", apiLogin, qd.Uuid)
	for {
		time.Sleep(*interval)
		raw, err := postJSON(client, checkURL, map[string]any{})
		if err != nil {
			fmt.Println("轮询请求失败:", err)
			continue
		}
		var chk apiEnvelope
		if err := json.Unmarshal(raw, &chk); err != nil {
			fmt.Println("轮询 JSON 解析失败:", err, "原始:", string(raw))
			continue
		}
		fmt.Printf("[%s] Code=%d Success=%v Message=%s\n", time.Now().Format("15:04:05"), chk.Code, chk.Success, chk.Message)

		// 外层 Code=0、Message=成功 只表示 check 接口正常；Data.status 才是扫码进度（2=已确认，将进入二次登录）。
		if len(chk.Data) > 0 && chk.Code == 0 && chk.Success {
			var dm map[string]json.RawMessage
			if json.Unmarshal(chk.Data, &dm) == nil {
				for _, sk := range []string{"status", "Status"} {
					if rv, ok := dm[sk]; ok {
						var st int64
						if json.Unmarshal(rv, &st) == nil {
							switch st {
							case 0:
								fmt.Println("  → 扫码状态: 等待中（请在手机上完成扫码/确认登录）")
							case 1:
								fmt.Println("  → 扫码状态: 已扫码，请在手机上点「登录」确认（否则不会进入 status=2）")
							case 2:
								fmt.Println("  → 扫码状态: 已确认，正在走二次登录…")
							default:
								fmt.Printf("  → 扫码状态: status=%d\n", st)
							}
						}
						break
					}
				}
			}
		}

		if chk.Code == -8 {
			fmt.Println("  完整响应:", string(raw))
		}

		var m map[string]json.RawMessage
		if len(chk.Data) > 0 && json.Unmarshal(chk.Data, &m) == nil {
			for _, key := range []string{"UserName", "userName", "wxid", "Wxid"} {
				if v, ok := m[key]; ok {
					var s string
					if json.Unmarshal(v, &s) == nil && s != "" {
						p.Wxid = s
						_ = st.save(p)
						fmt.Println("已更新档案中的 wxid:", s)
					}
				}
			}
		}

		if chk.Code == -3 {
			fmt.Println("需要验证码流程，请按 Message / Data 处理。原始:", string(raw))
		}
		if chk.Success && chk.Code == 0 && strings.Contains(strings.ToLower(chk.Message), "成功") {
			var atConfirm bool
			var dm2 map[string]json.RawMessage
			if json.Unmarshal(chk.Data, &dm2) == nil {
				for _, sk := range []string{"status", "Status"} {
					if rv, ok := dm2[sk]; ok {
						var st int64
						if json.Unmarshal(rv, &st) == nil && st == 2 {
							atConfirm = true
						}
						break
					}
				}
			}
			if !atConfirm {
				fmt.Println("（尚未登录完成，继续轮询；若已点确认仍失败请看上面 Code=-8 的「完整响应」里 ret/errMsg。）")
			}
			fmt.Println("本轮响应:", string(raw))
		}
	}
}
