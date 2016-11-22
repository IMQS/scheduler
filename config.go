package scheduler

import (
	"crypto/sha1"
	"encoding/hex"
	"github.com/IMQS/serviceconfigsgo"
	"sort"
	"strings"
)

const serviceConfigFileName = "scheduled-tasks.json"
const serviceConfigVersion  = 1
const serviceName           = "ImqsScheduler"

// If you add or remove any members here, be sure to update HashSignature
type ConfigCommand struct {
	Name      string
	Pool      string
	Interval  string
	Timeout   string
	Command   string
	Params    []string
	StartTime string
}

// If you add or remove any members here, be sure to update HashSignature
type Config struct {
	Variables map[string]string
	Enabled   []string
	Disabled  []string
	Commands  []ConfigCommand
}

func (c *Config) LoadFile(filename string) error {
	err := serviceconfig.GetConfig(filename, serviceName, serviceConfigVersion, serviceConfigFileName, c)
	if err != nil {
		return err
	}
	return nil
}

func (c *Config) SetCommandEnabled(cmd string, enabled bool) {
	// This function is extremely unpleasant and verbose, but the JSON config design forces this upon us
	indexInDisabled := -1
	indexInEnabled := -1
	for i := range c.Disabled {
		if c.Disabled[i] == cmd {
			indexInDisabled = i
			break
		}
	}
	for i := range c.Enabled {
		if c.Enabled[i] == cmd {
			indexInEnabled = i
			break
		}
	}

	if enabled && indexInDisabled != -1 {
		// remove from disabled list
		c.Disabled = append(c.Disabled[:indexInDisabled], c.Disabled[indexInDisabled+1:]...)
	}

	if !enabled && indexInEnabled != -1 {
		// remove from enabled list
		c.Enabled = append(c.Enabled[:indexInEnabled], c.Enabled[indexInEnabled+1:]...)
	}

	if enabled && indexInEnabled == -1 {
		// add to enabled list
		c.Enabled = append(c.Enabled, cmd)
	}

	if !enabled && indexInDisabled == -1 {
		// add to disabled list
		c.Disabled = append(c.Disabled, cmd)
	}
}

func (c *ConfigCommand) HashSignature() string {
	return c.Name + "." + c.Pool + "." + c.Interval + "." + c.Timeout + "." + c.Command + "." + strings.Join(c.Params, ",") + "." + c.StartTime
}

// Returns a hex encoded SHA1 hash of all the contents of the configuration. This is used to
// detect whether the config has changed since the last time we loaded the configuration.
// Only if it has changed, do we emit a log message about the new config.
func (c *Config) HashSignature() string {
	s := ""
	s += "> Enabled: " + strings.Join(c.Enabled, ",")
	s += "> Disabled: " + strings.Join(c.Disabled, ",")
	keys := []string{}
	for k, _ := range c.Variables {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		s += "(" + k + ")=(" + c.Variables[k] + ")"
	}
	for _, cmd := range c.Commands {
		s += cmd.HashSignature()
	}
	hash := sha1.Sum([]byte(s))
	return hex.EncodeToString(hash[:])
}
