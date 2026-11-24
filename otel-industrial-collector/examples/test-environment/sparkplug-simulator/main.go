// Command sparkplug-simulator is a small Sparkplug B edge node for the test
// environment. It connects to the MQTT broker that the collector's sparkplug
// receiver runs, publishes NBIRTH and DBIRTH, then a DDATA update for one
// device every few seconds. If the connection drops, it reconnects and sends
// new births, as Sparkplug requires.
//
// It is built in the Dockerfile against the same sparkplug commit as the
// OCB manifests; it has no go.mod in this repository.
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/jmacd/opentelemetry-mqtt-sparkplug/sparkplug"
	sparkproto "github.com/jmacd/opentelemetry-mqtt-sparkplug/sparkplug/proto"
	"github.com/jmacd/opentelemetry-mqtt-sparkplug/sparkplug/sparkplugclient"
)

// Sparkplug IDs of the simulated edge node and its one device.
const (
	groupID  = "demo-group"
	nodeID   = "demo-node"
	deviceID = "demo-conveyor"
)

var (
	broker   = flag.String("broker", "tcp://gateway:1883", "MQTT broker URL: the collector's sparkplug receiver")
	interval = flag.Duration("interval", 5*time.Second, "time between DDATA messages")
)

func main() {
	flag.Parse()
	log.SetPrefix("sparkplug-simulator: ")

	c := newConveyor()
	// bdSeq counts sessions; every session starts with new births.
	for bdSeq := uint32(0); ; bdSeq++ {
		err := session(bdSeq, c)
		log.Printf("%v; reconnecting in 2s", err)
		time.Sleep(2 * time.Second)
	}
}

// session connects to the broker, publishes NBIRTH and DBIRTH, then DDATA
// every interval until the connection is lost.
func session(bdSeq uint32, c *conveyor) error {
	lost := make(chan error, 1)
	opts := sparkplugclient.NewOptions()
	opts.AddBroker(*broker).
		SetClientID(nodeID).
		SetCleanSession(true).
		SetConnectionLostHandler(func(_ pahomqtt.Client, err error) {
			select {
			case lost <- err:
			default:
			}
		})

	client := sparkplugclient.NewClient(opts)
	if err := wait(client.Connect()); err != nil {
		return fmt.Errorf("connect to %s: %w", *broker, err)
	}
	defer client.Disconnect(250)
	log.Printf("connected to %s as %s/%s", *broker, groupID, nodeID)

	var seq uint64
	publish := func(topic sparkplug.Topic, metrics ...*sparkproto.Payload_Metric) error {
		now := uint64(time.Now().UnixMilli())
		for _, m := range metrics {
			m.Timestamp = &now
		}
		s := seq
		seq = (seq + 1) % 256 // Sparkplug sequence numbers wrap at 256
		return wait(client.PublishSparkplug(topic, 1, false, &sparkproto.Payload{
			Timestamp: &now,
			Seq:       &s,
			Metrics:   metrics,
		}))
	}

	if err := publish(sparkplug.NewTopic(groupID, sparkplug.NBIRTH, nodeID, ""),
		intMetric("bdSeq", bdSeq),
		stringMetric("Node Properties/Description", "OpenTelemetry Industrial demo edge node"),
	); err != nil {
		return fmt.Errorf("NBIRTH: %w", err)
	}
	if err := publish(sparkplug.NewTopic(groupID, sparkplug.DBIRTH, nodeID, deviceID), c.metrics()...); err != nil {
		return fmt.Errorf("DBIRTH: %w", err)
	}
	log.Printf("published NBIRTH and DBIRTH for device %s", deviceID)

	ddata := sparkplug.NewTopic(groupID, sparkplug.DDATA, nodeID, deviceID)
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()
	for {
		select {
		case err := <-lost:
			return fmt.Errorf("connection lost: %w", err)
		case <-ticker.C:
			c.step(*interval)
			if err := publish(ddata, c.metrics()...); err != nil {
				return fmt.Errorf("DDATA: %w", err)
			}
		}
	}
}

// conveyor is the simulated device: a conveyor motor and a parts counter.
type conveyor struct {
	start              time.Time
	speed, temperature float64
	parts              uint32
}

func newConveyor() *conveyor {
	return &conveyor{start: time.Now(), speed: 1450, temperature: 55}
}

// step advances the simulation: speed (rpm) and temperature (°C) drift slowly
// around their set points, and a few parts come off the line every interval.
func (c *conveyor) step(d time.Duration) {
	t := time.Since(c.start).Seconds()
	c.speed = 1450 + 40*math.Sin(t/90) + rand.NormFloat64()*5
	c.temperature = 55 + 6*math.Sin(t/600) + rand.NormFloat64()*0.3
	c.parts += uint32(d.Seconds()/2) + uint32(rand.Intn(2))
}

func (c *conveyor) metrics() []*sparkproto.Payload_Metric {
	return []*sparkproto.Payload_Metric{
		floatMetric("motor_speed", c.speed),
		floatMetric("motor_temperature", c.temperature),
		intMetric("parts_produced", c.parts),
	}
}

func floatMetric(name string, v float64) *sparkproto.Payload_Metric {
	return &sparkproto.Payload_Metric{Name: &name, Value: &sparkproto.Payload_Metric_FloatValue{FloatValue: float32(v)}}
}

func intMetric(name string, v uint32) *sparkproto.Payload_Metric {
	return &sparkproto.Payload_Metric{Name: &name, Value: &sparkproto.Payload_Metric_IntValue{IntValue: v}}
}

func stringMetric(name, v string) *sparkproto.Payload_Metric {
	return &sparkproto.Payload_Metric{Name: &name, Value: &sparkproto.Payload_Metric_StringValue{StringValue: v}}
}

// wait waits for an MQTT operation to finish.
func wait(t pahomqtt.Token) error {
	if !t.WaitTimeout(5 * time.Second) {
		return errors.New("timed out")
	}
	return t.Error()
}
