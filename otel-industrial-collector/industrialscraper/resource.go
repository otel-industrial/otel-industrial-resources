package industrialscraper

import "go.opentelemetry.io/collector/pdata/pcommon"

// Canonical industrial resource-attribute keys; provisional pending #11.
const (
	AttributeDeviceID      = "device.id"
	AttributePLCIPAddress  = "plc.ip_address"
	AttributeFacilityLine  = "industrial.facility.line"
	AttributeEquipmentType = "industrial.equipment.type"
)

// ResourceAttributes holds industrial resource attributes shared across
// receivers.
type ResourceAttributes struct {
	DeviceID      string
	PLCIPAddress  string
	FacilityLine  string
	EquipmentType string
}

// SetInto writes each non-empty field to m under its canonical key.
func (a ResourceAttributes) SetInto(m pcommon.Map) {
	put := func(k, v string) {
		if v != "" {
			m.PutStr(k, v)
		}
	}
	put(AttributeDeviceID, a.DeviceID)
	put(AttributePLCIPAddress, a.PLCIPAddress)
	put(AttributeFacilityLine, a.FacilityLine)
	put(AttributeEquipmentType, a.EquipmentType)
}
