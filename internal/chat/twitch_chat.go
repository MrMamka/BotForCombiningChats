package chat

import (
	"github.com/gempir/go-twitch-irc/v4"
)

type TwitchChat struct {
	channelName string
}

func NewTwitchChat(channelName string) *TwitchChat {
	return &TwitchChat{
		channelName: channelName,
	}
}

func (tc *TwitchChat) Start() <-chan Message {
	resultChan := make(chan Message)

	go func() {
		client := twitch.NewAnonymousClient()

		client.OnPrivateMessage(func(msg twitch.PrivateMessage) { // TODO: обрабатывать, когда сообщение - ответ на другое
			resultChan <- Message{Text: msg.Message, Author: msg.User.DisplayName, Time: msg.Time}
		})

		//client.OnConnect(func() { //TODO: выводить пользователю connecting и connected
		//	fmt.Println("Connected")
		//})

		client.Join(tc.channelName)

		err := client.Connect()
		if err != nil {
			panic(err)
		}
	}()

	return resultChan
}
