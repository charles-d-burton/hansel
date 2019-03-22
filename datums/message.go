package datums

type Message struct {
	Sequence int      `yaml:"sequence"`
	Type     string   `yaml:"type"`
	Actions  []string `yaml:"actions"`
	Targets  []string `yaml:"targets"`
}

type ClientStatus struct {
	Name    string
	Message string
}
