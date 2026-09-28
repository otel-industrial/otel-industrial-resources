// cmd/simulator/main.go
// A simple Modbus TCP simulator that serves fake register data for local testing.
// Simulates a device with temperature, pressure, humidity (holding registers)
// and pump/valve status (coils), with values that drift over time.
package main

import (
	"flag"
	"log"
	"math"
	"math/rand"
	"time"

	"github.com/tbrandon/mbserver"
)

func main() {
	addr := flag.String("addr", "localhost:502", "Modbus TCP listen address")
	flag.Parse()

	serv := mbserver.NewServer()

	// Register map (mbserver stores registers as []uint16):
	//   0-1  float32  temperature  (°C)   ~22.5
	//   2    uint16   pressure     (hPa)  ~1013
	//   3    uint16   humidity     (%)    ~55
	//   4-5  float32  flow_rate    (L/m)  ~12.3
	//
	// Coil map:
	//   0    bool     pump_status   (1 = running)
	//   1    bool     valve_status  (1 = open)

	setFloat32(serv, 0, 22.5)
	setUint16(serv, 2, 1013)
	setUint16(serv, 3, 55)
	setFloat32(serv, 4, 12.3)

	serv.Coils[0] = 1 // pump running
	serv.Coils[1] = 1 // valve open

	if err := serv.ListenTCP(*addr); err != nil {
		log.Fatalf("failed to start Modbus simulator: %v", err)
	}
	defer serv.Close()

	log.Printf("Modbus TCP simulator listening on %s", *addr)
	log.Printf("Register map:")
	log.Printf("  holding[0-1]  float32  temperature (°C)")
	log.Printf("  holding[2]    uint16   pressure (hPa)")
	log.Printf("  holding[3]    uint16   humidity (%%)")
	log.Printf("  holding[4-5]  float32  flow_rate (L/min)")
	log.Printf("  coil[0]       bool     pump_status")
	log.Printf("  coil[1]       bool     valve_status")

	// Drift values every 2 seconds to simulate a live device.
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	t := 0.0
	for range ticker.C {
		t += 0.1

		temp := 22.5 + 2.0*math.Sin(t) + (rand.Float64()-0.5)*0.2
		setFloat32(serv, 0, float32(temp))

		pressure := 1013.0 + 5.0*math.Sin(t*0.5) + (rand.Float64()-0.5)*1.0
		setUint16(serv, 2, uint16(pressure))

		humidity := 55.0 + 10.0*math.Sin(t*0.3) + (rand.Float64()-0.5)*0.5
		setUint16(serv, 3, uint16(humidity))

		flow := 12.3 + 3.0*math.Sin(t*0.7) + (rand.Float64()-0.5)*0.3
		setFloat32(serv, 4, float32(flow))

		if int(t*10)%150 == 0 {
			if serv.Coils[0] == 1 {
				serv.Coils[0] = 0
				log.Printf("pump_status → OFF")
			} else {
				serv.Coils[0] = 1
				log.Printf("pump_status → ON")
			}
		}

		log.Printf("temp=%.2f°C  pressure=%d hPa  humidity=%d%%  flow=%.2f L/min  pump=%d",
			temp, uint16(pressure), uint16(humidity), flow, serv.Coils[0])
	}
}

// setFloat32 encodes a float32 as two holding registers (ABCD byte order).
// mbserver stores registers as []uint16, each register holds 16 bits.
func setFloat32(serv *mbserver.Server, addr int, val float32) {
	bits := math.Float32bits(val)
	serv.HoldingRegisters[addr] = uint16(bits >> 16)
	serv.HoldingRegisters[addr+1] = uint16(bits & 0xFFFF)
}

// setUint16 encodes a uint16 as one holding register.
func setUint16(serv *mbserver.Server, addr int, val uint16) {
	serv.HoldingRegisters[addr] = val
}