package datums

type ClientResult struct {
	Name    string
	Results []string
}

func (result *ClientResult) GetResult() []string {
	return result.Results
}

func (result *ClientResult) GetClientInfo() HostInfo {
	hostinfo := HostInfo{Name: result.Name}
	return hostinfo
}

type ClientStatus struct {
	Name    string
	Message string
}

func (status *ClientStatus) GetResult() []string {
	return []string{status.Message}
}

func (status *ClientStatus) GetClientInfo() HostInfo {
	hostinfo := HostInfo{Name: status.Name}
	return hostinfo
}
