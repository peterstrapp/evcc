package meter

import (
	"context"
	"sync"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/util"
)

func init() {
	// register under type "ocppdatatransfer"
	registry.AddCtx("ocppdatatransfer", NewOCPPDataTransferMeter)
}

// OCPP DataTransfer based meter
type OCPPDataTransferMeter struct {
	id      string // optional charge point id to match
	mu      sync.RWMutex
	current float64 // in A
	voltage float64 // in V
}

// instances keyed by id ("" for wildcard)
var (
	instancesMu sync.RWMutex
	instances   = make(map[string]*OCPPDataTransferMeter)
)

// NewOCPPDataTransferMeter constructs meter from config.
// Config options:
// - id: optional charge point id to match; empty matches any
func NewOCPPDataTransferMeter(ctx context.Context, other map[string]any) (api.Meter, error) {
	cfg := struct {
		ID string `mapstructure:"id"`
	}{}

	if err := util.DecodeOther(other, &cfg); err != nil {
		return nil, err
	}

	m := &OCPPDataTransferMeter{id: cfg.ID}

	instancesMu.Lock()
	instances[cfg.ID] = m
	instancesMu.Unlock()

	return m, nil
}

// Update instances matching the provided chargePoint id and also any wildcard instance (id=="").
func Update(chargePoint string, current, voltage float64) {
	instancesMu.RLock()
	defer instancesMu.RUnlock()

	if m, ok := instances[chargePoint]; ok {
		m.mu.Lock()
		m.current = current
		m.voltage = voltage
		m.mu.Unlock()
	}

	if m, ok := instances[""]; ok {
		m.mu.Lock()
		m.current = current
		m.voltage = voltage
		m.mu.Unlock()
	}
}

// CurrentPower returns instantaneous power in W
func (m *OCPPDataTransferMeter) CurrentPower() (float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// simple 1-phase power: P = U * I
	return m.voltage * m.current, nil
}

// Currents returns phase currents (A)
func (m *OCPPDataTransferMeter) Currents() (float64, float64, float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current, 0, 0, nil
}

// Voltages returns phase voltages (V)
func (m *OCPPDataTransferMeter) Voltages() (float64, float64, float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.voltage, 0, 0, nil
}

// Powers returns per-phase power (W)
func (m *OCPPDataTransferMeter) Powers() (float64, float64, float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.voltage * m.current, 0, 0, nil
}
