package router

import (
	"github.com/am-li/wechatpaydemo/controller/wechatpay"
	"github.com/astaxie/beego"
)

func init() {
	// wechat pay callback by wechat admin config
	beego.Router("/wechat/pay_callback", &wechatpay.WechatPayCallback{})

	beego.Router("/wechat/pay_result", &wechatpay.WechatPayResult{})
}
