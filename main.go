package main

import (
	"os"

	log "github.com/sirupsen/logrus"
)

func main() {
	cfg := InitConfig()

	if cfg.Common.LogDebug {
		log.SetLevel(log.DebugLevel)
	}

	logFileLocation := cfg.Common.LogFileLocation
	if logFileLocation != "" {
		f, err := os.OpenFile(logFileLocation, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			log.Errorf("error accessing the specified log file: %s", err)
		} else {
			log.SetOutput(f)
			log.Debugf("set log output to %s", logFileLocation)
		}
	} else {
		log.Debugf("log File not specified. outputting logs only to terminal")
	}

	// Uncomment this to turn logs into JSON format
	// log.SetFormatter(&log.JSONFormatter{})
	log.Infoln(" ---- ")
	log.Info("begin Evergreen dealbot!")

	CancelAllRetrievals(cfg)
	CancelAllTransfers(cfg)

	// threadManager(cfg)
}

func threadManager(cfg EvergreenDealbotConfig) {
	doneChan := make(chan bool)
	var numActiveThreads uint = 0

	go WatcherThread(cfg)

	for {
		if numActiveThreads < cfg.Common.MaxThreads {
			go DealbotThread(doneChan, cfg)
			numActiveThreads++
			log.Debugf("spawning a new thread. there are now %d active \n", numActiveThreads)
			continue
		}

		<-doneChan
		numActiveThreads--
		log.Debugf("a thread just finished. there are now %d active \n", numActiveThreads)
		continue
	}
}
