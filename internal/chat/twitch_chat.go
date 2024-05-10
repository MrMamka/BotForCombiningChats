package chat

import (
	"fmt"

	"github.com/gempir/go-twitch-irc/v4"
)

type TwitchChat struct {
	channelName string
	client      *twitch.Client
}

func NewTwitchChat(channelName string) *TwitchChat {
	return &TwitchChat{
		channelName: channelName,
	}
}

func (tc *TwitchChat) Start(output chan<- Message) {
	go func() {
		tc.client = twitch.NewAnonymousClient()

		tc.client.OnPrivateMessage(func(msg twitch.PrivateMessage) { // TODO: обрабатывать, когда сообщение - ответ на другое?
			output <- Message{Text: msg.Message, Author: msg.User.DisplayName, Time: msg.Time}
		})

		tc.client.Join(tc.channelName)

		err := tc.client.Connect()
		if err != nil {
			fmt.Printf("error in connect: %v\n", err)
		}
	}()
}

func (tc *TwitchChat) Stop() {
	print("here stop\n")
	tc.client.Disconnect()
}

type TwitchSender struct {
	client  *twitch.Client
	channel string
}

func NewTwitchSender(userName, channel, authToken string) *TwitchSender {
	client := twitch.NewClient(userName, authToken)

	go client.Connect() // TODO: передавать канал, чтобы убивать эту горутину

	return &TwitchSender{
		client:  client,
		channel: channel,
	}
}

func (ts *TwitchSender) Send(msg string) error {
	ts.client.Say(ts.channel, msg)
	return nil
}

func (tc *TwitchSender) Stop() {
	tc.client.Disconnect()
}
