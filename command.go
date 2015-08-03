package scheduler

import (
	"bytes"
	"github.com/IMQS/log"
	"os/exec"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

// Quite a few of Command's methods take a 'now' parameter. We do it like this
// to make it easy to test.

// We differentiate between tasks with a 24 hour interval and all other tasks.
// Those with a 24h interval are called "daily" tasks, and they get special treatment.
// We give daily tasks a 2 hour window in which to start. If they don't start within
// that window, then we don't run them at all. This is to safeguard things like nightly
// backups that shouldn't be running during the day.

// We make an arbitrary buffer of 2 hours for daily tasks.
// They must start within 2 hours of their start time, or they don't start at all.
// We assume that our scheduler service will never be down for more than a few minutes,
// so it's very unlikely that we miss our 2 hour window.
const dailyCommandWindow = 2 * time.Hour

/* A scheduled task
Every scheduled task belongs to a pool. At most one job from a pool may run at any one time.
*/
type Command struct {
	Name            string
	Pool            string
	Enabled         bool
	StartTime       time.Time // Year,Month,Day is ignored. Only hour,minute,second (since midnight) is important
	Interval        time.Duration
	Timeout         time.Duration
	Exec            string
	Params          []string
	lastRun         time.Time
	isRunningAtomic int32
}

type SortCommands struct {
	List []*Command
	Now  time.Time
}

func (v SortCommands) Len() int      { return len(v.List) }
func (v SortCommands) Swap(i, j int) { v.List[i], v.List[j] = v.List[j], v.List[i] }
func (v SortCommands) Less(i, j int) bool {
	// A daily task is always "more overdue" (ie more important) than a non-daily task
	if v.List[i].isDaily() != v.List[j].isDaily() {
		return v.List[j].isDaily()
	}
	return v.List[i].timeOverdue(v.Now) < v.List[j].timeOverdue(v.Now)
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

func (c *Command) MustRun(now time.Time) bool {
	if atomic.LoadInt32(&c.isRunningAtomic) != 0 {
		return false
	}
	if c.isDaily() {
		return c.Enabled && ((now.Sub(c.mostRecentStartTime(now)) < dailyCommandWindow) && (now.Sub(c.lastRun) > dailyCommandWindow))
	} else {
		return c.Enabled && (now.Sub(c.lastRun) >= c.Interval)
	}
}

func (c *Command) SetStartTime(hour, minute int) {
	if !c.isDaily() {
		panic("StartTime is only applicable to daily tasks")
	}
	c.StartTime = time.Date(2000, time.January, 1, hour, minute, 0, 0, time.Local)
}

func (c *Command) isDaily() bool {
	return c.Interval == 24*time.Hour
}

// Find the most recent point in history that crossed StartTime
func (c *Command) mostRecentStartTime(now time.Time) time.Time {
	if !c.isDaily() {
		panic("mostRecentStartTime is only applicable to daily tasks")
	}
	startOfToday := now.Add(-offsetFromStartOfDay(now))
	at_today := startOfToday.Add(offsetFromStartOfDay(c.StartTime))
	at_yesterday := at_today.Add(-24 * time.Hour)
	if now.Sub(at_today) > 0 {
		return at_today
	} else {
		return at_yesterday
	}
}

func (c *Command) timeOverdue(now time.Time) time.Duration {
	if c.isDaily() {
		if !c.lastRun.IsZero() {
			return now.Sub(c.lastRun) - c.Interval
		} else {
			return now.Sub(c.mostRecentStartTime(now))
		}
	} else {
		return now.Sub(c.lastRun) - c.Interval
	}
}

func (c *Command) Run(logger *log.Logger, variables map[string]string) {
	// It is important that we toggle isRunningAtomic = 1 from here.
	// If we only toggled isRunningAtomic = 1 from inside the goroutine that we launch,
	// then we'd be at risk of the function that called Run() trying to launch the same job twice.
	atomic.StoreInt32(&c.isRunningAtomic, 1)
	go func() {
		defer atomic.StoreInt32(&c.isRunningAtomic, 0)
		c.lastRun = time.Now()
		params := strings.Fields(substitute_variables(strings.Join(c.Params, " "), variables))
		logger.Infof("Running '%v' %v (%v)", c.Name, c.Exec, params)
		cmd := exec.Command(c.Exec, params...)
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Start()
		if err != nil {
			logger.Errorf("Failed %v: %v", c.Name, err)
			logger.Infof("stdout: " + stdout.String())
			logger.Infof("stderr: " + stderr.String())
		} else {
			// wait or timeout
			donec := make(chan error, 1)
			go func() {
				donec <- cmd.Wait()
			}()
			select {
			case <-time.After(c.Timeout):
				logger.Errorf("%v timed out after %v seconds.", c.Name, c.Timeout)
				if !killProcessTree(cmd.Process.Pid) {
					logger.Errorf("Failed to kill process.")
				}
			case <-donec:
				// Success logs are just spammy.
				//logger.Infof("Success %v", c.Name)
			}
		}
	}()
}

func offsetFromStartOfDay(t time.Time) time.Duration {
	return time.Second * time.Duration(t.Hour()*3600+t.Minute()*60+t.Second())
}

// Prioritize the list of commands, and return the next one (if any) that is ready to run.
// If no command is ready to run, return nil
func NextRunnable(cmd []*Command, now time.Time) *Command {

	// Assemble the list of all busy pools
	busyPools := map[string]bool{}
	for _, c := range cmd {
		if atomic.LoadInt32(&c.isRunningAtomic) != 0 {
			busyPools[c.Pool] = true
		}
	}

	// Produce a filtered list of commands that are runnable
	// Are we doing something wrong by reading the atomic variable isRunning twice?
	// Yes, but it's OK, because the behaviour here is conservative. A command cannot go
	// from not-running to running, because this function is called from the one-and-only
	// goroutine that launches commands.
	filtered := []*Command{}
	for _, c := range cmd {
		if atomic.LoadInt32(&c.isRunningAtomic) == 0 && c.MustRun(now) && !busyPools[c.Pool] {
			filtered = append(filtered, c)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	sortable := SortCommands{
		List: filtered,
		Now:  now,
	}
	sort.Sort(sort.Reverse(sortable))
	return sortable.List[0]
}
