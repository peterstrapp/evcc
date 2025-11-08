package ocpp

import (
	"encoding/json"
	"fmt"

	"github.com/evcc-io/evcc/meter"

	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/security"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/types"
)

// cp actions

func (cs *CS) OnAuthorize(id string, request *core.AuthorizeRequest) (*core.AuthorizeConfirmation, error) {
	// no cp handler

	res := &core.AuthorizeConfirmation{
		IdTagInfo: &types.IdTagInfo{
			Status: types.AuthorizationStatusAccepted,
		},
	}

	return res, nil
}

func (cs *CS) OnBootNotification(id string, request *core.BootNotificationRequest) (*core.BootNotificationConfirmation, error) {
	if cp, err := cs.ChargepointByID(id); err == nil {
		return cp.OnBootNotification(request)
	}

	res := &core.BootNotificationConfirmation{
		CurrentTime: types.Now(),
		Interval:    int(Timeout.Seconds()),
		Status:      core.RegistrationStatusPending, // not accepted during startup
	}

	return res, nil
}

func (cs *CS) OnDataTransfer(id string, request *core.DataTransferRequest) (*core.DataTransferConfirmation, error) {
	// handle known vendor messages
	if request != nil {
		cs.log.DEBUG.Printf("DataTransfer from %s: vendorId=%s messageId=%s data=%v", id, request.VendorId, request.MessageId, request.Data)

		// Example payload from MasterPlug:
		// {"vendorId":"MasterPlug","messageId":"GetCTClampValue","data":"{\"current\":4110,\"voltage\":249700}"}
		if request.VendorId == "MasterPlug" && request.MessageId == "GetCTClampValue" {
			s, _ := request.Data.(string)
			var inner json.RawMessage = json.RawMessage(s)
			if cur, volt, err := meterParseMasterplug(inner); err == nil {
				cs.log.DEBUG.Printf("parsed MasterPlug values from %s: current=%f, voltage=%f", id, cur, volt)
				meter.Update(id, cur, volt)
			} else {
				cs.log.WARN.Printf("failed to parse MasterPlug payload from %s: %v", id, err)
			}
		}
	}

	res := &core.DataTransferConfirmation{
		Status: core.DataTransferStatusAccepted,
	}

	return res, nil
}

// helper to parse MasterPlug payload where values may be provided in mA/mV
func meterParseMasterplug(data json.RawMessage) (float64, float64, error) {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return 0, 0, err
	}

	var curVal float64
	var voltVal float64

	if v, ok := obj["current"]; ok {
		switch t := v.(type) {
		case float64:
			curVal = t
		case string:
			// try parse numeric string
			var tmp float64
			if err := json.Unmarshal([]byte("\""+t+"\""), &tmp); err == nil {
				curVal = tmp
			}
		}
	}
	if v, ok := obj["voltage"]; ok {
		switch t := v.(type) {
		case float64:
			voltVal = t
		case string:
			var tmp float64
			if err := json.Unmarshal([]byte("\""+t+"\""), &tmp); err == nil {
				voltVal = tmp
			}
		}
	}

	if curVal == 0 && voltVal == 0 {
		return 0, 0, fmt.Errorf("no values")
	}

	// The device reports current in mA and voltage in mV in the example. Convert to A and V.
	// If values appear already in A/V, these conversions will produce very small numbers, but this is a best-effort heuristic.
	// Heuristic: if current > 100 (likely mA) divide by 1000; if voltage > 1000 (likely mV) divide by 1000.
	if curVal > 100 {
		curVal = curVal / 1000.0
	}
	if voltVal > 1000 {
		voltVal = voltVal / 1000.0
	}

	return curVal, voltVal, nil
}

func (cs *CS) OnHeartbeat(id string, request *core.HeartbeatRequest) (*core.HeartbeatConfirmation, error) {
	// no cp handler

	res := &core.HeartbeatConfirmation{
		CurrentTime: types.Now(),
	}

	return res, nil
}

func (cs *CS) OnMeterValues(id string, request *core.MeterValuesRequest) (*core.MeterValuesConfirmation, error) {
	if cp, err := cs.ChargepointByID(id); err == nil {
		return cp.OnMeterValues(request)
	}

	return new(core.MeterValuesConfirmation), nil
}

func (cs *CS) OnStatusNotification(id string, request *core.StatusNotificationRequest) (*core.StatusNotificationConfirmation, error) {
	cs.mu.Lock()
	// cache status for future cp connection
	if reg, ok := cs.regs[id]; ok && request != nil {
		reg.mu.Lock()
		reg.status[request.ConnectorId] = request
		reg.mu.Unlock()
	}
	cs.mu.Unlock()

	if cp, err := cs.ChargepointByID(id); err == nil {
		return cp.OnStatusNotification(request)
	}

	return new(core.StatusNotificationConfirmation), nil
}

func (cs *CS) OnStartTransaction(id string, request *core.StartTransactionRequest) (*core.StartTransactionConfirmation, error) {
	if cp, err := cs.ChargepointByID(id); err == nil {
		return cp.OnStartTransaction(request)
	}

	res := &core.StartTransactionConfirmation{
		IdTagInfo: &types.IdTagInfo{
			Status: types.AuthorizationStatusAccepted,
		},
	}

	return res, nil
}

func (cs *CS) OnStopTransaction(id string, request *core.StopTransactionRequest) (*core.StopTransactionConfirmation, error) {
	if cp, err := cs.ChargepointByID(id); err == nil {
		cp.OnStopTransaction(request)
	}

	res := &core.StopTransactionConfirmation{
		IdTagInfo: &types.IdTagInfo{
			Status: types.AuthorizationStatusAccepted, // accept old pending stop message during startup
		},
	}

	return res, nil
}

func (cs *CS) OnSecurityEventNotification(id string, request *security.SecurityEventNotificationRequest) (*security.SecurityEventNotificationResponse, error) {
	// Acknowledge any security event
	return &security.SecurityEventNotificationResponse{}, nil
}

func (cs *CS) OnSignCertificate(id string, request *security.SignCertificateRequest) (*security.SignCertificateResponse, error) {
	// Reject any certificate signing request
	return &security.SignCertificateResponse{
		Status: types.GenericStatusRejected,
	}, nil
}

func (cs *CS) OnCertificateSigned(id string, request *security.CertificateSignedRequest) (*security.CertificateSignedResponse, error) {
	// Acknowledge any certificate
	return &security.CertificateSignedResponse{
		Status: security.CertificateSignedStatusAccepted,
	}, nil
}
