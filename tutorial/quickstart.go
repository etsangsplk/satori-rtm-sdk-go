// For tutorial purposes, we subscribe to the same channel that we publish a
// message to. So we receive our own published message. This allows end-to-end
// illustration of data flow with just a single client.
//
// ==== Before you start ====
// Make sure that you have `go` installed: https://golang.org/doc/install
// Also please make sure that your Go-Workspace is properly configured: https://golang.org/doc/code.html#Workspaces
// and your GOPATH environment variable is point to your Go-Workspace.
//
// Hint: Run `make run` from the console to make auto-configuration for the Go-Workspace
//
//   $ git clone git@github.com:satori-com/satori-rtm-sdk-go.git
//   $ cd satori-rtm-sdk-go/tutorial/
//   $ make run
//
package main

import (
	"encoding/json"
	"fmt"
	"github.com/satori-com/satori-rtm-sdk-go/rtm"
	"github.com/satori-com/satori-rtm-sdk-go/rtm/auth"
	"github.com/satori-com/satori-rtm-sdk-go/rtm/pdu"
	"github.com/satori-com/satori-rtm-sdk-go/rtm/subscription"
	"math/rand"
	"os"
	"os/signal"
	"time"
)

const (
	// Replace these values with your project's credentials
	// from Dev Portal (https://developer.satori.com/#/projects).
	ENDPOINT = "YOUR_ENDPOINT"
	APP_KEY  = "YOUR_APPKEY"

	// Role and Secret are optional. Leaving these variables as is means no authentication.
	ROLE            = "YOUR_ROLE"
	ROLE_SECRET_KEY = "YOUR_SECRET"

	// Any channel name
	CHANNEL = "animals"
)

// We define message struct that we are going to publish
type Animal struct {
	Who   string     `json:"who"`
	Where [2]float32 `json:"where"`
}

func main() {
	options := rtm.Options{}
	if ROLE_SECRET_KEY != "YOUR_SECRET" {
		// AuthProvider performs additional actions to authenticate the client
		//
		// Satori Docs: Authentication
		// https://www.satori.com/docs/using-satori/authentication
		authProvider := auth.New(ROLE, ROLE_SECRET_KEY)
		options = rtm.Options{
			AuthProvider: authProvider,
		}
	}

	client, err := rtm.New(ENDPOINT, APP_KEY, options)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// We just created a new client. The client is not connected yet and we will connect it later.
	// Before start the client you can specify additional callbacks to be able to react on events.
	// The full events list is specified here: https://godoc.org/github.com/satori-com/satori-rtm-sdk-go/rtm#hdr-EVENTS
	//
	// Let's use only three of them: OnConnected, OnError and OnStop
	client.OnConnectedOnce(func() {
		fmt.Println("Connected to RTM!")
	})
	client.OnError(func(err rtm.RTMError) {
		fmt.Println(err.Reason)
	})
	client.OnStop(func() {
		fmt.Println("Gracefully shutdown a program")
		os.Exit(0)
	})

	// For synchronisation reason we will use typed channel (type Animal) to be able to collect all incoming messages
	data_c := make(chan Animal)

	// We create a subscription listener in order to receive callbacks
	// for incoming data, state changes and errors.
	//
	// The full list of available subscription events is here:
	// https://godoc.org/github.com/satori-com/satori-rtm-sdk-go/rtm#hdr-SUBSCRIPTIONS
	listener := subscription.Listener{
		// In this callback we will process all incoming messages
		OnData: func(data pdu.SubscriptionData) {
			for _, message := range data.Messages {
				var animal Animal

				// We assume, that the message in the channel has an Animal struct.
				err := json.Unmarshal(message, &animal)
				if err == nil {
					data_c <- animal
				} else {
					// We failed to convert message to the Animal struct.
					// Let's just print the message
					fmt.Println("Whoops! Not an animal: ", string(message))
				}
			}
		},

		// Called when the subscription is established. Once we subscribed we create 2 demo animals that
		// will randomly move.
		// Be aware: All callbacks MUST NOT block the main thread. You should use go-routines in cases if you need
		// to wait for some data/events/etc.
		OnSubscribed: func(pdu.SubscribeOk) {
			// We have a function that will create an animal in a separate go-routine and will randomly change coords
			go createAnimal(client, "zebra", 34.134358, -118.321506, 300*time.Millisecond)
			go createAnimal(client, "giraffe", 34.22123, -118.336543, 1*time.Second)
		},

		// Called when failed to subscribe
		OnSubscribeError: func(err pdu.SubscribeError) {
			fmt.Println("Failed to subscribe:", err.Error, err.Reason)
		},
	}

	// Create Subscription. You should specify SubscriptionID, Subscription Mode, Options and Callbacks listener
	// To get more information about the Subscription modes follow the link:
	// https://godoc.org/github.com/satori-com/satori-rtm-sdk-go/rtm/subscription#pkg-variables
	//
	// Satori Docs: Subscribing
	// https://www.satori.com/docs/using-satori/subscribing
	client.Subscribe(CHANNEL, subscription.SIMPLE, pdu.SubscribeBodyOpts{}, listener)

	// Now we start the client. After that client will establish connection to the Satori endpoint,
	// pass the authentication and subscribe to the channel.
	// Our callbacks will catch all these events.
	client.Start()

	// Catch "Ctrl + C" and gracefully shutdown the program
	sig_c := make(chan os.Signal, 1)
	signal.Notify(sig_c, os.Interrupt)
	go func() {
		<-sig_c
		// We got Ctrl + C. Stop the program
		client.Stop()
	}()
	fmt.Println("Press CTRL-C to exit")
	fmt.Println("====================")

	// Now we have a subscription that will forward all incoming messages to our go-channel.
	for animal := range data_c {
		fmt.Printf("%+v\n", animal)
	}
}

func createAnimal(client *rtm.RTM, who string, lat, long float32, sleep time.Duration) {
	for {
		response := <-client.PublishAck(CHANNEL, Animal{
			Who:   who,
			Where: [2]float32{lat, long}},
		)

		// Process possible publish error. You can get error response, when, for example, you use role that
		// has no access to publish to specified channel.
		if response.Err != nil {
			fmt.Printf("Unable to publish message to the '%s' channel\n", CHANNEL)
			fmt.Println(response.Err.Error())
		}

		move := func(in float32) float32 {
			return in + float32(rand.Intn(100)-50)/100000
		}
		lat = move(lat)
		long = move(long)
		time.Sleep(sleep)
	}
}
