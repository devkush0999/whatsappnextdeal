package whatsmeow_service

import (
	"sync"
	"testing"
	"time"

	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	instance_repository "github.com/evolution-foundation/evolution-go/pkg/instance/repository"
)

func workerService() whatsmeowService { return whatsmeowService{activeClients: &sync.Map{}} }
func waitWorker(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not progress")
	}
}

func TestWorkerReconnectWaitsForExitAndAllowsNewGeneration(t *testing.T) {
	w := workerService()
	started, stopped, release, exited := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(exited)
		w.runClientWorker("sender", func(stop <-chan struct{}) {
			close(started)
			<-stop
			close(stopped)
			<-release
		})
	}()
	waitWorker(t, started)
	recovery := make(chan error, 1)
	go func() { recovery <- w.stopClientWorker("sender", time.Second) }()
	waitWorker(t, stopped)
	if w.runClientWorker("sender", func(<-chan struct{}) { t.Error("overlapping client admitted") }) {
		t.Fatal("old worker still owns sender")
	}
	select {
	case <-recovery:
		t.Fatal("stop acknowledged before worker exit")
	default:
	}
	close(release)
	if err := <-recovery; err != nil {
		t.Fatal(err)
	}
	waitWorker(t, exited)
	ran := false
	if !w.runClientWorker("sender", func(<-chan struct{}) { ran = true }) || !ran {
		t.Fatal("replacement worker rejected after stop")
	}
}

func TestWorkerTimeoutPreservesAdmissionUntilExit(t *testing.T) {
	w := workerService()
	worker, ok := w.claimClientWorker("sender")
	if !ok {
		t.Fatal("initial claim failed")
	}
	if err := w.stopClientWorker("sender", time.Millisecond); err == nil {
		t.Fatal("unacknowledged stop must time out")
	}
	if _, claimed := w.claimClientWorker("sender"); claimed {
		t.Fatal("timed-out worker must not be replaced")
	}
	select {
	case <-worker.stop:
	default:
		t.Fatal("durable stop signal lost")
	}
	w.finishClientWorker("sender", worker)
}

func TestConcurrentStopsAreIdempotentAndOtherSendersIndependent(t *testing.T) {
	w := workerService()
	worker, _ := w.claimClientWorker("sender")
	other, claimed := w.claimClientWorker("other")
	if !claimed {
		t.Fatal("independent sender blocked")
	}
	var stops sync.WaitGroup
	for n := 0; n < 8; n++ {
		stops.Add(1)
		go func() {
			defer stops.Done()
			if err := w.stopClientWorker("sender", time.Second); err != nil {
				t.Error(err)
			}
		}()
	}
	waitWorker(t, worker.stop)
	w.finishClientWorker("sender", worker)
	stops.Wait()
	select {
	case <-other.stop:
		t.Fatal("another sender stopped")
	default:
	}
	w.finishClientWorker("other", other)
	if err := w.stopClientWorker("missing", time.Second); err != nil {
		t.Fatal(err)
	}
}

type workerRepository struct {
	instance_repository.InstanceRepository
}

func (workerRepository) GetInstanceByID(id string) (*instance_model.Instance, error) {
	return &instance_model.Instance{Id: id}, nil
}

func TestStartInstanceDoesNotReplaceActiveWorkerChannels(t *testing.T) {
	w := workerService()
	w.instanceRepository = workerRepository{}
	original := make(chan bool)
	w.killChannel = map[string]chan bool{"sender": original}
	// Admission is reserved synchronously before the asynchronous startup begins.
	worker, _ := w.claimClientWorker("sender")
	defer w.finishClientWorker("sender", worker)
	if err := w.StartInstance("sender"); err != nil {
		t.Fatal(err)
	}
	if w.killChannel["sender"] != original {
		t.Fatal("active generation's channel replaced")
	}
}
