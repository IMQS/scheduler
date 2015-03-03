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
	"github.com/IMQS/scheduler"
	"github.com/natefinch/lumberjack"
	"log"
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

func setDefaultVariables() {
	config.Variables = make(map[string]string)
	config.Variables["LOCATOR_SRC"] = "c:\\temp\\ImportClients"
	config.Variables["LEGACY_LOCK_DIR"] = "c:\\imqsvar\\locks" // No longer needed, since we serialize all scheduled tasks. Should remove from imqstool.
	config.Variables["JOB_SERVICE_URL"] = "http://localhost"
}

func addCommands() {
	add := func(enabled bool, name string, interval_seconds int, exec string, params ...string) {
		commands = append(commands, scheduler.Command{
			Name:     name,
			Interval: time.Second * time.Duration(interval_seconds),
			Exec:     exec,
			Params:   params,
			Enabled:  enabled,
		})
	}

	add(true, "Locator", 15, "c:\\imqsbin\\bin\\imqstool", "locator", "imqs", "!LOCATOR_SRC", "c:\\imqsvar\\staging", "!JOB_SERVICE_URL", "!LEGACY_LOCK_DIR")
	add(true, "ImqsTool Importer", 15, "c:\\imqsbin\\bin\\imqstool", "importer", "!LEGACY_LOCK_DIR", "!JOB_SERVICE_URL")
	add(true, "Docs Importer", 15, "ruby, c:\\imqsbin\\jsw\\ImqsDocs\\importer\\importer.rb")
	add(true, "ImqsConf Update", 3*60, "c:\\imqsbin\\cronjobs\\update_runner.bat", "conf")
	add(true, "ImqsBin Update", 3*60, "c:\\imqsbin\\cronjobs\\update_runner.bat", "imqsbin")
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
		for _, cmd := range commands {
			if cmd.MustRun() {
				cmd.Run(config.Variables)
			}
		}
		time.Sleep(5 * time.Second)
	}
}
