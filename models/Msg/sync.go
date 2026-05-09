package Msg

import (
	"fmt"
	"wechatdll/Algorithm"
	"wechatdll/Cilent/mm"
	"wechatdll/comm"
	"wechatdll/models"

	"github.com/golang/protobuf/proto"
)

type SyncParam struct {
	Wxid    string
	Scene   uint32
	Synckey string
}

type SyncResponse struct {
	ModUserInfos    []mm.ModUserInfo    //CmdId = 1
	ModContacts     []mm.ModContact     //CmdId = 2
	DelContacts     []mm.DelContact     //CmdId = 4
	ModUserImgs     []mm.ModUserImg     //CmdId = 35
	FunctionSwitchs []mm.FunctionSwitch //CmdId = 23
	UserInfoExts    []mm.UserInfoExt    //CmdId = 44
	AddMsgs         []mm.AddMsg         //CmdId = 5
	ContinueFlag    int32
	KeyBuf          mm.SKBuiltinBufferT
	Status          int32
	Continue        int32
	Time            int32
	UnknownCmdId    string
	Remarks         string
}

func Sync(Data SyncParam) models.ResponseResult {
	D, err := comm.GetLoginata(Data.Wxid, nil)
	if err != nil {
		return models.ResponseResult{
			Code:    -8,
			Success: false,
			Message: fmt.Sprintf("异常：%v", err.Error()),
			Data:    nil,
		}
	}

	var Synckey mm.SKBuiltinBufferT

	Synckey = mm.SKBuiltinBufferT{
		ILen:   proto.Uint32(uint32(len(D.SyncKey))),
		Buffer: D.SyncKey,
	}

	//if Data.Synckey != "" {
	//	key, _ := base64.StdEncoding.DecodeString(Data.Synckey)
	//	Synckey = mm.SKBuiltinBufferT{
	//		ILen:   proto.Uint32(uint32(len(key))),
	//		Buffer: key,
	//	}
	//}

	deviceType := D.DeviceType
	if deviceType == "" {
		deviceType = "iPad"
	}

	req := &mm.NewSyncRequest{
		Oplog: &mm.CmdList{
			Count: proto.Uint32(0),
			List:  nil,
		},
		Selector:      proto.Uint32(262151),
		KeyBuf:        &Synckey,
		Scene:         proto.Uint32(Data.Scene),
		DeviceType:    proto.String(deviceType),
		SyncMsgDigest: proto.Uint32(3),
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

	//发包
	protobufdata, _, errtype, err := comm.SendRequest(comm.SendPostData{
		Ip:     D.Mmtlsip,
		Host:   D.ShortHost,
		Cgiurl: "/cgi-bin/micromsg-bin/newsync",
		Proxy:  D.Proxy,
		PackData: Algorithm.PackData{
			Reqdata:          reqdata,
			Cgi:              138,
			Uin:              D.Uin,
			Cookie:           D.Cooike,
			Sessionkey:       D.Sessionkey,
			Loginecdhkey:     D.RsaPublicKey,
			Clientsessionkey: D.Clientsessionkey,
			Serversessionkey: D.Serversessionkey,
			UseCompress:      false,
		},
	}, D.MmtlsKey)

	if err != nil {
		return models.ResponseResult{
			Code:    errtype,
			Success: false,
			Message: err.Error(),
			Data:    nil,
		}
	}

	//解包
	NewSyncResponse := mm.NewSyncResponse{}

	err = proto.Unmarshal(protobufdata, &NewSyncResponse)
	if err != nil {
		return models.ResponseResult{
			Code:    -8,
			Success: false,
			Message: fmt.Sprintf("反序列化失败：%v", err.Error()),
			Data:    nil,
		}
	}

	UnknownCmdId := ""

	var ModUserInfos []mm.ModUserInfo
	var ModContacts []mm.ModContact
	var DelContacts []mm.DelContact
	var ModUserImgs []mm.ModUserImg
	var FunctionSwitchs []mm.FunctionSwitch
	var UserInfoExts []mm.UserInfoExt
	var AddMsgs []mm.AddMsg

	if NewSyncResponse.CmdList != nil && len(NewSyncResponse.CmdList.List) > 0 {
		for _, v := range NewSyncResponse.CmdList.List {
			switch *v.CmdId {
			case int32(mm.SyncCmdID_CmdIdModUserInfo): // CmdId = 1
				var data mm.ModUserInfo
				_ = proto.Unmarshal(v.CmdBuf.Buffer, &data)
				ModUserInfos = append(ModUserInfos, data)
			case int32(mm.SyncCmdID_CmdIdModContact): // CmdId = 2
				var data mm.ModContact
				_ = proto.Unmarshal(v.CmdBuf.Buffer, &data)
				ModContacts = append(ModContacts, data)
			case int32(mm.SyncCmdID_CmdIdDelContact): // CmdId = 4
				var data mm.DelContact
				_ = proto.Unmarshal(v.CmdBuf.Buffer, &data)
				DelContacts = append(DelContacts, data)
			case int32(mm.SyncCmdID_MM_SYNCCMD_MODUSERIMG): // CmdId = 35
				var data mm.ModUserImg
				_ = proto.Unmarshal(v.CmdBuf.Buffer, &data)
				ModUserImgs = append(ModUserImgs, data)
			case int32(mm.SyncCmdID_CmdIdFunctionSwitch): // CmdId = 23
				var data mm.FunctionSwitch
				_ = proto.Unmarshal(v.CmdBuf.Buffer, &data)
				FunctionSwitchs = append(FunctionSwitchs, data)
			case int32(mm.SyncCmdID_MM_SYNCCMD_USERINFOEXT): // CmdId = 44
				var data mm.UserInfoExt
				_ = proto.Unmarshal(v.CmdBuf.Buffer, &data)
				UserInfoExts = append(UserInfoExts, data)
			case int32(mm.SyncCmdID_CmdIdAddMsg): // CmdId = 5
				var data mm.AddMsg
				_ = proto.Unmarshal(v.CmdBuf.Buffer, &data)
				AddMsgs = append(AddMsgs, data)
			default:
				UnknownCmdId += UnknownCmdId + ";" + fmt.Sprintf("%v", *v.CmdId)
			}
		}

		// 将新的SyncKey保存到数据库
		loginDataMu := comm.GetLoginLock(D.Wxid)
		loginDataMu.Lock()
		latestD, latestErr := comm.GetLoginata(D.Wxid, loginDataMu)
		if latestErr != nil || latestD == nil || latestD.Wxid == "" {
			if latestErr != nil {
				fmt.Printf("[sync] skip SyncKey persist for wxid=%s: reload latest login data failed: %v\n", D.Wxid, latestErr)
			} else {
				fmt.Printf("[sync] skip SyncKey persist for wxid=%s: latest login data is empty\n", D.Wxid)
			}
		} else {
			latestD.SyncKey = NewSyncResponse.KeyBuf.Buffer
			if err := comm.CreateLoginData(latestD, latestD.Wxid, 0, loginDataMu); err != nil {
				fmt.Printf("[sync] persist SyncKey failed for wxid=%s: %v\n", latestD.Wxid, err)
			}
		}
		loginDataMu.Unlock()

		if len(AddMsgs) > 0 {
			robotID := D.Wxid
			msgsCopy := append([]mm.AddMsg(nil), AddMsgs...)
			go func() {
				defer func() {
					if r := recover(); r != nil {
						fmt.Printf("[cmdchat] ProcessCmdChatAddMsgs panic robot=%s: %v\n", robotID, r)
					}
				}()
				ProcessCmdChatAddMsgs(robotID, msgsCopy)
			}()
		}

		return models.ResponseResult{
			Code:    0,
			Success: true,
			Message: "成功",
			Data: SyncResponse{
				ModUserInfos:    ModUserInfos,
				ModContacts:     ModContacts,
				DelContacts:     DelContacts,
				ModUserImgs:     ModUserImgs,
				FunctionSwitchs: FunctionSwitchs,
				UserInfoExts:    UserInfoExts,
				AddMsgs:         AddMsgs,
				ContinueFlag:    *NewSyncResponse.ContinueFlag,
				KeyBuf: mm.SKBuiltinBufferT{
					ILen:   NewSyncResponse.KeyBuf.ILen,
					Buffer: NewSyncResponse.KeyBuf.Buffer,
				},
				Status:       *NewSyncResponse.Status,
				Continue:     *NewSyncResponse.Continue,
				Time:         *NewSyncResponse.Time,
				UnknownCmdId: UnknownCmdId,
				Remarks:      "出现未解析的CmdId类型数据,请联系客服人员处理。",
			},
		}
	}

	return models.ResponseResult{
		Code:    0,
		Success: true,
		Message: "当前未有新消息",
		Data:    NewSyncResponse,
	}
}

// SyncContinueDrain 在已成功执行过一次 Sync 后，若返回体为 SyncResponse 且 Continue!=0，则继续调用 Sync 直到拉完或达到 max 次（避免 AddMsg 被拆在多轮 NewSync 里导致指令漏处理）。
func SyncContinueDrain(wxID string, last models.ResponseResult, max int) {
	if max <= 0 {
		max = 12
	}
	cur := last
	for i := 0; i < max; i++ {
		sr, ok := cur.Data.(SyncResponse)
		if !ok || sr.Continue == 0 {
			return
		}
		cur = Sync(SyncParam{Wxid: wxID, Synckey: "", Scene: 0})
		if !cur.Success {
			return
		}
	}
}
