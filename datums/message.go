package datums

type ServerMessage interface {
	GetSequence() int
	GetType() string
	Execute() ([]string, error)
}

type ClientMessage interface {
	GetClientInfo() HostInfo
	GetResults() []string
}

type HostInfo struct {
	Name string
}
