package datums

import (
	"log"
	"os/exec"
	"strings"
)

type CommandRunner struct {
	Sequence int      `yaml:"sequence"`
	Type     string   `yaml:"type"`
	Actions  []string `yaml:"actions"`
	Targets  []string `yaml:"targets"`
}

func (runner *CommandRunner) GetSequence() int {
	return runner.Sequence
}

func (runner *CommandRunner) GetType() string {
	return runner.Type
}

func (runner *CommandRunner) Execute() ([]string, error) {
	var result []string
	for _, action := range runner.Actions {
		log.Println("Running:")
		log.Println(action)
		descmd := strings.Fields(action)
		bin, err := exec.LookPath(descmd[0])
		if err != nil {
			log.Println(err)
		}
		cmd := exec.Command(bin, descmd[1:]...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Fatalf("cmd.Run() failed with %s\n", err)
		}
		result = append(result, string(out))
	}
	return result, nil
}
