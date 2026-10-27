package whatsmeow_service

import (
	"fmt"
	"sync"
	"time"
)

// Each worker owns a durable stop signal and an exit acknowledgement. These
// belong to a worker generation, not to the replaceable legacy channel map.
type clientWorker struct {
	stop chan struct{}
	done chan struct{}
	once sync.Once
}

func (w whatsmeowService) claimClientWorker(id string) (*clientWorker, bool) {
	worker := &clientWorker{stop: make(chan struct{}), done: make(chan struct{})}
	_, active := w.activeClients.LoadOrStore(id, worker)
	return worker, !active
}

func (w whatsmeowService) finishClientWorker(id string, worker *clientWorker) {
	w.activeClients.CompareAndDelete(id, worker)
	close(worker.done)
}

func (w whatsmeowService) runClientWorker(id string, run func(<-chan struct{})) bool {
	worker, claimed := w.claimClientWorker(id)
	if !claimed {
		return false
	}
	defer w.finishClientWorker(id, worker)
	run(worker.stop)
	return true
}

func (w whatsmeowService) stopClientWorker(id string, timeout time.Duration) error {
	value, active := w.activeClients.Load(id)
	if !active {
		return nil
	}
	worker := value.(*clientWorker)
	worker.once.Do(func() { close(worker.stop) })
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-worker.done:
		return nil
	case <-timer.C:
		return fmt.Errorf("client worker did not stop before reconnect deadline")
	}
}
