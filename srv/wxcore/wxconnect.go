package wxcore

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"
	"wechatdll/Cilent/mm"
	"wechatdll/comm"
	"wechatdll/srv"
	"wechatdll/srv/wxface"
)

// WXConnect 微信链接
type WXConnect struct {
	wXConnectMgr *WXConnectMgr
	// 请求调用器
	wxModels wxface.IWXModels
	// 微信账号信息
	WxAccount *srv.WXAccount
	// 心跳定时器
	HeartBeatTimer *time.Timer
	// 刷新 token 定时器(二次登录)
	RefreshTokenTimer *time.Timer
	// 断开链接
	ExitFlagChan chan bool
	//
	isConnected bool
	// 启动时间，避免重复启动
	startTime int64
	// 互斥锁
	mu sync.Mutex
}

// GetWXAccount 获取微信帐号信息
func (wxconn *WXConnect) GetWXAccount() *srv.WXAccount {
	return wxconn.WxAccount
}

// NewWXConnect 新的微信连接
func NewWXConnect(wXConnectMgr *WXConnectMgr, wxAccount *srv.WXAccount) wxface.IWXConnect {
	wxconn := &WXConnect{
		wXConnectMgr: wXConnectMgr,
		WxAccount:    wxAccount,
		ExitFlagChan: make(chan bool, 1),
		isConnected:  false,
	}
	wxconn.wxModels = NewWXModels(wxconn)
	return wxconn
}

// startLongWriter 开启长链接发送数据
func (wxconn *WXConnect) startLongWriter() {
	startTime := wxconn.startTime
	for { // 心跳包
		select {
		case <-wxconn.HeartBeatTimer.C:
			if startTime != wxconn.startTime {
				return
			}
			// 发送心跳包
			_ = wxconn.SendHeartBeat()
			continue

		case <-wxconn.ExitFlagChan:
			return
		}
	}
}

// 发送心跳
//
//	func (wxconn *WXConnect) SendHeartBeat() error {
//		userInfo := wxconn.WxAccount.GetUserInfo()
//		var BaseRes *mm.HeartBeatResponse = &mm.HeartBeatResponse{}
//		// 判断 linux 和 win
//		switch runtime.GOOS {
//		case "linux":
//			_, BaseRes = wxconn.wxModels.LoginHeartBeatLong(wxconn.WxAccount.GetUserInfo().Wxid)
//		default:
//			_, BaseRes = wxconn.wxModels.LoginHeartBeat(wxconn.WxAccount.GetUserInfo().Wxid)
//		}
//
//		NextTime := BaseRes.GetNextTime()
//		if NextTime < 100 {
//			NextTime = 175
//		}
//		wxconn.SendHeartBeatWaitingSeconds(NextTime)
//		timeStr := time.Now().Add(time.Duration(NextTime) * time.Second).Format("2006-01-02 15:04:05")
//
//		if BaseRes == nil || BaseRes.GetBaseResponse().GetRet() != 0 {
//			timeStr := time.Now().Format("2006-01-02 15:04:05")
//			comm.AutoHeartBeatListAdd(userInfo.Wxid, fmt.Sprintf("[%s],[%s] 发送心跳失败，不暂停自动心跳 保持下一次 %s", userInfo.Wxid, userInfo.GetNickName(), timeStr))
//			fmt.Println(fmt.Sprintf("[%s],[%s] 发送心跳失败，不暂停自动心跳 %s", userInfo.Wxid, userInfo.GetNickName(), timeStr))
//			//wxconn.Stop()
//			//return errors.New("发送心跳失败")
//		} else {
//			comm.AutoHeartBeatListAdd(userInfo.Wxid, fmt.Sprintf("[%s],[%s] 发送心跳成功，下次刷新时间：%s", userInfo.Wxid, userInfo.GetNickName(), timeStr))
//			fmt.Println(fmt.Sprintf("[%s],[%s] 发送心跳成功，下次刷新时间：%s", userInfo.Wxid, userInfo.GetNickName(), timeStr))
//		}
//
//		return nil
//	}
func (wxconn *WXConnect) SendHeartBeat() error {
	userInfo := wxconn.WxAccount.GetUserInfo()
	var BaseRes *mm.HeartBeatResponse

	fmt.Println(fmt.Sprintf("[%s],[%s] 发送==================", userInfo.Wxid, userInfo.GetNickName()))

	// 最大重试次数
	const maxRetries = 3
	// 每次重试间隔时间（秒）
	const retryInterval = 175 * time.Second

	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			time.Sleep(retryInterval)
			fmt.Sprintf("[%s] 正在第 %d 次重试发送心跳...", userInfo.Wxid, i)
		}

		D, err := comm.GetLoginata(userInfo.Wxid, nil)
		if err != nil || D == nil || D.Wxid == "" {
			fmt.Printf("[online_guard] wxid=%s mismatch=Wxid\n", userInfo.Wxid)
			comm.AutoHeartBeatListClear(userInfo.Wxid)
			wxconn.Stop()
			return errors.New("在线设备档案校验失败")
		}
		if err := comm.ValidateCarOnlineProfile(userInfo.Wxid, D); err != nil {
			fmt.Printf("[online_guard] wxid=%s mismatch=%s\n", userInfo.Wxid, err.Error())
			comm.AutoHeartBeatListClear(userInfo.Wxid)
			wxconn.Stop()
			return errors.New("在线设备档案校验失败")
		}

		switch runtime.GOOS {
		case "linux":
			_, BaseRes = wxconn.wxModels.LoginHeartBeatLong(userInfo.Wxid)
		default:
			_, BaseRes = wxconn.wxModels.LoginHeartBeat(userInfo.Wxid)
		}

		if BaseRes != nil && BaseRes.GetBaseResponse().GetRet() == 0 {
			// 心跳成功
			NextTime := BaseRes.GetNextTime()
			if NextTime < 100 {
				NextTime = 175
			}
			timeStr := time.Now().Add(time.Duration(NextTime) * time.Second).Format("2006-01-02 15:04:05")
			msg := fmt.Sprintf("[%s],[%s] 发送心跳成功，下次刷新时间：%s", userInfo.Wxid, userInfo.GetNickName(), timeStr)

			comm.AutoHeartBeatListAdd(userInfo.Wxid, msg)
			fmt.Println(msg)
			wxconn.SendHeartBeatWaitingSeconds(NextTime)
			return nil
		} else {
			// 心跳失败
			fmt.Sprintf("[%s] 第 %d 次发送心跳失败", userInfo.Wxid, i+1)
			//wxconn.wxModels.LoginSecautoauth(userInfo.Wxid)
		}
	}

	// 所有重试都失败后不再自动二次登录，交给前端手动接口处理。
	timeStr := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf("[%s],[%s] 心跳多次失败，已停止连接，用户可能退出登录！ %s", userInfo.Wxid, userInfo.GetNickName(), timeStr)
	comm.AutoHeartBeatListAdd(userInfo.Wxid, msg)
	fmt.Println(msg)

	wxconn.Stop()
	return errors.New("心跳多次失败，已关闭连接，用户可能退出登录！")
}

// 发送二次登录
//
//	func (wxconn *WXConnect) RefreshToken(num int) error {
//		timeNowStr := time.Now().Format("2006-01-02 15:04:05")
//		temUserInfo := wxconn.WxAccount.GetUserInfo()
//		userInfo, err := comm.GetLoginata(temUserInfo.Wxid, nil)
//		if err != nil || userInfo == nil || userInfo.Wxid == "" {
//			fmt.Println("RefreshToken 获取用户信息失败", temUserInfo.Wxid)
//			comm.AutoHeartBeatListAdd(temUserInfo.Wxid, fmt.Sprintf("[%s],[%s] RefreshToken 获取用户信息失败，已暂停自动心跳 %s", temUserInfo.Wxid, temUserInfo.GetNickName(), timeNowStr))
//			return errors.New("获取用户信息失败")
//		}
//		// 获取上一次刷新 token 时间
//		lastRefreshTokenTime := userInfo.RefreshTokenDate
//		// 判断是否需要刷新 token
//		if lastRefreshTokenTime+1800 > time.Now().Unix() {
//			Minutes := (lastRefreshTokenTime + 3600 - time.Now().Unix()) / 60
//			if Minutes <= 1 {
//				Minutes = 1
//			}
//			wxconn.SendRefreshTokenWaitingMinutes(uint32(int(Minutes)))
//			timeStr := time.Now().Add(time.Minute * time.Duration(Minutes)).Format("2006-01-02 15:04:05")
//			comm.AutoHeartBeatListAdd(userInfo.Wxid, fmt.Sprintf("[%s],[%s] RefreshToken 自动二次登录已开启，下次刷新时间：%s", userInfo.Wxid, userInfo.GetNickName(), timeStr))
//			fmt.Println(fmt.Sprintf("[%s],[%s] RefreshToken 自动二次登录已开启，下次刷新时间：%s", userInfo.Wxid, userInfo.GetNickName(), timeStr))
//			return nil
//		}
//
//		_, res := wxconn.wxModels.LoginSecautoauth(userInfo.Wxid)
//		if res == nil {
//			fmt.Println("发送二次登录失败: ", userInfo.Wxid)
//			if num < 3 {
//				time.Sleep(time.Second * 10)
//				go wxconn.RefreshToken(num + 1)
//				return nil
//			}
//			//wxconn.Stop()
//			comm.AutoHeartBeatListAdd(userInfo.Wxid, fmt.Sprintf("[%s],[%s] res.Data == nil 发送二次登录失败，不暂停自动心跳 %s", userInfo.Wxid, userInfo.GetNickName(), timeNowStr))
//			//return errors.New("res.Data == nil 发送二次登录失败")
//		}
//		wxconn.SendRefreshTokenWaitingMinutes(60)
//		timeStr := time.Now().Add(time.Minute * 60).Format("2006-01-02 15:04:05")
//
//		if res.GetBaseResponse().GetRet() != 0 {
//			fmt.Println("发送二次登录失败 GetRet() != 0: ", userInfo.Wxid)
//			if num < 3 {
//				time.Sleep(time.Second * 10)
//				go wxconn.RefreshToken(num + 1)
//				return nil
//			}
//			//wxconn.Stop()
//			comm.AutoHeartBeatListAdd(userInfo.Wxid, fmt.Sprintf("[%s],[%s] res.GetBaseResponse().GetRet() != 0 发送二次登录失败，不暂停自动心跳 %s", userInfo.Wxid, userInfo.GetNickName(), timeNowStr))
//			//return errors.New("res.GetBaseResponse().GetRet() != 0 发送二次登录失败")
//		} else {
//			// 打印日志
//			comm.AutoHeartBeatListAdd(userInfo.Wxid, fmt.Sprintf("[%s],[%s] 二次登录成功，下次刷新时间：%s", userInfo.Wxid, userInfo.GetNickName(), timeStr))
//			fmt.Println(fmt.Sprintf("[%s],[%s] 二次登录成功，下次刷新时间：%s", userInfo.Wxid, userInfo.GetNickName(), timeStr))
//		}
//		return nil
//	}
//

// RefreshToken 发送一次手动二次登录请求，失败时只在当前调用内短间隔重试。
func (wxconn *WXConnect) RefreshToken(maxRetries int) error {
	const retryInterval = 10 * time.Second // 每次重试间隔

	timeNowStr := time.Now().Format("2006-01-02 15:04:05")
	temUserInfo := wxconn.WxAccount.GetUserInfo()

	// 获取用户信息
	userInfo, err := comm.GetLoginata(temUserInfo.Wxid, nil)
	if err != nil || userInfo == nil || userInfo.Wxid == "" {
		msg := fmt.Sprintf("[%s],[%s] RefreshToken 获取用户信息失败 %s", temUserInfo.Wxid, temUserInfo.GetNickName(), timeNowStr)
		fmt.Println(msg)
		comm.AutoHeartBeatListAdd(temUserInfo.Wxid, msg)
		return errors.New("获取用户信息失败")
	}

	// 判断是否需要刷新 token
	lastRefreshTokenTime := userInfo.RefreshTokenDate
	if lastRefreshTokenTime+1800 > time.Now().Unix() {
		msg := fmt.Sprintf("[%s],[%s] RefreshToken 距离上次刷新不足 30 分钟，跳过本次手动二次登录", userInfo.Wxid, userInfo.GetNickName())
		comm.AutoHeartBeatListAdd(userInfo.Wxid, msg)
		fmt.Println(msg)
		return nil
	}

	attempts := maxRetries
	if attempts <= 0 {
		attempts = 2
	}
	for attempt := 1; attempt <= attempts; attempt++ {
		_, res := wxconn.wxModels.LoginSecautoauth(userInfo.Wxid)

		if res == nil {
			msg := fmt.Sprintf("[%s],[%s] 第 %d 次发送二次登录失败", userInfo.Wxid, userInfo.GetNickName(), attempt)
			fmt.Println(msg)
			comm.AutoHeartBeatListAdd(userInfo.Wxid, msg)
		} else if res.GetBaseResponse().GetRet() != 0 {
			msg := fmt.Sprintf("[%s],[%s] 第 %d 次发送二次登录失败：retCode=%d", userInfo.Wxid, userInfo.GetNickName(), attempt, res.GetBaseResponse().GetRet())
			fmt.Println(msg)
			comm.AutoHeartBeatListAdd(userInfo.Wxid, msg)
		} else {
			// 成功
			msg := fmt.Sprintf("[%s],[%s] 手动二次登录成功", userInfo.Wxid, userInfo.GetNickName())
			comm.AutoHeartBeatListAdd(userInfo.Wxid, msg)
			fmt.Println(msg)
			return nil
		}

		// 如果不是最后一次尝试，则等待一段时间再重试
		if attempt < attempts {
			time.Sleep(retryInterval)
		}
	}

	// 所有重试都失败了，不关闭连接，交给前端手动处理。
	msg := fmt.Sprintf("[%s],[%s] 手动二次登录多次失败，未关闭连接", userInfo.Wxid, userInfo.GetNickName())
	fmt.Println(msg)
	comm.AutoHeartBeatListAdd(userInfo.Wxid, msg)

	return errors.New("二次登录多次失败，未关闭连接")
}

// Start 开启微信链接任务
func (wxconn *WXConnect) Start() error {
	wxconn.mu.Lock()
	defer wxconn.mu.Unlock()
	// 如果是链接状态
	if wxconn.isConnected {
		return nil
	}
	wxconn.isConnected = true

	userInfo := wxconn.WxAccount.GetUserInfo()
	// 判断微信信息是否为空
	if userInfo == nil {
		return errors.New("wxconn.Start() err: userInfo == nil")
	}
	// 重置启动时间
	wxconn.startTime = time.Now().Unix()
	wxconn.HeartBeatTimer = time.NewTimer(time.Second * 175)
	// 保留刷新 token 定时器字段，但启动后立即关闭，避免自动二次登录。
	wxconn.RefreshTokenTimer = time.NewTimer(time.Hour * 1)
	if !wxconn.RefreshTokenTimer.Stop() {
		select {
		case <-wxconn.RefreshTokenTimer.C:
		default:
		}
	}
	wxconn.SendHeartBeatWaitingSeconds(175)
	go wxconn.startLongWriter()
	return nil
}

// Stop 关闭链接
func (wxconn *WXConnect) Stop() {
	wxconn.mu.Lock()
	defer wxconn.mu.Unlock()
	// 重置启动时间
	wxconn.startTime = time.Now().Unix()
	// 断开链接
	wxconn.isConnected = false
	wxconn.ExitFlagChan <- true
	userInfo := wxconn.WxAccount.GetUserInfo()
	wxconn.wXConnectMgr.Remove(wxconn)
	// 立即过期
	wxconn.HeartBeatTimer.Reset(0)
	if wxconn.RefreshTokenTimer != nil {
		wxconn.RefreshTokenTimer.Stop()
	}
	timeStr := time.Now().Format("2006-01-02 15:04:05")
	fmt.Println(fmt.Sprintf("[%s],[%s] 退出！ %s", userInfo.Wxid, userInfo.GetNickName(), timeStr))
}

// SendHeartBeatWaitingSeconds 添加到微信心跳包队列
func (wxconn *WXConnect) SendHeartBeatWaitingSeconds(seconds uint32) {
	wxconn.HeartBeatTimer.Reset(time.Second * time.Duration(seconds))
}

// SendRefreshTokenWaitingMinutes 保留旧调用入口；当前不再安排自动二次登录。
func (wxconn *WXConnect) SendRefreshTokenWaitingMinutes(minutes uint32) {
}
