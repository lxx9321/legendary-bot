package Msg

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math/big"
	"sort"
	"strings"
	"time"
	"wechatdll/Cilent/mm"
	"wechatdll/comm"

	"github.com/astaxie/beego"
)

const (
	redisOwnerKeyFmt     = "wxapi:ctl:owner:%s"
	redisDelegatesFmt    = "wxapi:ctl:delegates:%s"
	redisInviteFmt       = "wxapi:ctl:invite:%s:%s"
	redisSeenFmt         = "wxapi:ctl:seen:%s:%d"
	redisAuditFmt        = "wxapi:ctl:audit:%s"
	seenTTL              = 48 * time.Hour
	auditMax             = 200
	defaultInviteTTLSecs = 600
)

func cmdChatEnabled() bool {
	v, err := beego.AppConfig.Bool("cmdchat_enabled")
	return err == nil && v
}

// CmdChatEnabled 供长连/心跳等包判断：开启时即使 msgpush=false 也会拉 Msg.Sync 以处理微信内指令。
func CmdChatEnabled() bool {
	return cmdChatEnabled()
}

// CmdChatSyncDrainMax NewSync 分批返回时最多追加同步轮数（Continue!=0）。<=0 或未配置时用 12。
func CmdChatSyncDrainMax() int {
	v, err := beego.AppConfig.Int("cmdchat_sync_drain_max")
	if err != nil || v <= 0 {
		return 12
	}
	return v
}

// cmdChatPrefixes 支持多个触发前缀（英文逗号或中文逗号分隔），长匹配优先。默认 #、英文句号、中文句号。
func cmdChatPrefixes() []string {
	raw := strings.TrimSpace(beego.AppConfig.String("cmdchat_prefix"))
	if raw == "" {
		return []string{"。", ".", "#"}
	}
	raw = strings.ReplaceAll(raw, "，", ",")
	var out []string
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return []string{"。", ".", "#"}
	}
	sort.Slice(out, func(i, j int) bool { return len(out[i]) > len(out[j]) })
	return out
}

func cmdChatStripPrefixes(body string) (rest string, ok bool) {
	for _, px := range cmdChatPrefixes() {
		if strings.HasPrefix(body, px) {
			return strings.TrimSpace(strings.TrimPrefix(body, px)), true
		}
	}
	return "", false
}

func cmdChatPrefixHint() string {
	ps := cmdChatPrefixes()
	if len(ps) == 0 {
		return "#"
	}
	return strings.Join(ps, " 或 ")
}

// cmdChatShortPrefix 用于文案里举例（取最短前缀，方便输入）。
func cmdChatShortPrefix() string {
	ps := cmdChatPrefixes()
	if len(ps) == 0 {
		return "#"
	}
	return ps[len(ps)-1]
}

func cmdChatSessions() map[string]bool {
	raw := strings.TrimSpace(beego.AppConfig.String("cmdchat_sessions"))
	if raw == "" {
		return map[string]bool{"pm": true}
	}
	m := make(map[string]bool)
	for _, p := range strings.Split(raw, ",") {
		k := strings.TrimSpace(strings.ToLower(p))
		if k != "" {
			m[k] = true
		}
	}
	if len(m) == 0 {
		return map[string]bool{"pm": true}
	}
	return m
}

func cmdChatOwnerCode() string {
	v := strings.TrimSpace(beego.AppConfig.String("cmdchat_owner_code"))
	if v == "" {
		return "888"
	}
	return strings.ToUpper(v)
}

func isGroupChat(from, to string) bool {
	return strings.Contains(from, "@chatroom") || strings.Contains(to, "@chatroom")
}

func isFilehelperSession(from, to string) bool {
	f := strings.ToLower(from)
	t := strings.ToLower(to)
	return strings.Contains(f, "filehelper") || strings.Contains(t, "filehelper")
}

func isPMSession(robot, from, to string) bool {
	if from == robot && to != robot && !strings.Contains(to, "@chatroom") {
		return true
	}
	if to == robot && from != robot && !strings.Contains(from, "@chatroom") {
		return true
	}
	return false
}

func cmdChatSessionAllowed(robot, from, to string) bool {
	if isGroupChat(from, to) {
		return false
	}
	s := cmdChatSessions()
	ok := false
	if s["filehelper"] && isFilehelperSession(from, to) {
		ok = true
	}
	if s["pm"] && isPMSession(robot, from, to) {
		ok = true
	}
	return ok
}

func redisOwnerKey(robot string) string {
	return fmt.Sprintf(redisOwnerKeyFmt, robot)
}

func redisDelegatesKey(robot string) string {
	return fmt.Sprintf(redisDelegatesFmt, robot)
}

func getOwner(robot string) string {
	if comm.RedisClient == nil {
		return ""
	}
	v, err := comm.RedisClient.Get(redisOwnerKey(robot)).Result()
	if err != nil || v == "" {
		return ""
	}
	return v
}

func setOwner(robot, owner string) error {
	return comm.RedisClient.Set(redisOwnerKey(robot), owner, 0).Err()
}

func isDelegate(robot, wxid string) bool {
	if comm.RedisClient == nil {
		return false
	}
	ok, err := comm.RedisClient.SIsMember(redisDelegatesKey(robot), wxid).Result()
	return err == nil && ok
}

func addDelegate(robot, wxid string) error {
	return comm.RedisClient.SAdd(redisDelegatesKey(robot), wxid).Err()
}

func removeDelegate(robot, wxid string) error {
	return comm.RedisClient.SRem(redisDelegatesKey(robot), wxid).Err()
}

func listDelegates(robot string) []string {
	if comm.RedisClient == nil {
		return nil
	}
	return comm.RedisClient.SMembers(redisDelegatesKey(robot)).Val()
}

func audit(robot, from, line, result string) {
	if comm.RedisClient == nil {
		return
	}
	b, _ := json.Marshal(map[string]string{
		"ts":     time.Now().Format(time.RFC3339),
		"from":   from,
		"line":   line,
		"result": result,
	})
	key := fmt.Sprintf(redisAuditFmt, robot)
	_ = comm.RedisClient.LPush(key, string(b)).Err()
	_ = comm.RedisClient.LTrim(key, 0, auditMax-1).Err()
}

func fnvShort(s string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return fmt.Sprintf("%08x", h.Sum32())
}

// seenMark 首次见到的消息返回 true，用于去重防重复回执。
func seenMark(robot string, m *mm.AddMsg) bool {
	if comm.RedisClient == nil {
		return false
	}
	var key string
	if m.GetNewMsgId() != 0 {
		key = fmt.Sprintf(redisSeenFmt, robot, m.GetNewMsgId())
	} else {
		from := ""
		if m.FromUserName != nil {
			from = m.FromUserName.GetString_()
		}
		to := ""
		if m.ToUserName != nil {
			to = m.ToUserName.GetString_()
		}
		body := addMsgBodyText(m)
		key = fmt.Sprintf("wxapi:ctl:seenf:%s:%d:%d:%s", robot, m.GetMsgId(), m.GetCreateTime(), fnvShort(from+"|"+to+"|"+body))
	}
	ok, err := comm.RedisClient.SetNX(key, "1", seenTTL).Result()
	if err != nil {
		fmt.Printf("[cmdchat] seenMark Redis 错误 robot=%s key=%s err=%v\n", robot, key, err)
		return false
	}
	return ok
}

func reply(robot, toWxid, text string) {
	if toWxid == "" || text == "" {
		return
	}
	res := SendNewMsg(SendNewMsgParam{
		Wxid:    robot,
		ToWxid:  toWxid,
		Content: text,
		Type:    1,
	})
	if !res.Success {
		fmt.Printf("[cmdchat] 回执发送失败 robot=%s to=%s code=%d msg=%s\n", robot, toWxid, res.Code, res.Message)
	}
}

// role: owner=已认领的主人 wxid；self=尚无主人时由机器人号自己在助手等会话发令；delegate=副控；guest=其它。
func role(robot, sender string) string {
	o := getOwner(robot)
	if sender == robot {
		if o != "" && o == robot {
			return "owner"
		}
		if o != "" && o != robot {
			// 主人是其它号时，本机登录号发出的消息不享有主人权限（主人请在私聊机器人中操作）
			return "guest"
		}
		return "self"
	}
	if o == "" {
		return "guest"
	}
	if sender == o {
		return "owner"
	}
	if isDelegate(robot, sender) {
		return "delegate"
	}
	return "guest"
}

func genInviteCode() string {
	const chars = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"
	b := make([]byte, 6)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			b[i] = chars[time.Now().UnixNano()%int64(len(chars))]
			continue
		}
		b[i] = chars[n.Int64()]
	}
	return string(b)
}

// normalizeCmdToken 将中文或英文口令统一为内部英文 key。
func normalizeCmdToken(token string) string {
	t := strings.TrimSpace(token)
	low := strings.ToLower(t)
	switch low {
	case "ping", "help", "status", "claim", "invite", "bind", "unbind", "kick":
		return low
	case "pong":
		return "ping"
	}
	switch t {
	case "在吗", "测试", "连通", "在线吗", "卡了吗":
		return "ping"
	case "帮助", "说明", "？", "?":
		return "help"
	case "状态":
		return "status"
	case "认领", "主人":
		return "claim"
	case "邀请", "邀请码":
		return "invite"
	case "绑定":
		return "bind"
	case "解绑":
		return "unbind"
	case "踢出", "移除":
		return "kick"
	}
	return low
}

func addMsgBodyText(m *mm.AddMsg) string {
	body := ""
	if m.Content != nil {
		body = strings.TrimSpace(m.Content.GetString_())
	}
	if body == "" {
		body = strings.TrimSpace(m.GetPushContent())
	}
	return body
}

func parseCmdLine(body string) (rest string, ok bool) {
	body = strings.TrimSpace(body)
	if body == "" {
		return "", false
	}
	if rest, ok := cmdChatStripPrefixes(body); ok && rest != "" {
		return rest, true
	}
	lower := strings.ToLower(body)
	noSpaceLower := strings.ReplaceAll(lower, " ", "")
	switch noSpaceLower {
	case "ping", "test", "help", "status", "claim", "owner", "bindowner", "setowner":
		return body, true
	}
	switch strings.ReplaceAll(body, " ", "") {
	case "在吗", "测试", "帮助", "状态", "认领", "主人", "绑定主人", "设主人", "设置主人", "认领主人":
		return body, true
	}
	if strings.HasPrefix(lower, "bind#") {
		code := strings.TrimSpace(body[len("bind#"):])
		if code == "" {
			return "bind", true
		}
		return "bind " + code, true
	}
	if strings.HasPrefix(body, "绑定#") {
		code := strings.TrimSpace(strings.TrimPrefix(body, "绑定#"))
		if code == "" {
			return "绑定", true
		}
		return "绑定 " + code, true
	}
	if strings.HasPrefix(lower, "bind ") {
		code := strings.TrimSpace(body[len("bind "):])
		if code == "" {
			return "bind", true
		}
		return "bind " + code, true
	}
	if strings.HasPrefix(body, "绑定 ") {
		code := strings.TrimSpace(strings.TrimPrefix(body, "绑定 "))
		if code == "" {
			return "绑定", true
		}
		return "绑定 " + code, true
	}
	return "", false
}

// ProcessCmdChatAddMsgs 在 Sync 解析出 AddMsg 后调用（建议异步）。实现第一期：指令、主人/副控、邀请码、审计、微信内回执。
func ProcessCmdChatAddMsgs(robotWxid string, addMsgs []mm.AddMsg) {
	if !cmdChatEnabled() || comm.RedisClient == nil || robotWxid == "" {
		return
	}
	for i := range addMsgs {
		m := &addMsgs[i]
		if m.GetMsgType() != 1 {
			continue
		}
		from := ""
		if m.FromUserName != nil {
			from = m.FromUserName.GetString_()
		}
		to := ""
		if m.ToUserName != nil {
			to = m.ToUserName.GetString_()
		}
		body := addMsgBodyText(m)
		line, ok := parseCmdLine(body)
		if !ok || line == "" {
			continue
		}
		if !cmdChatSessionAllowed(robotWxid, from, to) {
			fmt.Printf("[cmdchat] skip session robot=%s from=%s to=%s body=%q line=%q\n", robotWxid, from, to, body, line)
			continue
		}
		// 须在确认是指令且会话合法后再去重：若先 seenMark，首轮 Sync 正文为空会占坑，后续带正文的同一条 AddMsg 会被永久跳过。
		if !seenMark(robotWxid, m) {
			fmt.Printf("[cmdchat] skip duplicate robot=%s from=%s to=%s msgid=%d newmsgid=%d body=%q\n", robotWxid, from, to, m.GetMsgId(), m.GetNewMsgId(), body)
			continue
		}
		replyTo := from
		if from == robotWxid {
			replyTo = to
		}
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
		cmd := normalizeCmdToken(parts[0])
		args := parts[1:]

		r := role(robotWxid, from)
		out := dispatchCmd(robotWxid, from, r, cmd, args)
		fmt.Printf("[cmdchat] robot=%s from=%s to=%s role=%s body=%q line=%q cmd=%s args=%v\n", robotWxid, from, to, r, body, line, cmd, args)
		audit(robotWxid, from, line, out)
		if out != "" {
			reply(robotWxid, replyTo, out)
		}
	}
}

func dispatchCmd(robot, sender, r, cmd string, args []string) string {
	switch cmd {
	case "ping":
		return "收到，长连/指令通道正常。"
	case "help":
		return helpText(r)
	case "status":
		return statusText(robot, r, sender)
	case "claim":
		return cmdClaim(robot, sender, r)
	case "invite":
		return cmdInvite(robot, sender, r)
	case "bind":
		return cmdBind(robot, sender, r, args)
	case "unbind":
		return cmdUnbind(robot, sender, r)
	case "kick":
		return cmdKick(robot, sender, r, args)
	default:
		return "未识别的口令。发「" + cmdChatShortPrefix() + "帮助」查看列表（也可用前缀：" + cmdChatPrefixHint() + "）。"
	}
}

func helpText(r string) string {
	h := cmdChatPrefixHint()
	sp := cmdChatShortPrefix()
	var b strings.Builder
	b.WriteString("【控制指令】触发前缀：" + h + "\n")
	b.WriteString(sp + "在吗 — 测试是否在线\n")
	b.WriteString(sp + "帮助 — 本说明\n")
	b.WriteString(sp + "状态 — 主人/副控概况\n")
	b.WriteString(sp + "认领 — 首次设主人（仅一次）\n")
	switch r {
	case "owner":
		b.WriteString(sp + "邀请 — 生成副控绑定码\n")
		b.WriteString(sp + "踢出 <对方wxid> — 移除副控\n")
	}
	if r == "delegate" || r == "guest" {
		b.WriteString(sp + "绑定 <码> — 成为副控\n")
	}
	if r == "delegate" {
		b.WriteString(sp + "解绑 — 退出副控\n")
	}
	return b.String()
}

func statusText(robot, r, sender string) string {
	o := getOwner(robot)
	ds := listDelegates(robot)
	sp := cmdChatShortPrefix()
	var b strings.Builder
	b.WriteString("机器人：" + robot + "\n")
	b.WriteString("你的身份：" + r + "（发送者 " + sender + "）\n")
	if o != "" {
		b.WriteString("主人：" + o + "\n")
	} else {
		b.WriteString("主人：未设置（发 " + sp + "认领）\n")
	}
	b.WriteString(fmt.Sprintf("副控数量：%d\n", len(ds)))
	b.WriteString("允许会话：私聊机器人、文件传输助手（配置项 cmdchat_sessions）\n")
	b.WriteString("自动收款/抢包/记账将在后续版本接入。")
	return b.String()
}

func cmdClaim(robot, sender, r string) string {
	if getOwner(robot) != "" {
		return "已有主人，无法再次认领。"
	}
	if r == "self" {
		_ = setOwner(robot, robot)
		return "已认领：当前登录号为「主人」（自托管）。副控请主人发「" + cmdChatShortPrefix() + "邀请」生成码。"
	}
	if r == "guest" {
		_ = setOwner(robot, sender)
		return "已认领：你已成为「主人」。副控请私聊本号发送「" + cmdChatShortPrefix() + "绑定 邀请码」。"
	}
	return "当前身份无法执行认领。"
}

func cmdInvite(robot, sender, r string) string {
	if r != "owner" {
		return "仅主人可发「邀请」。若主人是其它微信号，请在私聊本机器人里操作。"
	}
	code := genInviteCode()
	if getOwner(robot) == "" && code == cmdChatOwnerCode() {
		if sender == robot {
			return "不能使用机器人号自身绑定。"
		}
		if err := setOwner(robot, sender); err != nil {
			return "绑定失败：" + err.Error()
		}
		return "绑定成功：你已成为主人。"
	}
	key := fmt.Sprintf(redisInviteFmt, robot, code)
	secs := defaultInviteTTLSecs
	if v, err := beego.AppConfig.Int("cmdchat_invite_ttl_secs"); err == nil && v > 0 {
		secs = v
	}
	if err := comm.RedisClient.Set(key, "1", time.Duration(secs)*time.Second).Err(); err != nil {
		return "生成失败，请稍后再试。"
	}
	return fmt.Sprintf("邀请码（%d 秒内有效）：%s\n请让对方在「私聊本机器人」里发送：%s绑定 %s", secs, code, cmdChatShortPrefix(), code)
}

func cmdBind(robot, sender, r string, args []string) string {
	if len(args) < 1 {
		return "用法：" + cmdChatShortPrefix() + "绑定 <邀请码>"
	}
	code := strings.ToUpper(strings.TrimSpace(strings.Join(args, "")))
	if len(code) < 4 || len(code) > 16 {
		return "邀请码格式不正确。"
	}
	key := fmt.Sprintf(redisInviteFmt, robot, code)
	n, err := comm.RedisClient.Exists(key).Result()
	if err != nil || n == 0 {
		return "邀请码无效或已过期，请让主人重新发「" + cmdChatShortPrefix() + "邀请」。"
	}
	if sender == robot {
		return "不能使用机器人号自身绑定。"
	}
	if getOwner(robot) == sender {
		return "你已是主人，无需绑定副控。"
	}
	_ = comm.RedisClient.Del(key).Err()
	if err := addDelegate(robot, sender); err != nil {
		return "绑定失败：" + err.Error()
	}
	return "绑定成功：你已成为副控。发「" + cmdChatShortPrefix() + "帮助」查看可用指令。"
}

func cmdUnbind(robot, sender, r string) string {
	if r != "delegate" {
		return "仅副控可发「解绑」。"
	}
	if err := removeDelegate(robot, sender); err != nil {
		return "解除失败：" + err.Error()
	}
	return "已解除副控身份。"
}

func cmdKick(robot, sender, r string, args []string) string {
	if r != "owner" {
		return "仅主人可「踢出」副控。"
	}
	if len(args) < 1 {
		return "用法：" + cmdChatShortPrefix() + "踢出 <对方wxid>"
	}
	target := strings.TrimSpace(args[0])
	if target == "" || target == robot {
		return "wxid 无效。"
	}
	if err := removeDelegate(robot, target); err != nil {
		return "操作失败：" + err.Error()
	}
	return "已移除副控：" + target
}

