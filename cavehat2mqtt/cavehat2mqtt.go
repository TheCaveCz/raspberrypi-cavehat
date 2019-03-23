package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"strconv"
	"syscall"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	ws2811 "github.com/rpi-ws281x/rpi-ws281x-go"
)

var neopixelDev ws2811.WS2811

type neopixel struct {
	Count uint8
	Led   [8]led
}

type led struct {
	Red   uint8
	Green uint8
	Blue  uint8
}

var leds neopixel

var mqttNeopixelCallback MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
	fmt.Printf("TOPIC: %s\n", msg.Topic())
	fmt.Printf("MSG: %s\n", msg.Payload())

	var data map[string]interface{}
	if err := json.Unmarshal(msg.Payload(), &data); err != nil {
		panic(err)
	}

	if uint8(data["Led"].(float64)) >= 0 && uint8(data["Led"].(float64)) < leds.Count {
		if reflect.TypeOf(data["Red"]) != nil {
			leds.Led[uint8(data["Led"].(float64))].Red = uint8(data["Red"].(float64))
		} else {
			leds.Led[uint8(data["Led"].(float64))].Red = 0
		}
		if reflect.TypeOf(data["Green"]) != nil {
			leds.Led[uint8(data["Led"].(float64))].Green = uint8(data["Green"].(float64))
		} else {
			leds.Led[uint8(data["Led"].(float64))].Green = 0
		}
		if reflect.TypeOf(data["Blue"]) != nil {
			leds.Led[uint8(data["Led"].(float64))].Blue = uint8(data["Blue"].(float64))
		} else {
			leds.Led[uint8(data["Led"].(float64))].Blue = 0
		}
	}

	for i := 0; i < int(leds.Count); i++ {
		neopixelDev.Leds(0)[i] = uint32(leds.Led[i].Red)*256*256 + uint32(leds.Led[i].Green)*256 + uint32(leds.Led[i].Blue)
	}
	neopixelDev.Render()

	ret, _ := json.Marshal(leds)
	token := client.Publish("cavehat2mqtt/neopixel", 0, false, ret)
	token.Wait()
}

func main() {
	leds.Count = 8
	for i := 0; i < int(leds.Count); i++ {
		leds.Led[i].Red = 0
		leds.Led[i].Green = 0
		leds.Led[i].Blue = 0
	}

	fmt.Println("CaveHat GO MQTT Device Service")

	// MQTT set options
	hostname, _ := os.Hostname()
	mqttOpt := MQTT.NewClientOptions()
	mqttOpt.AddBroker("tcp://localhost:1883")
	mqttOpt.SetClientID(hostname + strconv.Itoa(time.Now().Second()))
	mqttOpt.SetOnConnectHandler(func(client MQTT.Client) {
		if token := client.Subscribe("cavehat2mqtt/neopixel/set", 0, mqttNeopixelCallback); token.Wait() && token.Error() != nil {
			fmt.Println(token.Error())
			os.Exit(1)
		}
	})

	// MQTT connect and subscribe to topics
	c := MQTT.NewClient(mqttOpt)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	// NeoPixel set options
	neopixelOpt := ws2811.DefaultOptions
	neopixelOpt.Channels[0].Brightness = 255
	neopixelOpt.Channels[0].LedCount = int(leds.Count)

	// NeoPixel create device
	dev, _ := ws2811.MakeWS2811(&neopixelOpt)
	dev.Init()
	defer dev.Fini()
	neopixelDev = *dev

	// NeoPixel switch off all lights
	for i := 0; i < int(leds.Count); i++ {
		neopixelDev.Leds(0)[i] = 0
	}
	neopixelDev.Render()

	// Gracefully finish
	e := make(chan os.Signal, 2)
	signal.Notify(e, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-e

		for i := 0; i < int(leds.Count); i++ {
			neopixelDev.Leds(0)[i] = 0
		}
		neopixelDev.Render()

		if token := c.Unsubscribe("cavehat2mqtt/neopixel/set"); token.Wait() && token.Error() != nil {
			fmt.Println(token.Error())
			os.Exit(1)
		}
		c.Disconnect(250)
		os.Exit(0)
	}()

	// Endless loop
	select {}
}
