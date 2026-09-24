package guard

type YYBRestartRequest = WMPFRestartRequest

func RestartYYBMiniapp(request YYBRestartRequest) (RestartResult, error) {
	return restartWMPFMiniapp(request, wmpfRestartProfile{platform: "yyb", label: "YYB"})
}
