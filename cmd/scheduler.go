/* A task scheduler (like cron) for Windows.

It may seem ridiculous to build a task scheduler. Why not just use cron on linux, and scheduled tasks on Windows?
Cron is fine, but the Windows task scheduler has proven unreliable.

We don't know where exactly the problem lies in our usage of Windows scheduled tasks, but having
struggled with them for a few years, it seems prudent to write a small service that takes care
of our simple needs.

One problem with scheduled tasks that is built into their design is the fact that we need to
run them with the authority of our IMQS domain user. This user is created for us by the IT
department of the client we're working with, and sometimes we can't control when the password
for that user changes. When the password does change, our scheduled tasks cease to work,
and we have a bricked installation.

This system is as simple as we can possibly make it. Embedded inside the Go code is a fixed
set of things we can do. These things run at a fixed interval, and they can be turned off via
a configuration file.

Our philosophy here is "never die". So if we encounter errors, we soldier on. We must never
die, because then the server is bricked, and humans need to go sort out all bricked servers.
*/
package main

import (
	"bytes"
	"github.com/IMQS/log"
	"github.com/IMQS/scheduler"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

var commands []*scheduler.Command
var logger *log.Logger
var config scheduler.Config

const (
	taskConfUpdate = "ImqsConf Update"
)

func main() {
	logger = log.New("c:/imqsvar/logs/scheduler.log")

	setDefaultVariables()
	loadConfig()

	logger.Infof("Scheduler starting")
	logger.Infof("Variables: %v", config.Variables)
	logger.Infof("Enabled: %v", cmdEnabledList())

	if !scheduler.RunAsService(run) {
		run()
	}

	logger.Infof("Exiting")
}

func getImqsHttpPort() int {
	defaultPort := 80
	cmd := exec.Command("c:\\imqsbin\\bin\\imqsrouter.exe", "-show-http-port", "-mainconfig", "c:\\imqsbin\\static-conf\\router-config.json", "-auxconfig", "c:\\imqsbin\\conf\\router-config.json")
	outBuf := &bytes.Buffer{}
	cmd.Stdout = outBuf
	if err := cmd.Run(); err != nil {
		logger.Errorf("Error running router: %v", err)
		return defaultPort
	}
	if port, err := strconv.Atoi(string(outBuf.Bytes())); err != nil || port <= 0 {
		logger.Errorf("Error reading http port from router: %v", err)
		return defaultPort
	} else {
		return port
	}
}

func setDefaultVariables() {
	imqsHttpPort := getImqsHttpPort()
	config.Variables = make(map[string]string)
	config.Variables["LOCATOR_SRC"] = "c:\\imqsvar\\imports"
	config.Variables["LEGACY_LOCK_DIR"] = "c:\\imqsvar\\locks" // No longer needed, since we serialize all scheduled tasks. Should remove from imqstool.
	config.Variables["JOB_SERVICE_URL"] = "http://localhost"
	if imqsHttpPort != 80 {
		config.Variables["JOB_SERVICE_URL"] = config.Variables["JOB_SERVICE_URL"] + ":" + strconv.Itoa(imqsHttpPort)
	}
}

func buildCommandFromConfig(cmd scheduler.ConfigCommand, isEnabled bool) *scheduler.Command {
	// Convert time from string into time.Duration format
	interval, err := time.ParseDuration(cmd.Interval)
	if err != nil {
		logger.Errorf("Error parsing interval for task '%s': %v", cmd.Name, err)
		interval = 1 * time.Hour
	}
	timeout, err := time.ParseDuration(cmd.Timeout)
	if err != nil {
		logger.Errorf("Error parsing timeout for task '%s': %v", cmd.Name, err)
		timeout = 8 * time.Hour
	}

	// Sanity checks
	if interval < (5 * time.Second) {
		logger.Errorf("Invalid interval of less than 5 seconds for task '%v'", cmd.Name)
	}
	if interval > (24 * time.Hour) {
		logger.Errorf("Invalid interval of more than 24 hours for task '%v'", cmd.Name)
	}
	if timeout < (5 * time.Second) {
		logger.Errorf("Invalid timeout of less than 5 seconds for task '%v'", cmd.Name)
	}
	if timeout > (24 * time.Hour) {
		logger.Errorf("Invalid timeout of more than 24 hours for task '%v'", cmd.Name)
	}
	if len(strings.TrimSpace(cmd.Name)) == 0 {
		logger.Errorf("Invalid empty task name for command '%v'", cmd.Command)
	}
	if len(strings.TrimSpace(cmd.Command)) == 0 {
		logger.Errorf("Invalid empty command for task '%v'", cmd.Name)
	}

	newCommand := &scheduler.Command{
		Name:     cmd.Name,
		Pool:     cmd.Pool,
		Interval: interval,
		Timeout:  timeout,
		Exec:     cmd.Command,
		Params:   cmd.Params,
		Enabled:  isEnabled,
	}

	start_time, err := time.ParseDuration(cmd.StartTime)
	if err == nil {
		hours := int(start_time / time.Hour)
		start_time -= time.Duration(hours) * time.Hour
		minutes := int(start_time / time.Minute)
		newCommand.SetStartTime(hours, minutes)
	} else {
		logger.Errorf("Error parsing start time for cmd '%v': %v", cmd.Name, err)
	}

	return newCommand
}

func loadConfig() {
	if err := config.LoadFile("c:/imqsbin/static-conf/scheduled-tasks.json"); err != nil {
		logger.Errorf("Error loading static config file 'scheduled-tasks.json': %v", err)
	}

	// Build map of enabled jobs
	var enabledMap map[string]bool = make(map[string]bool)
	for _, e := range config.Enabled {
		enabledMap[e] = true
	}
	for _, e := range config.Disabled {
		enabledMap[e] = false
	}

	// Overlay the client config over commands defined in the static config
	var overlayConfig scheduler.Config
	if err := overlayConfig.LoadFile("c:/imqsbin/conf/scheduled-tasks.json"); err != nil {
		logger.Errorf("Error loading client config file 'scheduled-tasks.json': %v", err)
	}

	// Overlay variables
	for key, value := range overlayConfig.Variables {
		config.Variables[key] = value
	}

	// Build map of enabled jobs
	for _, e := range overlayConfig.Enabled {
		enabledMap[e] = true
	}
	for _, e := range overlayConfig.Disabled {
		enabledMap[e] = false
	}

	// Replace tasks if needed
	for _, t := range overlayConfig.Commands {
		foundCommand := false
		for i, c := range config.Commands {
			if c.Name == t.Name {
				foundCommand = true
				config.Commands[i] = t
				break
			}
		}

		// If command wasn't found, add it
		if !foundCommand {
			config.Commands = append(config.Commands, t)
		}
	}

	// Add or overwrite to commands array
	// Don't clobber things like 'lastRun' and 'isRunningAtomic' for existing commands
	for _, t := range config.Commands {
		newCommand := buildCommandFromConfig(t, enabledMap[t.Name])

		foundCommand := false
		for i, c := range commands {
			if newCommand.Name == c.Name {
				foundCommand = true
				commands[i].Pool = newCommand.Pool
				commands[i].Enabled = newCommand.Enabled
				commands[i].StartTime = newCommand.StartTime
				commands[i].Interval = newCommand.Interval
				commands[i].Timeout = newCommand.Timeout
				commands[i].Exec = newCommand.Exec
				commands[i].Params = newCommand.Params
				break
			}
		}

		if !foundCommand {
			commands = append(commands, newCommand)
		}
	}
}

func cmdEnabledList() string {
	list := ""
	for _, cmd := range commands {
		if cmd.Enabled {
			list += cmd.Name + ", "
		}
	}
	if list != "" {
		return list[0 : len(list)-2]
	} else {
		return list
	}
}

// This method handles the http request used to start a job.
func runCommandNow(commandName string) {
	// Look for the Importer command in the list of commands
	var command *scheduler.Command
	for _, cmd := range commands {
		if cmd.Name == commandName {
			command = cmd
		}
	}
	if len(command.Name) == 0 {
		logger.Errorf("Error cannot find requested command provided in url")
		return
	}
	command.Run(logger, config.Variables)
}

func run() {
	tickChan := time.NewTicker(time.Second * 5).C
	httpChan := make(chan string)

	http.HandleFunc("/scheduler/", func(w http.ResponseWriter, r *http.Request) {
		commandName := r.FormValue("command")
		if len(commandName) == 0 {
			http.Error(w, "Command name missing from request", http.StatusBadRequest)
			return
		}
		httpChan <- commandName
		w.WriteHeader(http.StatusOK)
	})
	go http.ListenAndServe(":2014", nil)

	for {
		select {
		case commandName := <-httpChan:
			{
				runCommandNow(commandName)
			}
		case <-tickChan:
			{
				next := scheduler.NextRunnable(commands, time.Now())
				if next != nil {
					next.Run(logger, config.Variables)
				}
				loadConfig()
			}
		}
	}
}
