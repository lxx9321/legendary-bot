package Tools

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"github.com/golang/protobuf/proto"
	"strings"
	"time"
	"wechatdll/Algorithm"
	"wechatdll/Cilent/wechat"
	"wechatdll/comm"
	"wechatdll/models"
)

func UploadAppAttach2(Data UploadParam) models.ResponseResult {
	var err error
	var protobufdata []byte
	var errtype int64
	var Bs64Data []byte

	D, err := comm.GetLoginata(Data.Wxid, nil)
	if err != nil {
		return models.ResponseResult{
			Code:    -8,
			Success: false,
			Message: fmt.Sprintf("异常：%v", err.Error()),
			Data:    nil,
		}
	}

	Base64Data := strings.Split(Data.Base64, ",")

	if len(Base64Data) > 1 {
		Bs64Data, _ = base64.StdEncoding.DecodeString(Base64Data[1])
	} else {
		Bs64Data, _ = base64.StdEncoding.DecodeString(Data.Base64)
	}

	Stream := bytes.NewBuffer(Bs64Data)

	Bs64MD5 := GetFileMD5Hash(Bs64Data)

	Startpos := 0
	datalen := 50000
	datatotalength := Stream.Len()

	ClientImgId := fmt.Sprintf("%v_%v_UploadFile", D.Wxid, time.Now().Unix())

	I := 0

	for {
		Startpos = I * datalen
		count := 0
		if datatotalength-Startpos > datalen {
			count = datalen
		} else {
			count = datatotalength - Startpos
		}
		if count < 0 {
			break
		}

		Databuff := make([]byte, count)
		_, _ = Stream.Read(Databuff)

		req := &wechat.UploadAppAttachRequest{
			BaseRequest: &wechat.BaseRequest{
				SessionKey:    []byte{},
				Uin:           proto.Uint32(D.Uin),
				DeviceId:      D.Deviceid_byte,
				ClientVersion: proto.Int32(int32(D.ClientVersion)),
				DeviceType:    []byte(D.DeviceType),
				Scene:         proto.Uint32(0),
			},
			AppId:           proto.String(""),
			SdkVersion:      proto.Uint32(0),
			ClientAppDataId: proto.String(ClientImgId),
			UserName:        proto.String(Data.Wxid),
			TotalLen:        proto.Uint32(uint32(datatotalength)),
			StartPos:        proto.Uint32(uint32(Startpos)),
			DataLen:         proto.Uint32(uint32(len(Databuff))),
			Data: &wechat.SKBuiltinBufferT{
				ILen:   proto.Uint32(uint32(len(Databuff))),
				Buffer: Databuff,
			},
			Type: proto.Uint32(6),
			Md5:  proto.String(Bs64MD5),
		}

		//序列化
		reqdata, _ := proto.Marshal(req)

		//发包
		protobufdata, _, errtype, err = comm.SendRequest(comm.SendPostData{
			Ip:     D.Mmtlsip,
			Host:   D.ShortHost,
			Cgiurl: "/cgi-bin/micromsg-bin/uploadappattach",
			Proxy:  D.Proxy,
			PackData: Algorithm.PackData{
				Reqdata:          reqdata,
				Cgi:              220,
				Uin:              D.Uin,
				Cookie:           D.Cooike,
				Sessionkey:       D.Sessionkey,
				EncryptType:      5,
				Loginecdhkey:     D.Loginecdhkey,
				Clientsessionkey: D.Clientsessionkey,
				UseCompress:      true,
			},
		}, D.MmtlsKey)

		if err != nil {
			break
		}

		I++
	}

	if err != nil {
		return models.ResponseResult{
			Code:    errtype,
			Success: false,
			Message: err.Error(),
			Data:    nil,
		}
	}

	//解包
	Response := wechat.UploadAppAttachResponse{}
	err = proto.Unmarshal(protobufdata, &Response)
	if err != nil {
		return models.ResponseResult{
			Code:    -8,
			Success: false,
			Message: fmt.Sprintf("反序列化失败：%v", err.Error()),
			Data:    nil,
		}
	}

	return models.ResponseResult{
		Code:    0,
		Success: true,
		Message: "成功",
		Data:    Response,
	}

}
