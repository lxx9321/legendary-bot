package FriendCircle

import (
	"encoding/base64"
	"fmt"
	"math/rand"
	"strings"
	"time"
	"wechatdll/Cilent/mm"
	"wechatdll/clientsdk/baseutils"
	"wechatdll/comm"
	"wechatdll/models"
	"wechatdll/models/Tools"
)

// CDN上传朋友圈视频
func CdnSnsUploadVideo(Data SnsUploadVideoParam) models.ResponseResult {
	var err error
	var videoData []byte
	var thumbData []byte
	WXDATA := Tools.GetCdnDns(Data.Wxid)
	if !WXDATA.Success {
		return WXDATA
	}

	// 连接tcp
	Dns := WXDATA.Data.(mm.GetCDNDnsResponse)
	D, err := comm.GetLoginata(Data.Wxid, nil)
	if err != nil || D == nil || D.Wxid == "" {
		errorMsg := fmt.Sprintf("异常：%v [%v]", "未找到登录信息", Data.Wxid)
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
	vData := strings.Split(Data.VideoData, ",")
	tdata := strings.Split(Data.ThumbData, ",")
	if len(vData) > 1 {
		videoData, _ = base64.StdEncoding.DecodeString(vData[1])
	} else {
		videoData, _ = base64.StdEncoding.DecodeString(Data.VideoData)
	}

	if len(tdata) > 1 {
		thumbData, _ = base64.StdEncoding.DecodeString(tdata[1])
	} else {
		thumbData, _ = base64.StdEncoding.DecodeString(Data.ThumbData)
	}

	snsVideoItem := &SnsVideoUploadItem{}
	snsVideoItem.Seq = uint32(rand.Intn(10))
	snsVideoItem.AesKey = []byte(baseutils.RandomStringByLength(16))
	snsVideoItem.VideoData = videoData
	snsVideoItem.ThumbData = thumbData
	snsVideoItem.VideoID = CreateID(videoData)
	snsVideoItem.CreateTime = uint32(time.Now().UnixNano() / 1000000000)
	snsVideoItem.CDNDns = Dns

	request, err := CreateCdnSnsVideoUploadRequest(D, snsVideoItem)

	// 打包请求
	sendData := PackCdnSnsVideoUploadRequest(request)
	// 连接Cdn服务器
	serverIP := *snsVideoItem.CDNDns.SnsDnsInfo.FontIPList[0].String_
	serverPort := snsVideoItem.CDNDns.SnsDnsInfo.FrontIPPortList[0].PortList[0]
	conn, err := ConnectCdnServer(serverIP, serverPort)
	if err != nil {
		return models.ResponseResult{
			Code:    -9,
			Success: false,
			Message: err.Error(),
			Data:    nil,
		}
	}
	// 发送数据
	conn.Write(sendData)
	defer conn.Close()
	retryCount := uint32(0)
	for {
		// 接收响应信息，解析
		retData := CDNRecvData(conn)
		response, err := DecodeSnsVideoUploadResponse(retData)
		if err != nil {
			if retryCount < 3 {
				retryCount++
				continue
			}
			return models.ResponseResult{
				Code:    -9,
				Success: false,
				Message: err.Error(),
				Data:    nil,
			}
		}

		retryCount = 0
		// 判断错误码
		if response.RetCode != 0 {
			return models.ResponseResult{
				Code:    -9,
				Success: false,
				Message: fmt.Sprintf("上传朋友圈视频失败: ErrCode = " + GetErrStringByRetCode(response.RetCode)),
				Data:    nil,
			}
		}

		// 判断 服务器是否接收完毕
		if response.RecvLen < request.TotalSize {
			continue
		}

		// 设置请求数据
		response.ReqData = request
		return models.ResponseResult{
			Code:    0,
			Success: true,
			Message: "成功",
			Data:    response,
		}
	}

}
