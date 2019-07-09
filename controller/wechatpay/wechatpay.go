package wechatpay

import (
	"encoding/xml"
	"errors"
	"fmt"

	"github.com/am-li/wechatpaydemo/until"
	"github.com/astaxie/beego"

	"github.com/sirupsen/logrus"
	"gopkg.in/chanxuehong/wechat.v2/mch/core"
	"gopkg.in/chanxuehong/wechat.v2/mch/pay"
)

type (
	WechatPayCallback struct {
		beego.Controller
	}

	CallbackParams struct {
		AppId       string `xml:"appid"`
		Openid      string `xml:"openid"`
		MchId       string `xml:"mch_id"`
		IsSubscribe string `xml:"is_subscribe"`
		NonceStr    string `xml:"nonce_str"`
		ProductId   string `xml:"product_id"`
		Sign        string `xml:"sign"`
	}
	ReturnResult struct {
		XMLName    xml.Name `xml:"xml"`
		ReturnCode string   `xml:"return_code"` //YES SUCCESS/FAIL
		AppId      string   `xml:"appid"`       //YES
		MchId      string   `xml:"mch_id"`      //YES
		NonceStr   string   `xml:"nonce_str"`   //YES
		PrepayId   string   `xml:"prepay_id"`   //YES
		ResultCode string   `xml:"result_code"` //YES
		Sign       string   `xml:"sign"`        //YES
	}
)

//WechatPayCallback
func (this *WechatPayCallback) Post() {
	var callback CallbackParams
	wXNotifyResp := WXPayNotifyResp{
		ReturnCode: "FAIL",
	}
	body := this.Ctx.Input.CopyBody(99999)
	if body == nil {
		wXNotifyResp.ReturnMsg = "参数格式有误"
		this.Data["xml"] = wXNotifyResp
		this.ServeXML()
		return
	}
	err := xml.Unmarshal(body, &callback)
	if err != nil {
		logrus.Debugf("xml.Unmarshal:%+s", err)
		wXNotifyResp.ReturnMsg = "参数格式有误"
		this.Data["xml"] = wXNotifyResp
		this.ServeXML()
		return
	}
	logrus.Infof("微信回调，HTTP Body:%+v", callback)

	// gen Sign 签名校验.
	signParamsCallback := make(map[string]string)
	signParamsCallback["appid"] = callback.AppId
	signParamsCallback["openid"] = callback.Openid
	signParamsCallback["mch_id"] = callback.MchId
	signParamsCallback["is_subscribe"] = callback.IsSubscribe
	signParamsCallback["nonce_str"] = callback.NonceStr
	signParamsCallback["product_id"] = callback.ProductId

	if !WxpayVerifySign(signParamsCallback, callback.Sign) { //扫码回调
		logrus.Debugf("failed to verify Sign!")
		wXNotifyResp.ReturnMsg = "签名认证失败"
		this.Data["xml"] = wXNotifyResp
		this.ServeXML()
		return
	}

	// per-order for DB.

	//unified order for wechat.
	unifiedOrderRequest := &pay.UnifiedOrderRequest{
		Body:           "", //业务内容
		OutTradeNo:     "", //ID
		TotalFee:       0,  //金额>0,分
		SpbillCreateIP: "192.168.0.1",
		NotifyURL:      "..." + "/wechat/callback/my_path", // call back.
		TradeType:      "NATIVE",
	}
	logrus.Infof("统一下单：请求参数：%+v", unifiedOrderRequest)
	resp, err := pay.UnifiedOrder2(until.WechatClient, unifiedOrderRequest)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"err":         err,
			"client":      fmt.Sprintf("%#v", until.WechatClient),
			"requestData": fmt.Sprintf("%#v", unifiedOrderRequest),
		}).Error("pay.UnifiedOrder2")
		wXNotifyResp.ReturnMsg = "系统异常"
		this.Data["xml"] = wXNotifyResp
		this.ServeXML()
		return
	}
	logrus.Infof("统一下单：返回参数：%+v", resp)
	// 回应微信
	returnParams := &ReturnResult{
		ReturnCode: "SUCCESS",
		PrepayId:   resp.PrepayId,
		AppId:      until.WechatClient.AppId(),
		MchId:      until.WechatClient.MchId(),
		NonceStr:   callback.NonceStr,
		ResultCode: "FAIL",
	}

	// gen Sign
	signParams := make(map[string]string)
	signParams["return_code"] = "SUCCESS"
	signParams["appid"] = returnParams.AppId
	signParams["mch_id"] = returnParams.MchId
	signParams["prepay_id"] = returnParams.PrepayId
	signParams["nonce_str"] = returnParams.NonceStr
	signParams["result_code"] = "FAIL"
	signFail := core.Sign2(signParams, until.WechatClient.ApiKey(), nil)
	returnParams.Sign = signFail

	//success
	returnParams.ResultCode = "SUCCESS"
	signParams["result_code"] = "SUCCESS"
	signSuccess := core.Sign2(signParams, until.WechatClient.ApiKey(), nil)
	returnParams.Sign = signSuccess

	// TO XML
	logrus.Debugf("请求：%+v", returnParams)
	this.Data["xml"] = returnParams
	this.ServeXML()
	return
}

type (
	WechatPayResult struct {
		beego.Controller
	}
	WXPayNotifyResp struct {
		XMLName xml.Name `xml:"xml"`

		ReturnCode string `xml:"return_code"` // SUCCESS
		ReturnMsg  string `xml:"return_msg"`
	}
)

// WechatPayResult 微信支付结果通知异步回调
func (this *WechatPayResult) Post() {
	var resp WXPayNotifyResp
	body := this.Ctx.Input.CopyBody(99999)
	if body == nil {
		logrus.Debugf("读取http body失败，原因!:%s", string(body))
		resp.ReturnCode = "FAIL"
		resp.ReturnMsg = "读取body失败"
		this.Data["xml"] = resp
		this.ServeXML()
		return
	}

	logrus.Info("微信支付异步通知，HTTP Body:", string(body))
	reqMap := make(map[string]string, 0)
	err := xml.Unmarshal(body, (*until.StringMap)(&reqMap))
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"remark": "解析HTTP Body格式到xml失败，原因!",
			"err":    err,
			"body":   string(body),
		}).Error("xml.Unmarshal")
		resp.ReturnCode = "FAIL"
		resp.ReturnMsg = "解析body失败"
		this.Data["xml"] = resp
		this.ServeXML()
		return
	}
	// 获取签名.
	wechatSign := reqMap["sign"]
	delete(reqMap, "sign") //sign--不参与签名

	// 非必须参数.
	if reqMap["settlement_total_fee"] == "0" {
		delete(reqMap, "settlement_total_fee") //0--不参与签名
	}
	//进行签名校验
	if !WxpayVerifySign(reqMap, wechatSign) { //支付结果通知
		logrus.WithFields(logrus.Fields{
			"wechatSign": wechatSign,
			"body":       string(body),
			"reqMap":     reqMap,
		}).Debugf("WxpayVerifySign")
		resp.ReturnCode = "FAIL"
		resp.ReturnMsg = "签名校验失败"
		this.Data["xml"] = resp
		this.ServeXML()
		return
	}
	orderId := reqMap["out_trade_no"] // 商户系统内部订单id
	//totalFee := reqMap["total_fee"]

	//商户系统对于支付结果通知的内容一定要做签名验证,并校验返回的订单金额是否与商户侧的订单金额一致，
	// 防止数据泄漏导致出现“假通知”，造成资金损失

	// 以下是业务处理
	go AfterWechatSuccessPay(orderId, reqMap["openid"], reqMap["transaction_id"])
	resp.ReturnCode = "SUCCESS"
	resp.ReturnMsg = "OK"
	logrus.Infof("请求微信参数：%s", resp)
	this.Data["xml"] = resp
	this.ServeXML()
	return
}

// WxpayVerifySign 微信支付签名验证
func WxpayVerifySign(needVerifyM map[string]string, sign string) bool {
	signGen := core.Sign(needVerifyM, until.WechatClient.ApiKey(), nil)

	if sign == signGen {
		logrus.Debugf("签名认证通过,Sign:%s", sign)
		return true
	}
	logrus.Debugf("签名认证未通过！wechat.Sign:%s,genSign:%s,parameters:%v", sign, signGen, needVerifyM)
	return false
}

// AfterWechatSuccessPay 微信成功付款后 业务处理
func AfterWechatSuccessPay(orderId, openId, transactionId string) error {
	if orderId == "" {
		logrus.Debugf("AfterWechatSuccessPay,orderId is nil ")
		return errors.New("the orderId is nil")
	}
	//业务处理...

	return nil
}
