package ocpp

import (
	"net/http"
	"sync"
	"time"

	"github.com/evcc-io/evcc/util"
	"github.com/lorenzodonini/ocpp-go/ocpp"
	ocpp16 "github.com/lorenzodonini/ocpp-go/ocpp1.6"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/remotetrigger"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/security"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/smartcharging"
	"github.com/lorenzodonini/ocpp-go/ocppj"
	"github.com/lorenzodonini/ocpp-go/ws"
)

var (
	instanceMu sync.Mutex
	instance   *CS
)

// Start creates and starts the OCPP central system. If already started, returns the existing instance.
func Start() (*CS, error) {
	instanceMu.Lock()
	defer instanceMu.Unlock()

	if instance != nil {
		return instance, nil
	}

	log := util.NewLogger("ocpp")

	server := ws.NewServer()
	server.SetCheckOriginHandler(func(r *http.Request) bool { return true })

	dispatcher := ocppj.NewDefaultServerDispatcher(ocppj.NewFIFOQueueMap(0))
	dispatcher.SetTimeout(Timeout)

	endpoint := ocppj.NewServer(server, dispatcher, nil, core.Profile, remotetrigger.Profile, smartcharging.Profile, security.Profile)
	endpoint.SetInvalidMessageHook(func(client ws.Channel, err *ocpp.Error, rawMessage string, parsedFields []any) *ocpp.Error {
		log.ERROR.Printf("%v (%s)", err, rawMessage)
		return nil
	})

	cs := ocpp16.NewCentralSystem(endpoint, server)

	inst := &CS{
		log:           log,
		regs:          make(map[string]*registration),
		CentralSystem: cs,
		wsServer:      server,
		dispatcher:    dispatcher,
		endpoint:      endpoint,
	}

	inst.txnId.Store(time.Now().UTC().Unix())

	ocppj.SetLogger(inst)

	cs.SetCoreHandler(inst)
	cs.SetSecurityHandler(inst)
	cs.SetNewChargePointHandler(inst.NewChargePoint)
	cs.SetChargePointDisconnectedHandler(inst.ChargePointDisconnected)

	// publish instance early so other goroutines that call Instance() don't get nil
	instance = inst

	go inst.errorHandler(cs.Errors())
	go cs.Start(8887, "/{ws}")

	// wait for server to start
	for range time.Tick(10 * time.Millisecond) {
		if dispatcher.IsRunning() {
			break
		}
	}

	return instance, nil
}

// Stop stops the running OCPP central system if any.
func Stop() error {
	instanceMu.Lock()
	inst := instance
	instanceMu.Unlock()

	if inst == nil || inst.CentralSystem == nil {
		return nil
	}

	// stop central system
	inst.CentralSystem.Stop()

	// wait for dispatcher to stop
	for i := 0; i < 200; i++ {
		if inst.dispatcher == nil || !inst.dispatcher.IsRunning() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	instanceMu.Lock()
	instance = nil
	instanceMu.Unlock()

	return nil
}

// Restart stops and starts the OCPP central system.
func Restart() error {
	if err := Stop(); err != nil {
		return err
	}
	_, err := Start()
	return err
}

// Instance returns the current instance if started, otherwise nil.
func Instance() *CS {
	inst, _ := Start()
	return inst
}
