package desktop

type appVersion struct {
	Number int    `json:"number"`
	Name   string `json:"name"`
}

type appProgramVersion struct {
	Number      int    `json:"number"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	DownloadURL string `json:"downloadUrl,omitempty"`
	Notice      string `json:"notice,omitempty"`
}

type appProgramNotice struct {
	Content   string `json:"content"`
	ErrorCode string `json:"errorCode,omitempty"`
	Message   string `json:"message,omitempty"`
}

type appUpdateState struct {
	Current   appVersion        `json:"current"`
	Latest    appProgramVersion `json:"latest"`
	Available bool              `json:"available"`
	ErrorCode string            `json:"errorCode,omitempty"`
	Message   string            `json:"message,omitempty"`
}

func (a *App) CheckForUpdates() appUpdateState {
	return appUpdateState{
		Current:   appVersion{Name: "0.0.0"},
		ErrorCode: "update_unavailable",
		Message:   "未配置远程更新",
	}
}

func (a *App) GetProgramNotice() appProgramNotice {
	return appProgramNotice{
		ErrorCode: "notice_unavailable",
		Message:   "未配置程序公告",
	}
}
