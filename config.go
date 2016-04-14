package scheduler

import (
	"encoding/json"
	"io/ioutil"
)

type ConfigCommand struct {
	Name      string
	Pool      string
	Interval  string
	Timeout   string
	Command   string
	Params    []string
	StartTime string
}

type Config struct {
	Variables    map[string]string
	Enabled      []string
	Disabled     []string
	HttpService  string
	Httpport     string
	Schedulerurl string
	Commands     []ConfigCommand
}

func (c *Config) LoadFile(filename string) error {
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, c)
}
