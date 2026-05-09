package Login

import (
	"container/list"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"wechatdll/Algorithm"
	"wechatdll/comm"
	"wechatdll/models"
)

func CheckSecManualAuth(Data *comm.LoginData, ShortHost string) models.ResponseResult {
	if ShortHost == "" {
		ShortHost = Algorithm.MmtlsShortHost
		Data.ShortHost = ShortHost
	} else {
		Data.ShortHost = ShortHost
	}
	// 仅合并「同一客户端形态」的旧会话；否则同 wxid 下先 PC(Win) 再 iPad 扫码会把 Win 的
	// ClientVersion/DeviceType/DeviceToken 盖到当前 QR 流程上，导致 SecManualAuth 报「登录异常」。
	if Data.Wxid != "" {
		Datas, err := comm.GetLoginata(Data.Wxid, nil)
		if err != nil || Datas == nil || Datas.Uuid == "" {
		} else if Datas.DeviceType == Data.DeviceType && Datas.ClientVersion == Data.ClientVersion {
			Data.DeviceToken = Datas.DeviceToken
			if Data.DeviceType == Algorithm.CarDeviceType {
				Data.DeviceInfo = createCarDeviceInfo(Data)
			} else {
				Data.DeviceInfo = Datas.DeviceInfo
				Data.RomModel = Datas.RomModel
				Data.OsVersion = Datas.OsVersion
				Data.ClientVersion = Datas.ClientVersion
				Data.DeviceType = Datas.DeviceType
			}
		}
	}
	jsonData, err := json.Marshal(Data)
	fmt.Println("登入数据")
	fmt.Println(string(jsonData))
	loginRes, prikey, pubkey, Cookie, DeviceToken, err := SecManualAuth(Data)

	if err != nil {
		return models.ResponseResult{
			Code:    -8,
			Success: false,
			Message: fmt.Sprintf("登录异常: %v", err),
			Data:    err.Error(),
		}
	}

	//登录成功
	if loginRes.GetBaseResponse().GetRet() == 0 && loginRes.GetUnifyAuthSectFlag() > 0 {
		Wx_loginecdhkey := Algorithm.DoECDH713Key(prikey, loginRes.GetAuthSectResp().GetSvrPubEcdhkey().GetKey().GetBuffer())
		m := md5.New()
		m.Write(Wx_loginecdhkey)
		Data.Loginecdhkey = Wx_loginecdhkey
		ecdhdecrptkey := m.Sum(nil)
		Data.Uin = loginRes.GetAuthSectResp().GetUin()
		Data.Wxid = loginRes.GetAcctSectResp().GetUserName()
		Data.Alais = loginRes.GetAcctSectResp().GetAlias()
		Data.Mobile = loginRes.GetAcctSectResp().GetBindMobile()
		Data.Email = loginRes.GetAcctSectResp().GetBindEmail()
		Data.NickName = loginRes.GetAcctSectResp().GetNickName()
		Data.Cooike = Cookie
		Data.Sessionkey = Algorithm.AesDecrypt(loginRes.GetAuthSectResp().GetSessionKey().GetBuffer(), ecdhdecrptkey)
		Data.Sessionkey_2 = loginRes.GetAuthSectResp().GetSessionKey().GetBuffer()
		Data.Autoauthkey = loginRes.GetAuthSectResp().GetAutoAuthKey().GetBuffer()
		Data.Autoauthkeylen = int32(loginRes.GetAuthSectResp().GetAutoAuthKey().GetILen())
		Data.Serversessionkey = loginRes.GetAuthSectResp().GetServerSessionKey().GetBuffer()
		Data.Clientsessionkey = loginRes.GetAuthSectResp().GetClientSessionKey().GetBuffer()
		Data.DeviceToken = DeviceToken
		Data.ShortHost = comm.Rmu0000(*loginRes.NetworkSectResp.BuiltinIplist.ShortConnectIplist[0].Host)
		Data.LongHost = comm.Rmu0000(*loginRes.NetworkSectResp.BuiltinIplist.LongConnectIplist[0].Host)
		Data.RsaPublicKey = pubkey
		Data.RsaPrivateKey = prikey
		// 当前时间
		Data.LoginDate = time.Now().Unix() // 登录时间
		err := comm.CreateLoginData(Data, Data.Wxid, 0, nil)
		comm.RedisClient.Set("devId:"+Data.Deviceid_str, Data.Wxid, 0)

		if err != nil {
			return models.ResponseResult{
				Code:    -8,
				Success: false,
				Message: fmt.Sprintf("系统异常：%v", err.Error()),
				Data:    nil,
			}
		}
		return models.ResponseResult{
			Code:    0,
			Success: true,
			Message: "登录成功",
			Data:    &loginRes,
		}
	}

	//30系列转向
	if loginRes.GetBaseResponse().GetRet() == -301 {
		var Wx_newLong_Host, Wx_newshort_Host, Wx_newshortext_Host list.List

		dns_info := loginRes.GetNetworkSectResp().GetNewHostList().GetList()
		for _, v := range dns_info {
			if v.GetHost() == "long.weixin.qq.com" {
				ip_info := loginRes.GetNetworkSectResp().GetBuiltinIplist().GetLongConnectIplist()
				for _, ip := range ip_info {
					host := ip.GetHost()
					host = strings.Replace(host, string(byte(0x00)), "", -1)
					if host == v.GetRedirect() {
						Wx_newLong_Host.PushBack(host)
					}
				}
			} else if v.GetHost() == "short.weixin.qq.com" {
				ip_info := loginRes.GetNetworkSectResp().GetBuiltinIplist().GetShortConnectIplist()
				for _, ip := range ip_info {
					host := ip.GetHost()
					host = strings.Replace(host, string(byte(0x00)), "", -1)
					if host == v.GetRedirect() {
						Wx_newshort_Host.PushBack(host)
					}
				}
			} else if v.GetHost() == "extshort.weixin.qq.com" {
				ip_info := loginRes.GetNetworkSectResp().GetBuiltinIplist().GetShortConnectIplist()
				for _, ip := range ip_info {
					host := ip.GetHost()
					host = strings.Replace(host, string(byte(0x00)), "", -1)
					if host == v.GetRedirect() {
						Wx_newshortext_Host.PushBack(host)
					}
				}
			}
		}
		return CheckSecManualAuth(Data, Wx_newshort_Host.Front().Value.(string))
	}

	// 否则为微信业务失败（ret / errmsg）或协议字段不符合预期
	msg := "登录异常"
	if br := loginRes.GetBaseResponse(); br != nil {
		msg = fmt.Sprintf("登录异常: ret=%d", br.GetRet())
		if em := br.GetErrMsg(); em != nil {
			if s := em.GetString_(); s != "" {
				msg += " " + s
			}
		}
	}
	return models.ResponseResult{
		Code:    -8,
		Success: false,
		Message: msg,
		Data:    &loginRes,
	}
}
