package wxcore

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
	"wechatdll/comm"
	"wechatdll/srv"
	"wechatdll/srv/wxface"
)

// WXConnectMgr 微信链接管理器
type WXConnectMgr struct {
	wxConnectMap map[string]wxface.IWXConnect
	wxConnLock   sync.RWMutex
}

// Add 添加链接
func (wm *WXConnectMgr) Add(wxConnect wxface.IWXConnect) {
	wm.wxConnLock.Lock()
	defer wm.wxConnLock.Unlock()
	wm.wxConnectMap[wxConnect.GetWXAccount().GetUserInfo().Wxid] = wxConnect
	go wm.ShowConnectInfo()
}

// GetWXConnectByWXID 根据WXID获取微信链接
func (wm *WXConnectMgr) GetWXConnectByWXID(wxid string) wxface.IWXConnect {
	wm.wxConnLock.RLock()
	defer wm.wxConnLock.RUnlock()
	for _, wxConn := range wm.wxConnectMap {
		tmpUserInfo := wxConn.GetWXAccount().GetUserInfo()
		if tmpUserInfo == nil || strings.Compare(tmpUserInfo.Wxid, wxid) != 0 {
			continue
		}
		return wxConn
	}
	return nil
}

func (wm *WXConnectMgr) ShowConnectInfo() string {
	totalNum := len(wm.wxConnectMap)
	showText := time.Now().Format("2006-01-02 15:04:05")
	showText = showText + " 总链接数量: " + strconv.Itoa(totalNum)
	fmt.Println(showText)
	return showText
}

// Remove 删除连接
func (wm *WXConnectMgr) Remove(wxconn wxface.IWXConnect) {
	wm.wxConnLock.Lock()
	currentUserInfo := wxconn.GetWXAccount().GetUserInfo()
	delete(wm.wxConnectMap, currentUserInfo.Wxid)
	currentUserInfo = nil
	wm.wxConnLock.Unlock()
	wm.ShowConnectInfo()
}

// ClearWXConn 删除并停止所有链接
func (wm *WXConnectMgr) ClearWXConn() {
	wm.wxConnLock.Lock()
	for uuid, wxConn := range wm.wxConnectMap {
		wxConn.Stop()
		delete(wm.wxConnectMap, uuid)
	}
	wm.wxConnLock.Unlock()
	wm.ShowConnectInfo()
}

var wxConnectMgr *WXConnectMgr = &WXConnectMgr{
	wxConnectMap: make(map[string]wxface.IWXConnect),
}

// 获取 WXConnectMgr 对象
func GetWXConnectMgr() *WXConnectMgr {
	if wxConnectMgr == nil {
		wxConnectMgr = &WXConnectMgr{
			wxConnectMap: make(map[string]wxface.IWXConnect),
		}
	}
	return wxConnectMgr
}

// 初始化自动心跳
func (wm *WXConnectMgr) InitAutoHeartBeat() {
	for key, logs := range comm.GetAutoHeartBeatList() {
		wxid := strings.TrimPrefix(key, "AutoHeartBeatList:")
		if wxid == "" {
			continue
		}
		if len(logs) == 0 || !strings.Contains(logs[0], "下次刷新时间") {
			continue
		}

		wXConnect := wm.GetWXConnectByWXID(wxid)
		if wXConnect == nil {
			userInfo, err := comm.GetLoginata(wxid, nil)
			if err != nil || userInfo == nil || userInfo.Wxid == "" {
				fmt.Printf("[online_guard] wxid=%s mismatch=Wxid\n", wxid)
				comm.AutoHeartBeatListClear(wxid)
				continue
			}
			if err := comm.ValidateCarOnlineProfile(wxid, userInfo); err != nil {
				fmt.Printf("[online_guard] wxid=%s mismatch=%s\n", wxid, err.Error())
				comm.AutoHeartBeatListClear(wxid)
				continue
			}
			wxAccount := srv.NewWXAccount(userInfo)
			wXConnect = NewWXConnect(wxConnectMgr, wxAccount)
		} else {
			userInfo := wXConnect.GetWXAccount().GetUserInfo()
			if err := comm.ValidateCarOnlineProfile(wxid, userInfo); err != nil {
				fmt.Printf("[online_guard] wxid=%s mismatch=%s\n", wxid, err.Error())
				comm.AutoHeartBeatListClear(wxid)
				wXConnect.Stop()
				continue
			}
		}

		if err := wXConnect.Start(); err != nil {
			continue
		}
		go wXConnect.SendHeartBeat()
	}
}
