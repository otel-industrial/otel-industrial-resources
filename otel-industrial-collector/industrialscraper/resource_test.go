package industrialscraper

import (
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestResourceAttributesSetInto(t *testing.T) {
	m := pcommon.NewMap()
	ResourceAttributes{
		DeviceID:     "plc-1",
		PLCIPAddress: "192.168.1.10",
		// FacilityLine and EquipmentType left empty on purpose.
	}.SetInto(m)

	want := map[string]string{
		AttributeDeviceID:     "plc-1",
		AttributePLCIPAddress: "192.168.1.10",
	}
	if m.Len() != len(want) {
		t.Fatalf("map has %d entries, want %d (empty fields must be skipped)", m.Len(), len(want))
	}
	for k, v := range want {
		got, ok := m.Get(k)
		if !ok || got.Str() != v {
			t.Errorf("%s = %q (present=%v), want %q", k, got.Str(), ok, v)
		}
	}
}
