package grafana

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"

	"github.com/centrifugal/centrifuge-go"
	"github.com/grafana/grizzly/pkg/grizzly"
)

type eventHandler struct {
	filename string
}

func (h *eventHandler) OnConnect(c *centrifuge.Client, e centrifuge.ConnectEvent) {
	log.Printf("Connected to chat with ID %s", e.ClientID)
	return
}

func (h *eventHandler) OnError(c *centrifuge.Client, e centrifuge.ErrorEvent) {
	log.Printf("Error: %s", e.Message)
	return
}

func (h *eventHandler) OnDisconnect(c *centrifuge.Client, e centrifuge.DisconnectEvent) {
	log.Printf("Disconnected from chat: %s", e.Reason)
	return
}
func (h *eventHandler) OnSubscribeSuccess(sub *centrifuge.Subscription, e centrifuge.SubscribeSuccessEvent) {
	log.Printf("Subscribed on channel %s, resubscribed: %v, recovered: %v", sub.Channel(), e.Resubscribed, e.Recovered)
}

func (h *eventHandler) OnSubscribeError(sub *centrifuge.Subscription, e centrifuge.SubscribeErrorEvent) {
	log.Printf("Subscribed on channel %s failed, error: %s", sub.Channel(), e.Error)
}

func (h *eventHandler) OnUnsubscribe(sub *centrifuge.Subscription, e centrifuge.UnsubscribeEvent) {
	log.Printf("Unsubscribed from channel %s", sub.Channel())
}

func (h *eventHandler) OnMessage(_ *centrifuge.Client, e centrifuge.MessageEvent) {
	log.Printf("Message from server: %s", string(e.Data))
}
func (h *eventHandler) OnServerPublish(c *centrifuge.Client, e centrifuge.ServerPublishEvent) {
	log.Printf("Publication from server-side channel %s: %s", e.Channel, e.Data)
}
func (h *eventHandler) OnServerSubscribe(_ *centrifuge.Client, e centrifuge.ServerSubscribeEvent) {
	log.Printf("Subscribe to server-side channel %s: (resubscribe: %t, recovered: %t)", e.Channel, e.Resubscribed, e.Recovered)
}

func (h *eventHandler) OnServerUnsubscribe(_ *centrifuge.Client, e centrifuge.ServerUnsubscribeEvent) {
	log.Printf("Unsubscribe from server-side channel %s", e.Channel)
}

func (h *eventHandler) OnServerJoin(_ *centrifuge.Client, e centrifuge.ServerJoinEvent) {
	log.Printf("Server-side join to channel %s: %s (%s)", e.Channel, e.User, e.Client)
}

func (h *eventHandler) OnServerLeave(_ *centrifuge.Client, e centrifuge.ServerLeaveEvent) {
	log.Printf("Server-side leave from channel %s: %s (%s)", e.Channel, e.User, e.Client)
}

func (h *eventHandler) OnPublish(sub *centrifuge.Subscription, e centrifuge.PublishEvent) {
	response := struct {
		UID    string `json:"uid"`
		Action string `json:"action"`
		UserID int64  `json:"userId"`
	}{}
	err := json.Unmarshal(e.Data, &response)
	if err != nil {
		log.Println(err)
		return
	}
	if response.Action != "saved" {
		log.Println("Unknown action received", string(e.Data))
	}
	dashboard, err := getRemoteDashboard(response.UID)
	if err != nil {
		log.Println(err)
		return
	}
	dashboardJSON, err := dashboard.toJSON()
	if err != nil {
		log.Println(err)
		return
	}
	ioutil.WriteFile(h.filename, []byte(dashboardJSON), 0644)
	log.Printf("%s updated from dashboard %s", h.filename, response.UID)
}

func watchDashboard(notifier grizzly.Notifier, UID, filename string) error {
	wsURL, token, err := getWSGrafanaURL("live/ws?format=json")
	if err != nil {
		return err
	}
	//mt.Sprintf("ws://%s/live/ws?format=protobuf"
	log.Printf("Connect to %s\n", wsURL)

	c := centrifuge.New(wsURL, centrifuge.DefaultConfig())
	handler := &eventHandler{
		filename: filename,
	}
	c.OnConnect(handler)
	c.OnError(handler)
	c.OnDisconnect(handler)
	c.OnMessage(handler)
	c.OnServerPublish(handler)
	c.OnServerSubscribe(handler)
	c.OnServerUnsubscribe(handler)
	c.OnServerJoin(handler)
	c.OnServerLeave(handler)
	c.SetToken(token)

	channel := fmt.Sprintf("grafana/dashboard/%s", UID)
	sub, err := c.NewSubscription(channel)
	if err != nil {
		return err
	}

	sub.OnSubscribeSuccess(handler)
	sub.OnSubscribeError(handler)
	sub.OnUnsubscribe(handler)
	sub.OnPublish(handler)

	err = sub.Subscribe()
	if err != nil {
		return err
	}

	err = c.Connect()
	if err != nil {
		return err
	}

	// Run until CTRL+C.
	select {}
}
