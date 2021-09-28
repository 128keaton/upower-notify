package main

import (
	"os"
	"flag"
	"fmt"
	"log"
        "gopkg.in/ini.v1"
	"time"
	"github.com/128keaton/upower-notify/notify"
	"github.com/128keaton/upower-notify/upower"
)

func init() {}

var (
	tick               time.Duration
	warn               time.Duration
	critical           time.Duration
	notificationExpiry time.Duration
	device             string
	report             bool

	notificationExpiryMilliseconds int32
	notifyStartMessage  string
	notifyStartTitle    string
)

func main() {
	dirname, err := os.UserHomeDir()
	cfg, err := ini.Load(dirname + "/.config/upower-notify/config.ini")
    	if err != nil {
        	fmt.Printf("Fail to read file: %v", err)
        	os.Exit(1)
    	}

	notifyStartTitle := cfg.Section("startup").Key("notification_title").Validate(func(in string) string {
	    	if len(in) == 0 {
        		return "All good!"
    		}
    		return in
	})

	notifyStartMessage := cfg.Section("startup").Key("notification_message").Validate(func(in string) {
                if len(in) == 0 {
                        return "upower-notify has started and is ready to serve!"
                }
                return in
        })

	flag.DurationVar(&tick, "tick", 10*time.Second, "Update rate")
	flag.DurationVar(&warn, "warn", 20*time.Minute, "Time to start warning. (Warn)")
	flag.DurationVar(&critical, "critical", 10*time.Minute, "Time to start warning. (Critical)")
	flag.DurationVar(&notificationExpiry, "notification-expiration", 10*time.Second, "Notifications expiry duration")
	flag.StringVar(&device, "device", "DisplayDevice", "DBus device name for the battery")
	flag.BoolVar(&report, "report", false, "Print out updates to stdout.")
	flag.Parse()

	notificationExpiryMilliseconds = int32(notificationExpiry / time.Millisecond)
	up, err := upower.New(device)

	if err != nil {
		log.Fatal(err)
	}

	update, err := up.Get()
	if err != nil {
		log.Fatal(err)
	}

	notifier, err := notify.New("Upower Agent")
	if err != nil {
		log.Fatal(err)
	}

	err = notifier.Low(notifyStartTitle, notifyStartMessage, notificationExpiryMilliseconds)
	if err != nil {
		log.Fatal(err)
	}

	var old upower.Update

	for range time.Tick(tick) {
		update, err = up.Get()
		if err != nil {
			notifier.Critical("Oh Noosss!", fmt.Sprintf("Something went wrong: %s", err), notificationExpiryMilliseconds)
			fmt.Printf("ERROR!!")
		}
		if update.Changed(old) {
			sendNotify(update, notifier, old.State != update.State)
			if report {
				print(update, notifier)
			}
		}
		old = update
	}
}

func print(battery upower.Update, notifier *notify.Notifier) {
	switch battery.State {
	case upower.Charging:
		fmt.Printf("C(%v%%):%v\n", battery.Percentage, battery.TimeToFull)
	case upower.Discharging:
		fmt.Printf("D(%v%%):%v\n", battery.Percentage, battery.TimeToEmpty)
	case upower.Empty:
		fmt.Printf("DEAD!\n")
	case upower.FullCharged:
		fmt.Printf("F:%v%%\n", battery.Percentage)
	case upower.PendingCharge:
		fmt.Printf("PC\n")
	case upower.PendingDischarge:
		fmt.Printf("PD\n")
	default:
		fmt.Printf("UNKN(%v)", battery.State)
	}
}

func sendNotify(battery upower.Update, notifier *notify.Notifier, changed bool) {
	if changed {
		notifier.Normal("Power Change.", fmt.Sprintf("Heads up!! We are now %s.", battery.State), notificationExpiryMilliseconds)
	}

	switch battery.State {
	case upower.Charging:
		//Do nothing.
	case upower.Discharging:
		switch {
		case battery.TimeToEmpty < critical:
			notifier.Critical("BATTERY LOW!", fmt.Sprintf("Things are getting critical here. %s to go.", battery.TimeToEmpty), notificationExpiryMilliseconds)
			time.Sleep(critical / 10)
		case battery.TimeToEmpty < warn:
			notifier.Normal("Heads up!!", fmt.Sprintf("We only got %s of juice. Any powerpoints around?", battery.TimeToEmpty), notificationExpiryMilliseconds)
			time.Sleep(warn / 10)
		default:
			//Do nothing. Everything seems good.
		}
	case upower.Empty:
		notifier.Critical("BATTERY DEAD!", fmt.Sprintf("Things are pretty bad. Battery is dead. %s to go.", battery.TimeToEmpty), notificationExpiryMilliseconds)
	case upower.FullCharged, upower.PendingCharge, upower.PendingDischarge:
		//Do nothing.
	default:
		notifier.Critical("Oh Noosss!", "I can't figure out battery state!", notificationExpiryMilliseconds)
	}
}
