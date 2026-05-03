package FriendCircle

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"regexp"
	"time"
	"wechatdll/Algorithm"
	"wechatdll/Cilent/mm"
	"wechatdll/comm"
	"wechatdll/models"
)

type IdDetailParams struct {
	Wxid   string
	Towxid string
	Id     uint64
}

// 解析 XML 数据并返回 IdDetailParams
func ParseXMLData(wxid string, xmlData string) (IdDetailParams, error) {
	// 调试输出
	//fmt.Println("Received XML Data:", xmlData)

	idRegex := regexp.MustCompile(`<id>\s*(\d+)\s*</id>`)
	usernameRegex := regexp.MustCompile(`<username>\s*(.*?)\s*</username>`)

	idMatches := idRegex.FindStringSubmatch(xmlData)
	usernameMatches := usernameRegex.FindStringSubmatch(xmlData)

	//fmt.Println("ID Matches:", idMatches)
	//fmt.Println("Username Matches:", usernameMatches)

	if len(idMatches) < 2 || len(usernameMatches) < 2 {
		return IdDetailParams{}, fmt.Errorf("无法提取字段")
	}

	var id uint64
	_, err := fmt.Sscan(idMatches[1], &id)
	if err != nil {
		return IdDetailParams{}, fmt.Errorf("id 解析失败：%v", err)
	}

	towxid := usernameMatches[1]

	return IdDetailParams{
		Wxid:   wxid,
		Towxid: towxid,
		Id:     id,
	}, nil
}

func GetCommnet(wxid string, xmlData string) models.ResponseResult {
	// 解析 XML 数据
	param, err := ParseXMLData(wxid, xmlData)
	if err != nil {
		return models.ResponseResult{
			Code:    -8,
			Success: false,
			Message: err.Error(),
			Data:    nil,
		}
	}

	D, err := comm.GetLoginata(wxid, nil)
	if err != nil || D == nil || D.Wxid == "" {
		errorMsg := fmt.Sprintf("异常：%v [%v]", "未找到登录信息", wxid)
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

	req := &mm.SnsObjectDetailRequest{
		BaseRequest: &mm.BaseRequest{
			SessionKey:    D.Sessionkey,
			Uin:           proto.Uint32(D.Uin),
			DeviceId:      D.Deviceid_byte,
			ClientVersion: proto.Int32(369558056),
			DeviceType:    []byte(D.DeviceType),
			Scene:         proto.Uint32(0),
		},
		GroupDetail: proto.Uint32(0),
		Id:          proto.Uint64(param.Id),
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

	// 发包
	protobufdata, _, errtype, err := comm.SendRequest(comm.SendPostData{
		Ip:     D.Mmtlsip,
		Host:   D.ShortHost,
		Cgiurl: "/cgi-bin/micromsg-bin/mmsnsobjectdetail",
		Proxy:  D.Proxy,
		PackData: Algorithm.PackData{
			Reqdata:          reqdata,
			Cgi:              210,
			Uin:              D.Uin,
			Cookie:           D.Cooike,
			Sessionkey:       D.Sessionkey,
			EncryptType:      5,
			Loginecdhkey:     D.RsaPublicKey,
			Clientsessionkey: D.Clientsessionkey,
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

	// 解包
	Response := mm.SnsObjectDetailResponse{}
	err = proto.Unmarshal(protobufdata, &Response)
	if err != nil {
		return models.ResponseResult{
			Code:    -8,
			Success: false,
			Message: fmt.Sprintf("反序列化失败：%v", err.Error()),
			Data:    nil,
		}
	}

	// 提取 CommentUserList
	commentUserList := Response.Object.CommentUserList
	if len(commentUserList) == 0 {
		return models.ResponseResult{
			Code:    0,
			Success: true,
			Message: "成功，但没有评论用户",
			Data:    nil,
			ID:      param.Id,
		}
	}

	// 处理 CommentUserList
	simplifiedList := make([]map[string]interface{}, 0, len(commentUserList))
	for idx, comment := range commentUserList {
		// 处理 CreateTime，确保解引用
		var createTimeUnix int64
		if comment.CreateTime != nil {
			createTimeUnix = int64(*comment.CreateTime)
		} else {
			createTimeUnix = time.Now().Unix() // 或者设置为其他默认值
		}

		// 转换 CreateTime 为北京时间
		createTime := time.Unix(createTimeUnix, 0).In(time.FixedZone("CST", 8*3600)) // 北京时间
		simplifiedList = append(simplifiedList, map[string]interface{}{
			"Id":         idx + 1, // 添加递增的 id，从 1 开始
			"Username":   comment.Username,
			"Nickname":   comment.Nickname,
			"Content":    comment.Content, // 保留 Content
			"toUsername": comment.ReplyUsername,
			"CreateTime": createTime.Format("2006-01-02 15:04:05"), // 格式化为字符串
		})
	}

	// 返回新的结果，只包含简化后的 CommentUserList
	return models.ResponseResult{
		Code:    0,
		Success: true,
		Message: "获取评论成功",
		ID:      param.Id,
		Data:    simplifiedList,
	}
}
