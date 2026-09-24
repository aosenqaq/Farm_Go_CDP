package guard

type WeChatRestartRequest = WMPFRestartRequest

func RestartWeChatMiniapp(request WeChatRestartRequest) (RestartResult, error) {
	return restartWMPFMiniapp(request, wmpfRestartProfile{platform: "wx", label: "WeChat"})
}
