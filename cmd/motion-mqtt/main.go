package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	rpio "github.com/stianeikeland/go-rpio/v4"
)

func connect(clientID string, uri *url.URL) (mqtt.Client, error) {
	var opts = mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s", uri.Host))
	opts.SetUsername(uri.User.Username())
	password, _ := uri.User.Password()
	opts.SetPassword(password)
	opts.SetClientID(clientID)
	opts.CleanSession = false

	var client = mqtt.NewClient(opts)
	var token = client.Connect()
	for !token.WaitTimeout(3 * time.Second) {
	}
	return client, token.Error()
}

func main() {
	log.SetFlags(log.Lshortfile)

	var sensorGpioPin = flag.Int("gpioPin", 26, "gpio pin for the PIR sensor")
	var brokerURI = flag.String("brokerURI", "mqtt://127.0.0.1:1883", "URI of the MQTT broker")
	var clientID = flag.String("clientID", "motion-mqtt", "client ID for MQTT")
	var topic = flag.String("topic", "motion-mqtt", "MQTT topic to publish")

	flag.Parse()

	err := rpio.Open()
	if err != nil {
		log.Fatal(err)
	}

	mqttURI, err := url.Parse(*brokerURI)
	if err != nil {
		log.Fatal(err)
	}

	client, err := connect(*clientID, mqttURI)
	if err != nil {
		log.Fatal(err)
	}

	rpioPin := rpio.Pin(*sensorGpioPin)
	rpioPin.Input()

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		log.Println("cleanup")
		rpio.Close()
		os.Exit(1)
	}()

	log.Println("Started... waiting for sensor detection")

	// Button press loop
	var previousRpioState rpio.State
	for {
		var rpioState = rpioPin.Read()
		if previousRpioState == rpioState {
			continue
		}

		var message = ""

		switch rpioState {
		case rpio.High:
			log.Println("Sensor detected!", time.Now().Format(time.RFC1123Z))
			message = "ON"
			break
		case rpio.Low:
			message = "OFF"
			break
		}

		token := client.Publish(*topic, 0, true, message)
		if token.Error() != nil {
			log.Println(token.Error())
		}
		previousRpioState = rpioState
		time.Sleep(time.Millisecond * 100)
	}
}
