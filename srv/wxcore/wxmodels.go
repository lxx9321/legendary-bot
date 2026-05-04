package wxcore

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"wechatdll/Algorithm"
	"wechatdll/Cilent/mm"
	"wechatdll/TcpPoll"
	"wechatdll/comm"
	"wechatdll/models"
	"wechatdll/models/Login"
	"wechatdll/models/Msg"
	"wechatdll/srv/wxface"

	"github.com/astaxie/beego"

	"google.golang.org/protobuf/proto"
)

// WXModels 微信链接接口
type WXModels struct {
	wxconn *WXConnect
}

// NewWXReqInvoker 新建一个请求调用器
func NewWXModels(wxconn *WXConnect) wxface.IWXModels {
	return &WXModels{
		wxconn: wxconn,
	}
}

// 消息同步接口
func (m *WXModels) MsgSync(Data Msg.SyncParam) models.ResponseResult {
	return Msg.Sync(Data)
}

// 短链接心跳接口
func (m *WXModels) LoginHeartBeat(Wxid string) (models.ResponseResult, *mm.HeartBeatResponse) {
	return Login.HeartBeat(Wxid)
}

// 长连接心跳接口（含自动重试：建连/发包失败会 Remove 后退避再试，减轻瞬时断线影响）
func (m *WXModels) LoginHeartBeatLong(Wxid string) (models.ResponseResult, *mm.HeartBeatResponse) {
	retries := 3
	if v, err := beego.AppConfig.Int("longlink_heartbeat_retries"); err == nil && v > 0 {
		retries = v
	}
	delayMs := 800
	if v, err := beego.AppConfig.Int("longlink_heartbeat_retry_delay_ms"); err == nil && v >= 0 {
		delayMs = v
	}
	delay := time.Duration(delayMs) * time.Millisecond

	var lastMsg string
	for attempt := 1; attempt <= retries; attempt++ {
		if attempt > 1 {
			time.Sleep(delay)
			fmt.Printf("[LoginHeartBeatLong] wxid=%s 第 %d/%d 次重试（长连自动重连）...\n", Wxid, attempt, retries)
		}
		res, hr := m.loginHeartBeatLongOnce()
		if res.Success && hr != nil && hr.GetBaseResponse() != nil && hr.GetBaseResponse().GetRet() == 0 {
			return res, hr
		}
		lastMsg = res.Message
		if hr != nil && hr.GetBaseResponse() != nil {
			lastMsg = fmt.Sprintf("%s ret=%d", lastMsg, hr.GetBaseResponse().GetRet())
		}
	}
	return models.ResponseResult{
		Code:    -8,
		Success: false,
		Message: fmt.Sprintf("长连心跳失败（已重试 %d 次）：%s", retries, lastMsg),
		Data:    nil,
	}, nil
}

func (m *WXModels) loginHeartBeatLongOnce() (models.ResponseResult, *mm.HeartBeatResponse) {
	tcpManager, err := TcpPoll.GetTcpManager()
	if err != nil {
		return models.ResponseResult{
			Code:    -8,
			Success: false,
			Message: fmt.Sprintf("出错了: %v", err.Error()),
			Data:    nil,
		}, nil
	}
	userInfo := m.wxconn.GetWXAccount().GetUserInfo()
	D, err := comm.GetLoginata(userInfo.Wxid, nil)
	if err != nil || D == nil || D.Wxid == "" {
		errorMsg := fmt.Sprintf("LoginHeartBeatLong 出错了: %v [%v]", "未找到登录信息", userInfo.Wxid)
		if err != nil {
			errorMsg = fmt.Sprintf("LoginHeartBeatLong 出错了: %v", err.Error())
		}
		return models.ResponseResult{
			Code:    -8,
			Success: false,
			Message: errorMsg,
			Data:    nil,
		}, nil
	}
	client, err := tcpManager.GetClient(userInfo, m.MsgListen)
	if err != nil {
		return models.ResponseResult{
			Code:    -8,
			Success: false,
			Message: fmt.Sprintf("出错了: %v", err.Error()),
			Data:    nil,
		}, nil
	}
	req := &mm.HeartBeatRequest{
		BaseRequest: &mm.BaseRequest{
			SessionKey:    D.Sessionkey,
			Uin:           proto.Uint32(D.Uin),
			DeviceId:      D.Deviceid_byte,
			ClientVersion: proto.Int32(int32(D.ClientVersion)),
			DeviceType:    []byte(D.DeviceType),
			Scene:         proto.Uint32(2),
		},
		TimeStamp: proto.Uint32(uint32(time.Now().Unix())),
	}

	reqdata, err := proto.Marshal(req)
	sendData := Algorithm.Pack(reqdata, 518, D.Uin, D.Sessionkey, D.Cooike, D.Clientsessionkey, D.RsaPublicKey, 5, false)
	cmdId := 238
	protobufdata, err := client.MmtlsSend(sendData, cmdId, "238心跳")
	if err != nil {
		tcpManager.Remove(client)
		return models.ResponseResult{
			Code:    -8,
			Success: false,
			Message: fmt.Sprintf("系统异常：%v", err.Error()),
			Data:    nil,
		}, nil
	}
	HeartBeatResponse := mm.HeartBeatResponse{}
	err = proto.Unmarshal(*protobufdata, &HeartBeatResponse)
	if err != nil {
		tcpManager.Remove(client)
		return models.ResponseResult{
			Code:    -8,
			Success: false,
			Message: fmt.Sprintf("反序列化失败：%v", err.Error()),
			Data:    nil,
		}, nil
	}
	if br := HeartBeatResponse.GetBaseResponse(); br != nil && br.GetRet() != 0 {
		tcpManager.Remove(client)
		em := ""
		if br.GetErrMsg() != nil {
			em = br.GetErrMsg().GetString_()
		}
		return models.ResponseResult{
			Code:    -8,
			Success: false,
			Message: fmt.Sprintf("心跳业务失败：%s (ret=%d)", em, br.GetRet()),
			Data:    nil,
		}, &HeartBeatResponse
	}

	return models.ResponseResult{
		Code:    0,
		Success: true,
		Message: "成功",
		Data:    HeartBeatResponse,
	}, &HeartBeatResponse
}

// 二次登录接口
func (m *WXModels) LoginSecautoauth(Wxid string) (models.ResponseResult, *mm.UnifyAuthResponse) {
	return Login.Secautoauth(Wxid)
}

// 消息监听
func (m *WXModels) MsgListen(cmdId int) error {
	fmt.Println("接收到长链接消息，正在处理回调")
	wxid := m.wxconn.GetWXAccount().GetUserInfo().Wxid
	msgpush, _ := beego.AppConfig.Bool("msgpush")
	chatOn := Msg.CmdChatEnabled()
	if msgpush {
		WXDATA := Msg.Sync(Msg.SyncParam{Wxid: wxid, Synckey: "", Scene: 0})
		jsonValue, _ := json.Marshal(WXDATA)
		syncUrl := strings.Replace(beego.AppConfig.String("syncmessagebusinessuri"), "{0}", wxid, -1)
		reqBody := strings.NewReader(string(jsonValue))
		go comm.HttpPosthb(syncUrl, reqBody, nil, "", "", "", "")

		rabbitmqEnabled, err := beego.AppConfig.Bool("rabbitmq")
		if err != nil {
			return nil
		}
		if rabbitmqEnabled {
			comm.PublishRabbitMq(beego.AppConfig.String("rabbitmqexchange"), jsonValue)
		}
	} else {
		// 未开 msgpush 时原先不拉 Sync，微信内指令永远不会触发；开启 cmdchat 时仍要 Sync
		if chatOn {
			_ = Msg.Sync(Msg.SyncParam{Wxid: wxid, Synckey: "", Scene: 0})
		}
		syncUrl := strings.Replace(beego.AppConfig.String("syncmessagebusinessuri"), "{0}", wxid, -1)
		comm.HttpPosthb(syncUrl, strings.NewReader(""), nil, "", "", "", "")
	}

	return nil
}
