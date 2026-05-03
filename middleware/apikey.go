package middleware

import (
	"encoding/json"
	"strings"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/context"
	"wechatdll/comm"
)

// APIKeyGate 小范围出售：合法 Key 放在 Redis SET 中（SADD），请求头 X-API-Key 或 Authorization: Bearer <key>。
// app.conf：apikeyenforce=true 时生效；apikeysrediskey 指定 SET 名，默认 wxapi:api:keys。
func apiKeyEnforceOn() bool {
	s := strings.TrimSpace(strings.ToLower(beego.AppConfig.String("apikeyenforce")))
	return s == "true" || s == "1" || s == "on" || s == "yes"
}

func APIKeyGate(ctx *context.Context) {
	if !strings.HasPrefix(ctx.Request.URL.Path, "/api/") {
		return
	}
	if strings.ToUpper(ctx.Input.Method()) == "OPTIONS" {
		return
	}
	if !apiKeyEnforceOn() {
		return
	}
	if comm.RedisClient == nil {
		writeAPIKeyDeny(ctx, "Redis 未初始化，无法校验 API Key")
		return
	}
	redisKey := strings.TrimSpace(beego.AppConfig.String("apikeysrediskey"))
	if redisKey == "" {
		redisKey = "wxapi:api:keys"
	}
	token := strings.TrimSpace(ctx.Input.Header("X-API-Key"))
	if token == "" {
		auth := ctx.Input.Header("Authorization")
		if len(auth) > 7 && strings.EqualFold(auth[:7], "bearer ") {
			token = strings.TrimSpace(auth[7:])
		}
	}
	if token == "" {
		writeAPIKeyDeny(ctx, "缺少 API Key：请求头 X-API-Key 或 Authorization: Bearer <key>")
		return
	}
	ok, err := comm.RedisClient.SIsMember(redisKey, token).Result()
	if err != nil {
		writeAPIKeyDeny(ctx, "API Key 校验异常: "+err.Error())
		return
	}
	if !ok {
		writeAPIKeyDeny(ctx, "无效的 API Key")
		return
	}
}

func writeAPIKeyDeny(ctx *context.Context, msg string) {
	ctx.Output.Header("Content-Type", "application/json; charset=utf-8")
	ctx.ResponseWriter.WriteHeader(401)
	_ = json.NewEncoder(ctx.ResponseWriter).Encode(map[string]interface{}{
		"Code":    -401,
		"Success": false,
		"Message": msg,
		"Data":    nil,
	})
	ctx.Abort(401, "API Key")
}
