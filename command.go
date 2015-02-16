package scheduler

import (
	"bytes"
	"log"
	"os/exec"
	"strings"
	"time"
)

type Command struct {
	Name     string
	Interval time.Duration
	Exec     string
	Enabled  bool
	lastRun  time.Time
}

// Split a command-line into the executable and the parameters
func parse_exec(cmd string) (string, string) {
	if cmd[0] == uint8('"') {
		close := strings.Index(cmd[1:], "\"") - 1
		if close > 0 {
			return cmd[1:close], cmd[close+2:]
		}
	}
	firstSpace := strings.Index(cmd, " ")
	return cmd[0:firstSpace], cmd[firstSpace+1:]
}

func substitute_variables(params string, variables map[string]string) string {
	for key, value := range variables {
		params = strings.Replace(params, "!"+key, value, -1)
	}
	return params
}

func (c *Command) MustRun() bool {
	return c.Enabled && (c.lastRun.IsZero() || time.Now().Sub(c.lastRun) >= c.Interval)
}

func (c *Command) Run(variables map[string]string) {
	c.lastRun = time.Now()
	exe, params := parse_exec(substitute_variables(c.Exec, variables))
	log.Printf("Running %v (%v) (%v)", c.Name, exe, params)
	cmd := exec.Command(exe, params)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		log.Printf("Success %v", c.Name)
	} else {
		log.Printf("Failed %v: %v", c.Name, err)
		log.Print("stdout: " + stdout.String())
		log.Print("stderr: " + stderr.String())
	}
}
