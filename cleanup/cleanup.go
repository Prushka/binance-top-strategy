package cleanup

import (
	"os"
	"os/signal"
	"sync"
	"syscall"

	log "github.com/sirupsen/logrus"
)

const (
	Scheduler = iota
	Persistence
)

type OnStop func(sig os.Signal)

type stop struct {
	isStopping bool
	mutex      sync.Mutex
	onStopFunc map[int]OnStop
}

var quitInstance = &stop{
	isStopping: false,
	onStopFunc: make(map[int]OnStop),
}

func AddOnStopFunc(key int, f OnStop) {
	quitInstance.mutex.Lock()
	defer quitInstance.mutex.Unlock()
	quitInstance.onStopFunc[key] = f
	if quitInstance.isStopping {
		f(syscall.SIGTERM)
	}
}

func Stop(sig os.Signal) {
	quitInstance.mutex.Lock()
	defer quitInstance.mutex.Unlock()
	quitInstance.isStopping = true
	log.Warnf("Received signal %d, terminating...", sig)
	for k, f := range quitInstance.onStopFunc {
		f(sig)
		delete(quitInstance.onStopFunc, k)
	}
}

func InitSignalCallback(blocking chan bool) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	go func() {
		sig := <-sigChan
		Stop(sig)
		blocking <- true
	}()
}
