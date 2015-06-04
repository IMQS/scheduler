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
	"github.com/IMQS/scheduler"
	"github.com/natefinch/lumberjack"
	"log"
	"os/exec"
	"strconv"
	"time"
)

var commands []scheduler.Command
var config scheduler.Config

const (
	taskConfUpdate = "ImqsConf Update"
)

func main() {

	log.SetOutput(&lumberjack.Logger{
		Filename:   "c:/imqsvar/logs/scheduler.log",
		MaxSize:    20, // megabytes
		MaxBackups: 3,
		MaxAge:     30, //days
	})

	setDefaultVariables()
	addCommands()
	loadConfig()

	log.Printf("Scheduler starting")
	log.Printf("Variables: %v", config.Variables)
	log.Printf("Enabled: %v", cmdEnabledList())

	if !scheduler.RunAsService(run) {
		run()
	}

	log.Printf("Exiting")
}

func getImqsHttpPort() int {
	defaultPort := 80
	cmd := exec.Command("c:\\imqsbin\\bin\\imqsrouter.exe", "-show-http-port", "-mainconfig", "c:\\imqsbin\\static-conf\\router-config.json", "-auxconfig", "c:\\imqsbin\\conf\\router-config.json")
	outBuf := &bytes.Buffer{}
	cmd.Stdout = outBuf
	if err := cmd.Run(); err != nil {
		log.Printf("Error running router: %v", err)
		return defaultPort
	}
	if port, err := strconv.Atoi(string(outBuf.Bytes())); err != nil || port <= 0 {
		log.Printf("Error reading http port from router: %v", err)
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

func addCommands() {
	add := func(enabled bool, name string, interval_seconds int, timeout_seconds int, exec string, params ...string) {
		commands = append(commands, scheduler.Command{
			Name:     name,
			Interval: time.Second * time.Duration(interval_seconds),
			Timeout:  time.Second * time.Duration(timeout_seconds),
			Exec:     exec,
			Params:   params,
			Enabled:  enabled,
		})
	}

	minute := 60
	hour := 3600

	add(true, "Locator", 15, 2*hour, "c:\\imqsbin\\bin\\imqstool", "locator", "imqs", "!LOCATOR_SRC", "c:\\imqsvar\\staging", "!JOB_SERVICE_URL", "!LEGACY_LOCK_DIR")
	add(true, "ImqsTool Importer", 15, 6*hour, "c:\\imqsbin\\bin\\imqstool", "importer", "!LEGACY_LOCK_DIR", "!JOB_SERVICE_URL")
	add(true, "Docs Importer", 15, 2*hour, "ruby", "c:\\imqsbin\\jsw\\ImqsDocs\\importer\\importer.rb")
	add(true, "ImqsConf Update", 5*minute, 30*minute, "c:\\imqsbin\\cronjobs\\update_runner.bat", "conf")
	add(true, "ImqsBin Update", 5*minute, 2*hour, "c:\\imqsbin\\cronjobs\\update_runner.bat", "imqsbin")
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

func loadConfig() {
	if err := config.LoadFile("c:/imqsbin/conf/scheduled-tasks.json"); err != nil {
		log.Printf("Error loading config file 'scheduled-tasks.json': %v", err)
	}
	for _, name := range config.Enabled {
		for i, _ := range commands {
			if commands[i].Name == name {
				commands[i].Enabled = true
			}
		}
	}
	for _, name := range config.Disabled {
		for i, _ := range commands {
			if commands[i].Name == name {
				commands[i].Enabled = false
			}
		}
	}
}

func run() {
	for {
		for i := range commands {
			if commands[i].MustRun() {
				commands[i].Run(config.Variables)
			}
		}
		time.Sleep(5 * time.Second)
		loadConfig()
	}
}
