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
	"time"
)

var commands []*scheduler.Command
var config scheduler.Config
var logger *log.Logger

const (
	taskConfUpdate = "ImqsConf Update"
)

func main() {
	logger = log.New("c:/imqsvar/logs/scheduler.log")

	setDefaultVariables()
	addCommands()
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

func addCommands() {
	add := func(enabled bool, name, pool string, interval time.Duration, timeout time.Duration, exec string, params ...string) *scheduler.Command {
		commands = append(commands, &scheduler.Command{
			Name:     name,
			Pool:     pool,
			Interval: interval,
			Timeout:  timeout,
			Exec:     exec,
			Params:   params,
			Enabled:  enabled,
		})
		return commands[len(commands)-1]
	}

	//minute := time.Minute
	hour := time.Hour
	//daily := 24 * time.Hour

	// The second parameter here is the job pool. For any pool, at most one job can be running. For example, we have two jobs
	// in the "update" pool, so only one of them can be running at a time.

	// Imports and update are mutually exclusive. We don't want to interrupt an import with an update, nor vice versa,
	// and this is why we place them both in the same pool. They have their own independent "lock file" mechanisms
	// to ensure that they never run concurrently, but then we end up with a bunch of error messages in our log file,
	// so we simply prevent that from ever happening here by placing them in the same pool.
	import_update_pool := "import_update"

	// Jobs that run once a day (at night), and drain a machine's resources
	//heavy_daily_pool := "heavy_daily"

	// Imports
	add(true, "Locator", import_update_pool, 15, 2*hour, "c:\\imqsbin\\bin\\imqstool", "locator", "imqs", "!LOCATOR_SRC", "c:\\imqsvar\\staging", "!JOB_SERVICE_URL", "!LEGACY_LOCK_DIR")
	add(true, "ImqsTool Importer", import_update_pool, 15, 6*hour, "c:\\imqsbin\\bin\\imqstool", "importer", "!LEGACY_LOCK_DIR", "!JOB_SERVICE_URL")
	//add(true, "Docs Importer", import_update_pool, 15, 2*hour, "ruby", "c:\\imqsbin\\jsw\\ImqsDocs\\importer\\importer.rb")

	// Updaters
	//add(true, "ImqsConf Update", import_update_pool, 5*minute, 30*minute, "c:\\imqsbin\\cronjobs\\update_runner.bat", "conf -prod")
	//add(true, "ImqsBin Update", import_update_pool, 5*minute, 2*hour, "c:\\imqsbin\\cronjobs\\update_runner.bat", "imqsbin -prod")

	// Other
	//add(true, "Ping", "ping", minute, 5*minute, "ruby", "c:\\imqsbin\\cronjobs\\ping_services.rb")
	//add(true, "Auth Log Scraper", "logscrape", 24*hour, 24*hour, "ruby", "c:\\imqsbin\\cronjobs\\logscrape.rb")
	//add(false, "Theme saver", "theme_saver", minute, 5*minute, "ruby", "c:\\imqsbin\\conftools\\theme_saver.rb", "prepare_for_edit")

	// Heavy daily pool
	//backup := add(true, "Backup", heavy_daily_pool, daily, 12*hour, "ruby", "c:\\imqsbin\\cronjobs\\backup_v8.rb")
	//backup.SetStartTime(23, 0)

	//cleanup := add(true, "Cleanup", heavy_daily_pool, daily, 12*hour, "ruby", "c:\\imqsbin\\cronjobs\\cleanup.rb")
	//cleanup.SetStartTime(1, 0)

	//vacuum := add(true, "Search Vacuum", heavy_daily_pool, daily, 5*hour, "c:\\imqsbin\\bin\\imqssearch.exe", "-c=c:\\imqsbin\\conf\\search.json", "vacuum")
	//vacuum.SetStartTime(2, 0)
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
		logger.Errorf("Error loading config file 'scheduled-tasks.json': %v", err)
	}
	for _, name := range config.Enabled {
		for _, c := range commands {
			if c.Name == name {
				c.Enabled = true
			}
		}
	}
	for _, name := range config.Disabled {
		for _, c := range commands {
			if c.Name == name {
				c.Enabled = false
			}
		}
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
