package scheduler

import (
	"encoding/json"
	"io/ioutil"
)

type Config struct {
	Variables map[string]string
	Enabled   []string
	Disabled  []string
}

func (c *Config) LoadFile(filename string) error {
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, c)
}
