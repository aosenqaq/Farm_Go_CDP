package runtime

type Phase string

const (
	PhaseIdle         Phase = "idle"
	PhaseListening    Phase = "listening"
	PhaseHandshaking  Phase = "handshaking"
	PhaseReady        Phase = "ready"
	PhaseDisconnected Phase = "disconnected"
	PhaseError        Phase = "error"
)

type Status struct {
	Target         string `json:"target"`
	Phase          Phase  `json:"phase"`
	Connected      bool   `json:"connected"`
	Ready          bool   `json:"ready"`
	InstanceID     string `json:"instanceId,omitempty"`
	HostVersion    string `json:"hostVersion,omitempty"`
	LastSeenAt     string `json:"lastSeenAt,omitempty"`
	ProgressDetail string `json:"progressDetail,omitempty"`
	LastError      string `json:"lastError,omitempty"`
}

func InitialStatus() Status {
	return Status{
		Target: string(RuntimeTargetQQWS),
		Phase:  PhaseIdle,
	}
}
