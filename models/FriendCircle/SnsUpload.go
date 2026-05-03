package FriendCircle

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/lunny/log"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"time"
	"wechatdll/Algorithm"
	"wechatdll/Cilent/mm"
	"wechatdll/baseinfo"
	"wechatdll/clientsdk"
	"wechatdll/clientsdk/baseutils"
	"wechatdll/comm"
	"wechatdll/models"
)

type SnsUploadParam struct {
	Wxid   string
	Base64 string
}

// 朋友圈上传视频,CDN
type SnsUploadVideoParam struct {
	Wxid      string
	VideoData string
	ThumbData string
}

// 普通视频上传
type SnsVideoUploadItem struct {
	AesKey     []byte // 加密用的AesKey
	Seq        uint32 // 代表第几个请求
	VideoID    uint32 // ID
	CreateTime uint32 // 创建时间
	VideoData  []byte // 视频数据
	ThumbData  []byte // ThumbData
	CDNDns     mm.GetCDNDnsResponse
}

type CdnSnsVideoUploadRequest struct {
	Ver              uint32 // 1
	WeiXinNum        uint32 //
	Seq              uint32 // 6
	ClientVersion    uint32
	ClientOsType     string
	AuthKey          []byte
	NetType          uint32 // 1
	AcceptDupack     uint32 // 1
	RsaVer           uint32 // 1
	RsaValue         []byte
	FileType         uint32 // 2
	WxChatType       uint32 // 1
	LastRetCode      uint32 // 0
	IPSeq            uint32 // 0
	CliQuicFlag      uint32 // 0
	HasThumb         uint32 // 1
	NoCheckAesKey    uint32 // 1
	EnableHit        uint32 // 1
	ExistAnceCheck   uint32 // 0
	AppType          uint32 // 1
	FileKey          string // wxupload_21533455325@chatroom29_1572079793
	TotalSize        uint32 // 53440
	RawTotalSize     uint32 // 53425
	LocalName        string // 29.wxgf
	Offset           uint32 // 0
	ThumbTotalSize   uint32 // 4496
	RawThumbSize     uint32 // 4487
	RawThumbMD5      string // 0d29df2b74d29efa46dd6fa1e75e71ba
	ThumbCRC         uint32 // 2991702343
	IsStoreVideo     uint32
	ThumbData        []byte
	LargesVideo      uint32 // 0
	SourceFlag       uint32 // 0
	AdVideoFlag      uint32 // 0
	Mp4Identify      string
	FileMD5          string // e851e118f524b4219928bed3f3bd0d24
	RawFileMD5       string // e851e118f524b4219928bed3f3bd0d24
	DataCheckSum     uint32 // 737909102
	FileCRC          uint32 // 2444306137
	FileData         []byte // 文件数据
	UserLargeFileApi bool
}

// CdnSnsVideoUploadResponse 上传朋友圈视频响应
type CdnSnsVideoUploadResponse struct {
	Ver        uint32
	Seq        uint32
	RetCode    uint32
	FileKey    string
	RecvLen    uint32
	FileURL    string
	ThumbURL   string
	FileID     string
	EnableQuic uint32
	RetrySec   uint32
	IsRetry    uint32
	IsOverLoad uint32
	IsGetCDN   uint32
	XClientIP  string
	ReqData    *CdnSnsVideoUploadRequest
}

type DownloadMediaModel struct {
	Key  string
	Url  string
	Wxid string
}
type SnsVideoDownloadItem struct {
	Seq           uint32        // 代表第几个请求
	URL           string        // 视频加密地址
	RangeStart    uint32        // 起始地址
	RangeEnd      uint32        // 结束地址
	XSnsVideoFlag string        // 视频标志
	CDNDns        mm.CDNDnsInfo // DNS信息
}
type cdnInfo struct {
	snsDns  *mm.CDNDnsInfo
	appDns  *mm.CDNDnsInfo
	cdnDns  *mm.CDNDnsInfo
	fakeDns *mm.CDNDnsInfo
}

func SnsUpload(Data SnsUploadParam) models.ResponseResult {
	var err error
	var protobufdata []byte
	var errtype int64
	var Bs64Data []byte

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

	Base64Data := strings.Split(Data.Base64, ",")

	if len(Base64Data) > 1 {
		Bs64Data, _ = base64.StdEncoding.DecodeString(Base64Data[1])
	} else {
		Bs64Data, _ = base64.StdEncoding.DecodeString(Data.Base64)
	}

	Stream := bytes.NewBuffer(Bs64Data)

	Bs64MD5 := baseutils.GetFileMD5Hash(Bs64Data)

	Startpos := 0
	datalen := 50000
	datatotalength := Stream.Len()

	ClientImgId := fmt.Sprintf("%v_%v", Data.Wxid, time.Now().Unix())

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

		req := &mm.SnsUploadRequest{
			BaseRequest: &mm.BaseRequest{
				SessionKey:    D.Sessionkey,
				Uin:           proto.Uint32(D.Uin),
				DeviceId:      D.Deviceid_byte,
				ClientVersion: proto.Int32(int32(D.ClientVersion)),
				DeviceType:    []byte(D.DeviceType),
				Scene:         proto.Uint32(0),
			},
			Type:     proto.Uint32(2),
			StartPos: proto.Uint32(uint32(Startpos)),
			TotalLen: proto.Uint32(uint32(datatotalength)),
			Buffer: &mm.SKBuiltinBufferT{
				ILen:   proto.Uint32(uint32(len(Databuff))),
				Buffer: Databuff,
			},
			ClientId: proto.String(ClientImgId),
			MD5:      proto.String(Bs64MD5),
		}

		//序列化
		reqdata, _ := proto.Marshal(req)

		//发包
		protobufdata, _, errtype, err = comm.SendRequest(comm.SendPostData{
			Ip:     D.Mmtlsip,
			Host:   D.ShortHost,
			Cgiurl: "/cgi-bin/micromsg-bin/mmsnsupload", ///cgi-bin/micromsg-bin/uploadvideo
			Proxy:  D.Proxy,
			PackData: Algorithm.PackData{
				Reqdata:          reqdata,
				Cgi:              207,
				Uin:              D.Uin,
				Cookie:           D.Cooike,
				Sessionkey:       D.Sessionkey,
				EncryptType:      5,
				Loginecdhkey:     D.RsaPublicKey,
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
	Response := mm.SnsUploadResponse{}
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

func CreateID(data []byte) uint32 {
	length := len(data)
	if length < 1 {
		return 0
	}

	tmpTotalLength := uint32(length)
	if length>>2 > 0 {
		tmpLen := length>>2 + 1

		index := 0
		for tmpLen > 1 {
			value0 := uint32(data[index])
			value1 := uint32(data[index+1])
			value2 := uint32(data[index+2])
			value3 := uint32(data[index+3])

			v5 := (value0 | (value1 << 8)) + tmpTotalLength
			v6 := value2 | (value3 << 8)
			tmpValue := (v5 ^ (v5 << 16) ^ (v6 << 11))
			tmpTotalLength = tmpValue + (tmpValue >> 11)
			index = index + 4
			tmpLen = tmpLen - 1
		}
	}

	caseValue := length & 3
	if caseValue == 1 {
		tmpValue0 := uint32(data[0])
		tmpValue := tmpTotalLength + tmpValue0
		tmpValue2 := tmpValue ^ (tmpValue << 10)
		tmpTotalLength = tmpValue2 + (tmpValue2 >> 1)
	}

	if caseValue == 2 {
		value0 := uint32(data[0])
		value1 := uint32(data[1])
		tmpValue0 := value0 | (value1 << 8)
		tmpValue := tmpTotalLength + tmpValue0
		tmpValue2 := tmpValue ^ (tmpValue << 11)
		tmpTotalLength = tmpValue2 + (tmpValue2 >> 17)
	}

	if caseValue == 3 {
		value0 := uint32(data[0])
		value1 := uint32(data[1])
		value2 := uint32(data[2])
		tmpValue0 := (value0 | (value1 << 8)) + tmpTotalLength
		tmpValue1 := tmpValue0 ^ (value2 << 18)
		tmpValue2 := tmpValue1 ^ (tmpValue0 << 16)
		tmpTotalLength = tmpValue2 + (tmpValue2 >> 11)
	}

	tmpValue0 := tmpTotalLength ^ (8 * tmpTotalLength)
	tmpValue1 := tmpValue0 + (tmpValue0 >> 5)
	tmpValue2 := tmpValue1 ^ (16 * tmpValue1)
	tmpValue3 := tmpValue2 + (tmpValue2 >> 17)
	tmpValue4 := tmpValue3 ^ (tmpValue3 << 25)
	tmpValue5 := tmpValue4 + (tmpValue4 >> 6)

	return tmpValue5
}

// CreateCdnSnsVideoUploadRequest 上传朋友圈视频
func CreateCdnSnsVideoUploadRequest(D *comm.LoginData, videoUploadItem *SnsVideoUploadItem) (*CdnSnsVideoUploadRequest, error) {
	request := &CdnSnsVideoUploadRequest{}
	request.Ver = 1
	request.WeiXinNum = uint32(videoUploadItem.CDNDns.SnsDnsInfo.GetUin())
	request.Seq = videoUploadItem.Seq
	request.ClientVersion = uint32(D.ClientVersion)
	if D.DeviceInfo == nil {
		request.ClientOsType = Algorithm.AndroidDeviceType
	} else {
		request.ClientOsType = D.DeviceType
	}
	request.AuthKey = videoUploadItem.CDNDns.SnsDnsInfo.AuthKey.GetBuffer()
	request.NetType = 1
	request.AcceptDupack = 1
	request.RsaVer = 1
	rsaValue, _ := baseutils.CdnRsaEncrypt(videoUploadItem.AesKey)
	request.RsaValue = rsaValue
	request.FileType = 20203 //20303
	request.WxChatType = 0
	request.LastRetCode = 0
	request.IPSeq = 0
	request.CliQuicFlag = 0
	request.IsStoreVideo = 0
	request.NoCheckAesKey = 1
	request.EnableHit = 1
	request.ExistAnceCheck = 0
	request.AppType = 102
	totalSize := uint32(len(videoUploadItem.VideoData))
	request.TotalSize = totalSize
	request.RawTotalSize = totalSize
	tmpLocalNameNoExt := "[TEMP]" + strconv.Itoa(int(videoUploadItem.VideoID)) + "_" + strconv.Itoa(int(videoUploadItem.CreateTime))
	request.LocalName = tmpLocalNameNoExt + ".mp4"
	request.FileKey = tmpLocalNameNoExt + "_" + strconv.Itoa(int(videoUploadItem.CreateTime)+rand.Intn(1000000000))
	request.Offset = 0

	// 暂时不设置Thumb数据
	thumbDataLen := uint32(len(videoUploadItem.ThumbData))
	request.HasThumb = 1
	request.ThumbTotalSize = thumbDataLen
	request.RawThumbSize = thumbDataLen
	request.RawThumbMD5 = baseutils.Md5ValueByte(videoUploadItem.ThumbData, false)
	request.ThumbCRC = baseutils.Adler32(0, videoUploadItem.ThumbData)
	request.ThumbData = videoUploadItem.ThumbData

	request.LargesVideo = 80
	request.SourceFlag = 0
	request.AdVideoFlag = 0
	fileEncodeData := baseutils.AesEncryptECB(videoUploadItem.VideoData, videoUploadItem.AesKey)
	request.Mp4Identify = baseutils.Md5ValueByte(fileEncodeData, false)
	md5Value := baseutils.Md5ValueByte(videoUploadItem.VideoData, false)
	request.FileMD5 = md5Value
	request.RawFileMD5 = md5Value
	request.FileCRC = baseutils.Adler32(0, videoUploadItem.VideoData)
	request.DataCheckSum = baseutils.Adler32(0, fileEncodeData)
	request.FileData = videoUploadItem.VideoData
	//request.UserLargeFileApi=true
	return request, nil
}

// DecodeSnsVideoUploadResponse 解析上传朋友圈视频响应
func DecodeSnsVideoUploadResponse(data []byte) (*CdnSnsVideoUploadResponse, error) {
	if len(data) < 25 {
		return nil, errors.New("DecodeSnsVideoUploadResponse err: len(data) < 25")
	}

	response := &CdnSnsVideoUploadResponse{}

	// 头的总长度 固定为25个字节
	headerLength := uint32(25)
	// 解析头部
	// bodyLength
	bodyLength := baseutils.BytesToInt32(data[21:25])
	// 解析包体
	body := data[headerLength:]
	offset := uint32(0)
	for offset < bodyLength {
		fieldNameSize := baseutils.BytesToInt32(body[offset : offset+4])
		fieldName := string(body[offset+4 : offset+4+fieldNameSize])
		offset = offset + fieldNameSize + 4

		valueSize := baseutils.BytesToInt32(body[offset : offset+4])
		value := string(body[offset+4 : offset+4+valueSize])
		offset = offset + valueSize + 4

		// Ver
		if fieldName == "ver" {
			ver, _ := strconv.Atoi(value)
			response.Ver = uint32(ver)
		}

		// Seq
		if fieldName == "seq" {
			seq, _ := strconv.Atoi(value)
			response.Seq = uint32(seq)
		}

		// RetCode
		if fieldName == "retcode" {
			retcode, _ := strconv.Atoi(value)
			fmt.Println(retcode)
			response.RetCode = uint32(retcode)
		}

		// FileKey
		if fieldName == "filekey" {
			response.FileKey = value
		}

		// FileURL
		if fieldName == "fileurl" {
			response.FileURL = value
		}

		// ThumbURL
		if fieldName == "thumburl" {
			response.ThumbURL = value
		}

		// FileID
		if fieldName == "fileid" {
			response.FileID = value
		}

		// RecvLen
		if fieldName == "recvlen" {
			recvlen, _ := strconv.Atoi(value)
			response.RecvLen = uint32(recvlen)
		}

		// RetrySec
		if fieldName == "retrysec" {
			retrysec, _ := strconv.Atoi(value)
			response.RetrySec = uint32(retrysec)
		}

		// IsRetry
		if fieldName == "isretry" {
			isretry, _ := strconv.Atoi(value)
			response.IsRetry = uint32(isretry)
		}

		// IsOverLoad
		if fieldName == "isoverload" {
			isoverload, _ := strconv.Atoi(value)
			response.IsOverLoad = uint32(isoverload)
		}

		// IsGetCdn
		if fieldName == "isgetcdn" {
			isgetcdn, _ := strconv.Atoi(value)
			response.IsGetCDN = uint32(isgetcdn)
		}

		// XClientIP
		if fieldName == "x-ClientIp" {
			response.XClientIP = value
		}

	}
	dataJson, _ := json.Marshal(response)
	log.Info(string(dataJson))
	return response, nil
}

func PackCdnSnsVideoUploadRequest(request *CdnSnsVideoUploadRequest) []byte {
	bodyData := make([]byte, 0)
	bodyData = append(bodyData, PackCdnRequestElementUint32("ver", request.Ver)[0:]...)
	bodyData = append(bodyData, PackCdnRequestElementUint32("weixinnum", request.WeiXinNum)[0:]...)
	bodyData = append(bodyData, PackCdnRequestElementUint32("seq", request.Seq)[0:]...)
	bodyData = append(bodyData, PackCdnRequestElementUint32("clientversion", request.ClientVersion)[0:]...)
	bodyData = append(bodyData, PackCdnRequestElementData("clientostype", []byte(request.ClientOsType))[0:]...)
	bodyData = append(bodyData, PackCdnRequestElementData("authkey", request.AuthKey[0:])...)
	bodyData = append(bodyData, PackCdnRequestElementUint32("nettype", request.NetType)[0:]...)
	bodyData = append(bodyData, PackCdnRequestElementUint32("acceptdupack", request.AcceptDupack)[0:]...)
	bodyData = append(bodyData, PackCdnRequestElementUint32("rsaver", request.RsaVer)[0:]...)
	bodyData = append(bodyData, PackCdnRequestElementData("rsavalue", request.RsaValue)[0:]...)
	bodyData = append(bodyData, PackCdnRequestElementUint32("filetype", request.FileType)[0:]...)
	bodyData = append(bodyData, PackCdnRequestElementUint32("wxchattype", request.WxChatType)[0:]...)
	bodyData = append(bodyData, PackCdnRequestElementUint32("lastretcode", request.LastRetCode)[0:]...)
	bodyData = append(bodyData, PackCdnRequestElementUint32("ipseq", request.IPSeq)[0:]...)
	bodyData = append(bodyData, PackCdnRequestElementUint32("cli-quic-flag", request.CliQuicFlag)[0:]...)
	bodyData = append(bodyData, PackCdnRequestElementUint32("isstorevideo", request.IsStoreVideo)[0:]...)
	bodyData = append(bodyData, PackCdnRequestElementUint32("hasthumb", request.HasThumb)[0:]...)
	bodyData = append(bodyData, PackCdnRequestElementUint32("nocheckaeskey", request.NoCheckAesKey)[0:]...)
	bodyData = append(bodyData, PackCdnRequestElementUint32("enablehit", request.EnableHit)[0:]...)
	bodyData = append(bodyData, PackCdnRequestElementUint32("existancecheck", request.ExistAnceCheck)[0:]...)
	bodyData = append(bodyData, PackCdnRequestElementUint32("apptype", request.AppType)[0:]...)
	bodyData = append(bodyData, PackCdnRequestElementData("filekey", []byte(request.FileKey))[0:]...)
	bodyData = append(bodyData, PackCdnRequestElementUint32("totalsize", request.TotalSize)[0:]...)
	bodyData = append(bodyData, PackCdnRequestElementUint32("rawtotalsize", request.RawTotalSize)[0:]...)
	bodyData = append(bodyData, PackCdnRequestElementData("localname", []byte(request.LocalName))[0:]...)
	bodyData = append(bodyData, PackCdnRequestElementUint32("offset", request.Offset)[0:]...)
	bodyData = append(bodyData, PackCdnRequestElementUint32("thumbtotalsize", request.ThumbTotalSize)[0:]...)
	bodyData = append(bodyData, PackCdnRequestElementUint32("rawthumbsize", request.RawThumbSize)[0:]...)
	bodyData = append(bodyData, PackCdnRequestElementData("rawthumbmd5", []byte(request.RawThumbMD5))[0:]...)
	bodyData = append(bodyData, PackCdnRequestElementUint32("thumbcrc", request.ThumbCRC)[0:]...)
	bodyData = append(bodyData, PackCdnRequestElementData("thumbdata", request.ThumbData)[0:]...)
	bodyData = append(bodyData, PackCdnRequestElementUint32("largesvideo", request.LargesVideo)[0:]...)
	bodyData = append(bodyData, PackCdnRequestElementUint32("sourceflag", request.SourceFlag)[0:]...)
	bodyData = append(bodyData, PackCdnRequestElementUint32("advideoflag", request.AdVideoFlag)[0:]...)
	bodyData = append(bodyData, PackCdnRequestElementData("mp4identify", []byte(request.Mp4Identify))[0:]...)
	bodyData = append(bodyData, PackCdnRequestElementData("filemd5", []byte(request.FileMD5))[0:]...)
	bodyData = append(bodyData, PackCdnRequestElementData("rawfilemd5", []byte(request.RawFileMD5))[0:]...)
	bodyData = append(bodyData, PackCdnRequestElementUint32("datachecksum", request.DataCheckSum)[0:]...)
	bodyData = append(bodyData, PackCdnRequestElementUint32("filecrc", request.FileCRC)[0:]...)
	bodyData = append(bodyData, PackCdnRequestElementData("filedata", request.FileData)[0:]...)

	retData := make([]byte, 0)
	// 包头
	retData = append(retData, 0xab)
	// 总长度
	totalLength := uint32(25 + len(bodyData))
	retData = append(retData, baseutils.Int32ToBytes(totalLength)[0:]...)
	// Flag标志
	flag := uint16(10002)
	retData = append(retData, baseutils.Int16ToBytesBigEndian(flag)[0:]...)
	// weixinnum
	retData = append(retData, baseutils.Int32ToBytesLittleEndian(request.WeiXinNum)[0:]...)
	// 固定为空的数据
	zeroBytes := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	retData = append(retData, zeroBytes[0:]...)
	// bodyLength
	bodyLength := uint32(len(bodyData))
	retData = append(retData, baseutils.Int32ToBytes(bodyLength)[0:]...)
	// BodyData
	retData = append(retData, bodyData[0:]...)

	return retData
}

func SendCdnSnsVideoDownloadReuqest(req DownloadMediaModel) models.ResponseResult {
	retFileData := []byte{}
	lessLength := uint32(2000000)
	encLen := uint32(0)
	videoFlag := string("V2")
	tmpEncKey, _ := strconv.Atoi(req.Key)
	retryCount := uint32(0)
	D, err := comm.GetLoginatas(req.Wxid)
	var protobufdata []byte
	var errtype int64
	var cdnInfos *cdnInfo
	if err != nil || D == nil || D.Wxid == "" {
		errorMsg := fmt.Sprintf("异常：%v [%v]", "未找到登录信息", req.Wxid)
		if err != nil {
			errorMsg = fmt.Sprintf("异常：%v", err.Error())
		}
		return models.ResponseResult{
			Code:    -7,
			Success: false,
			Message: errorMsg,
			Data:    nil,
		}
	}
	video, _ := base64.StdEncoding.DecodeString(req.Url)
	videoUrl := string(video)
	for {
		// 生产SnsImgItem
		var snsVideoItem baseinfo.SnsVideoDownloadItem
		snsVideoItem.Seq = uint32(rand.Intn(10))
		snsVideoItem.URL = videoUrl
		snsVideoItem.RangeStart = uint32(len(retFileData))
		snsVideoItem.RangeEnd = snsVideoItem.RangeStart + lessLength
		snsVideoItem.XSnsVideoFlag = videoFlag

		req := &mm.GetCDNDnsRequest{
			BaseRequest: &mm.BaseRequest{
				SessionKey:    []byte{},
				Uin:           proto.Uint32(D.Uin),
				DeviceId:      D.Deviceid_byte,
				ClientVersion: proto.Int32(int32(D.ClientVersion)),
				DeviceType:    []byte(D.DeviceType),
				Scene:         proto.Uint32(0),
			},
			ClientIp: proto.String(""),
		}
		resp := &mm.GetCDNDnsResponse{}
		//序列化

		reqdata, _ := proto.Marshal(req)
		protobufdata, _, errtype, err = comm.SendRequest(comm.SendPostData{
			Ip:     D.Mmtlsip,
			Host:   D.ShortHost,
			Cgiurl: "/cgi-bin/micromsg-bin/getcdndns",
			Proxy:  D.Proxy,
			PackData: Algorithm.PackData{
				Reqdata:          reqdata,
				Cgi:              379,
				Uin:              D.Uin,
				Cookie:           D.Cooike,
				Sessionkey:       D.Sessionkey,
				EncryptType:      5,
				Loginecdhkey:     D.RsaPublicKey,
				Clientsessionkey: D.Clientsessionkey,
				UseCompress:      true,
			},
		}, D.MmtlsKey)
		err = proto.Unmarshal(protobufdata, resp)
		if err != nil {
			fmt.Println(errtype)
		}
		cdnInfos = &cdnInfo{
			appDns:  resp.AppDnsInfo,
			snsDns:  resp.SnsDnsInfo,
			cdnDns:  resp.DnsInfo,
			fakeDns: resp.FakeDnsInfo,
		}

		// 发送分片下载请求
		response, err := SendCdnSnsVideoDownloadReuqestPiece(D, cdnInfos, &snsVideoItem)
		if err != nil {
			fmt.Printf("cdn返回错误%v", err.Error())
			if retryCount < 3 {
				retryCount++
				continue
			}
			return models.ResponseResult{
				Code:    -8,
				Success: false,
				Message: "errcode",
				Data:    nil,
			}
		}
		retryCount = 0
		// 判断错误码
		if response.RetCode != 0 {
			return models.ResponseResult{
				Code:    -9,
				Success: false,
				Message: "errcode",
				Data:    nil,
			}
		}
		fmt.Println("88888")
		// 设置加密的字节数
		if encLen == 0 {
			encLen = response.XEncLen
		}

		// 合并数据
		retFileData = append(retFileData, response.FileData[0:]...)
		currentLen := uint32(len(retFileData))
		if currentLen >= response.TotalSize {
			break
		}

		// 如果没有读取完
		lessLength = response.TotalSize - currentLen
		videoFlag = response.XSnsVideoFlag
	}
	if tmpEncKey != 0 {
		retFileData = baseutils.DecryptSnsVideoData(retFileData, encLen, uint64(tmpEncKey))
	}
	// ioutil.WriteFile("log/1.mp4", retFileData, 0777)
	// 解密数据
	//return retFileData, nil
	fmt.Println(base64.StdEncoding.EncodeToString(retFileData))
	return models.ResponseResult{
		Code:    0,
		Success: true,
		Message: base64.StdEncoding.EncodeToString(retFileData),
		Data:    nil,
	}
}

// SendCdnSnsVideoDownloadReuqestPiece 分片下载
func SendCdnSnsVideoDownloadReuqestPiece(userInfo *comm.LoginData, info *cdnInfo, snsVideoItem *baseinfo.SnsVideoDownloadItem) (*baseinfo.CdnSnsVideoDownloadResponse, error) {
	// 创建朋友圈视频下载请求
	fmt.Println(1)
	request, err := CreateSnsVideoDownloadRequest(userInfo, info, snsVideoItem)
	if err != nil {
		fmt.Sprintf("创建朋友圈视频错误：%v", err.Error())
		return nil, err
	}
	jsonData, err := json.Marshal(request)
	fmt.Println(string(jsonData))

	// 打包请求
	sendData := clientsdk.PackCdnSnsVideoDownloadRequest(request)
	// 连接Cdn服务器
	serverIP := *info.snsDns.ZoneIPList[0].String_
	serverPort := info.snsDns.ZoneIPPortList[0].PortList[0]
	conn, err := ConnectCdnServer(serverIP, serverPort)
	if err != nil {
		fmt.Sprintf("cdn错误：%v", err.Error())
		return nil, err
	}
	// 发送数据
	conn.Write(sendData)
	defer conn.Close()

	// 接收响应信息
	// 接收响应信息，解析
	retData := CDNRecvData(conn)
	response, err := clientsdk.DecodeSnsVideoDownloadResponse(retData)
	if err != nil {
		return nil, err
	}
	// 判断错误码
	if response.RetCode != 0 {
		return nil, errors.New("下载朋友圈视频失败: ErrCode = " + clientsdk.GetErrStringByRetCode(response.RetCode))
	}
	return response, nil
}

// CreateSnsVideoDownloadRequest 创建Cdn下载朋友圈视频请求
func CreateSnsVideoDownloadRequest(userInfo *comm.LoginData, info *cdnInfo, snsVideoItem *baseinfo.SnsVideoDownloadItem) (*baseinfo.CdnSnsVideoDownloadRequest, error) {
	request := &baseinfo.CdnSnsVideoDownloadRequest{}
	request.Ver = 1
	request.WeiXinNum = uint32(info.snsDns.GetUin())
	request.Seq = snsVideoItem.Seq
	request.ClientVersion = uint32(userInfo.ClientVersion)

	if !(int(request.ClientVersion) > 0) {
		request.ClientVersion = baseinfo.ClientVersion
	}
	if userInfo.DeviceInfo == nil {
		request.ClientOsType = Algorithm.AndroidDeviceType
	} else {
		request.ClientOsType = userInfo.DeviceInfo.OsType
	}
	request.AuthKey = info.snsDns.AuthKey.GetBuffer()
	request.NetType = 1
	request.AcceptDupack = 1
	request.Signal = ""
	request.Scene = ""
	request.URL = snsVideoItem.URL
	request.RangeStart = snsVideoItem.RangeStart
	request.RangeEnd = snsVideoItem.RangeEnd
	request.LastRetCode = 0
	request.IPSeq = 0
	request.RedirectType = 0
	request.LastVideoFormat = 0
	request.VideoFormat = 2
	request.XSnsVideoFlag = snsVideoItem.XSnsVideoFlag
	return request, nil
}

// ConnectCdnServer 链接Cdn服务器
func ConnectCdnServer(ipAddress string, port uint32) (*net.TCPConn, error) {
	fmt.Println(ipAddress)
	strPort := strconv.Itoa(int(port))
	serverAddr := ipAddress + ":" + strPort

	fmt.Printf("cdn服务器地址%v:", serverAddr)
	tcpAddr, err := net.ResolveTCPAddr("tcp", serverAddr)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialTCP("tcp4", nil, tcpAddr)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

// CDNRecvData 发送Cdn数据
func CDNRecvData(conn *net.TCPConn) []byte {
	// 写数据
	// 接收数据
	retData := make([]byte, 0)
	buffer := make([]byte, 25)
	count, err := conn.Read(buffer)
	if err != nil {
		return []byte{}
	}

	// 读取返回数据
	retData = append(retData, buffer[0:count]...)
	// 数据总长度
	totalLength := ParseCdnResponseDataLength(retData)
	currentLength := uint32(len(retData))
	for currentLength < totalLength {
		lessCount := totalLength - currentLength
		buffer := make([]byte, lessCount)
		count, err := conn.Read(buffer)
		if err != nil {
			return []byte{}
		}
		if count > 0 {
			retData = append(retData, buffer[0:count]...)
			currentLength = uint32(len(retData))
		} else {
			break
		}
	}

	return retData
}
func ParseCdnResponseDataLength(data []byte) uint32 {
	totalLength := baseutils.BytesToInt32(data[1:5])
	return totalLength
}

func PackCdnRequestElementUint32(fieldName string, value uint32) []byte {
	retData := make([]byte, 0)

	// 写入字段名长度
	fieldNameLength := uint32(len(fieldName))
	fieldNameLengthData := baseutils.Int32ToBytes(fieldNameLength)
	retData = append(retData, fieldNameLengthData[0:]...)

	// 写入字段名称
	retData = append(retData, ([]byte(fieldName))[0:]...)

	// 字段值转成string
	valueString := strconv.Itoa(int(value))
	// 写入字段值字符串 长度
	valueStringLength := uint32(len(valueString))
	valueStringLengththData := baseutils.Int32ToBytes(valueStringLength)
	retData = append(retData, valueStringLengththData[0:]...)
	// 写入字段值字符串
	retData = append(retData, ([]byte(valueString))[0:]...)

	return retData
}

func PackCdnRequestElementData(fieldName string, value []byte) []byte {
	retData := make([]byte, 0)

	// 写入字段名长度
	fieldNameLength := uint32(len(fieldName))
	fieldNameLengthData := baseutils.Int32ToBytes(fieldNameLength)
	retData = append(retData, fieldNameLengthData[0:]...)

	// 写入字段名称
	retData = append(retData, ([]byte(fieldName))[0:]...)

	// 写入字段值字符串 长度
	valueLength := uint32(len(value))
	valueLengththData := baseutils.Int32ToBytes(valueLength)
	retData = append(retData, valueLengththData[0:]...)
	// 写入字段值字符串
	retData = append(retData, value[0:]...)

	return retData
}

func GetErrStringByRetCode(retCode uint32) string {
	if retCode == 4289864094 {
		return "大小超过限制"
	}
	return strconv.Itoa(int(retCode))
}
