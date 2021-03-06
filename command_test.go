package scheduler

import (
	"testing"
	"time"
)

func TestDailyTasks(t *testing.T) {
	loc := time.FixedZone("Pretoria", -7200)

	var tzero time.Time

	// basics
	t.Logf("now - zero = %v\n", time.Now().Sub(tzero))

	nowPresent := time.Date(2015, 07, 15, 5, 3, 20, 0, loc)
	nowPast5h := nowPresent.Add(-5 * time.Hour)
	nowPast5m := nowPresent.Add(-5 * time.Minute)
	nowFuture5m := nowPresent.Add(5 * time.Minute)
	nowFuture35m := nowPresent.Add(35 * time.Minute)
	nowFuture5h := nowPresent.Add(5 * time.Hour)

	for haveLastRun := 0; haveLastRun < 2; haveLastRun++ {
		// Test a daily task
		t.Logf("Daily")
		c := Command{
			Enabled: true,
		}
		c.Interval = 24 * time.Hour
		c.SetStartTime(5, 3)
		if haveLastRun == 1 {
			c.lastRun = nowPresent.Add(-24 * time.Hour)
		}
		t.Logf("Task start time: %v", c.StartTime)
		t.Logf("Now: %v", nowPresent)
		t.Logf("past 5h    %v  MustRun: %v  Overdue: %v", c.mostRecentStartTime(nowPast5h), c.MustRun(nowPast5h), c.timeOverdue(nowPast5h))
		t.Logf("past 5m    %v  MustRun: %v  Overdue: %v", c.mostRecentStartTime(nowPast5m), c.MustRun(nowPast5m), c.timeOverdue(nowPast5m))
		t.Logf("present    %v  MustRun: %v  Overdue: %v", c.mostRecentStartTime(nowPresent), c.MustRun(nowPresent), c.timeOverdue(nowPresent))
		t.Logf("future 5m  %v  MustRun: %v  Overdue: %v", c.mostRecentStartTime(nowFuture5m), c.MustRun(nowFuture5m), c.timeOverdue(nowFuture5m))
		t.Logf("future 5h  %v  MustRun: %v  Overdue: %v", c.mostRecentStartTime(nowFuture5h), c.MustRun(nowFuture5h), c.timeOverdue(nowFuture5h))
		if haveLastRun == 0 {
			if c.MustRun(nowPast5h) || c.MustRun(nowPast5m) || !c.MustRun(nowPresent) || !c.MustRun(nowFuture5m) || c.MustRun(nowFuture5h) {
				t.Fatalf("MustRun incorrect")
			}
		}
	}

	// Test a non-daily task
	t.Logf("Regular (non-daily)")
	c := Command{
		Enabled: true,
	}
	c.Interval = 30 * time.Minute
	c.lastRun = nowPresent
	t.Logf("past 5h     MustRun: %v  Overdue: %v", c.MustRun(nowPast5h), c.timeOverdue(nowPast5h))
	t.Logf("past 5m     MustRun: %v  Overdue: %v", c.MustRun(nowPast5m), c.timeOverdue(nowPast5m))
	t.Logf("present     MustRun: %v  Overdue: %v", c.MustRun(nowPresent), c.timeOverdue(nowPresent))
	t.Logf("future 5m   MustRun: %v  Overdue: %v", c.MustRun(nowFuture5m), c.timeOverdue(nowFuture5m))
	t.Logf("future 35m  MustRun: %v  Overdue: %v", c.MustRun(nowFuture35m), c.timeOverdue(nowFuture35m))
	t.Logf("future 5h   MustRun: %v  Overdue: %v", c.MustRun(nowFuture5h), c.timeOverdue(nowFuture5h))
	if c.MustRun(nowPast5h) || c.MustRun(nowPast5m) || c.MustRun(nowPresent) || c.MustRun(nowFuture5m) || !c.MustRun(nowFuture35m) || !c.MustRun(nowFuture5h) {
		t.Errorf("MustRun incorrect")
	}

	// Test command sorting
	cmd := []*Command{}
	add := func(name string, interval time.Duration, lastRun time.Time) *Command {
		c := &Command{
			Enabled:  true,
			Interval: interval,
			Name:     name,
			lastRun:  lastRun,
		}
		cmd = append(cmd, c)
		return c
	}
	backup := add("daily backup", 24*time.Hour, nowPresent.Add(-24*time.Hour))
	backup.SetStartTime(nowPresent.Hour(), nowPresent.Minute()-3)
	add("15 minute A", 15*time.Minute, nowPresent.Add(-30*time.Minute))
	add("15 minute B", 15*time.Minute, nowPresent.Add(-20*time.Minute))
	add("15 minute C", 15*time.Minute, nowPresent.Add(-10*time.Minute))
	add("15 minute D", 15*time.Minute, nowPresent.Add(-99*time.Minute))
	expected := []string{
		"daily backup",
		"15 minute D",
		"15 minute A",
		"15 minute B",
	}
	runNext := func(num int) *Command {
		next := NextRunnable(cmd, nowPresent)
		t.Logf("Next: %v", next)
		if next != nil {
			next.lastRun = nowPresent
			if next.Name != expected[num] {
				t.Fatalf("Ordering incorrect")
			}
		}
		return next
	}
	for i := 0; true; i++ {
		if runNext(i) == nil {
			break
		}
	}
}

func TestPools(t *testing.T) {
	// To run a kind of "integration test" for the pools, replace the list of commands inside scheduler.go with this list:
	//add(true, "sleep1", "poolA", 1*time.Second, 12*hour, `C:\Program Files (x86)\Git\bin\sleep.exe`, "10")
	//add(true, "sleep2", "poolA", 1*time.Second, 12*hour, `C:\Program Files (x86)\Git\bin\sleep.exe`, "10")
	//add(true, "sleep3", "poolA", 1*time.Second, 12*hour, `C:\Program Files (x86)\Git\bin\sleep.exe`, "10")
	//add(true, "sleep4", "poolB", 1*time.Second, 12*hour, `C:\Program Files (x86)\Git\bin\sleep.exe`, "1")
	// You'll see that sleep4 runs often, but sleep1-3 alternate between themselves.
	// You also need to change "time.Sleep(5 * time.Second)"" inside run() from 5 to 1 second.

	loc := time.FixedZone("Pretoria", -7200)
	nowPresent := time.Date(2015, 07, 15, 5, 3, 20, 0, loc)

	cmd := []*Command{}
	add := func(name, pool string, lastRun time.Time) *Command {
		c := &Command{
			Enabled:  true,
			Interval: 1 * time.Minute,
			Name:     name,
			Pool:     pool,
			lastRun:  lastRun,
		}
		cmd = append(cmd, c)
		return c
	}
	// The only reason we vary lastRun time is to get predictable priority sorting when all
	// other things are equal
	c_a := add("a", "import_pool", nowPresent.Add(-15*time.Hour))
	add("b", "import_pool", nowPresent.Add(-14*time.Hour))
	c_c := add("c", "update_pool", nowPresent.Add(-13*time.Hour))
	c_d := add("d", "update_pool", nowPresent.Add(-12*time.Hour))

	// import_pool is busy
	c_a.isRunningAtomic = 1
	if next := NextRunnable(cmd, nowPresent); next != c_c {
		t.Fatal("Pools not respected, or ordering incorrect (1)")
	}

	// update_pool is busy
	c_a.isRunningAtomic = 0
	c_d.isRunningAtomic = 1
	if next := NextRunnable(cmd, nowPresent); next != c_a {
		t.Fatal("Pools not respected, or ordering incorrect (2)")
	}

	// both pools busy
	c_a.isRunningAtomic = 1
	if next := NextRunnable(cmd, nowPresent); next != nil {
		t.Fatal("Pools not respected, or ordering incorrect (3)")
	}
}
