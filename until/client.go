package until

import "gopkg.in/chanxuehong/wechat.v2/mch/core"

var (
	WechatClient *core.Client
)

func init() {
	// you must init the client
	// WechatClient=core.NewClient()
}
