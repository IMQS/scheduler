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
	Timeout  time.Duration
	Exec     string
	Params   []string
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
	params := strings.Fields(substitute_variables(strings.Join(c.Params, " "), variables))
	log.Printf("Running %v (%v)", c.Exec, params)
	cmd := exec.Command(c.Exec, params...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Start()
	if err != nil {
		log.Printf("Failed %v: %v", c.Name, err)
		log.Print("stdout: " + stdout.String())
		log.Print("stderr: " + stderr.String())

	} else {
		// wait or timeout
		donec := make(chan error, 1)
		go func() {
			donec <- cmd.Wait()
		}()
		select {
		case <-time.After(c.Timeout):
			log.Printf("%v timed out after %v seconds.", c.Name, c.Timeout)
			if !killProcessTree(cmd.Process.Pid) {
				log.Printf("Failed to kill process.")
			}
		case <-donec:
			// Success logs are just spammy.
			//log.Printf("Success %v", c.Name)
		}
	}
}
