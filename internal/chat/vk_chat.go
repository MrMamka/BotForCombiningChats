package chat

import (
	"strings"
	"time"

	vk "github.com/MrMamka/combchats/pkg/vkplaylive"
)

type VkChat struct {
	channelName string
}

func NewVkChat(channelName string) *VkChat {
	return &VkChat{
		channelName: channelName,
	}
}

func (tc *VkChat) Start() <-chan Message {
	resultChan := make(chan Message)

	go func() {
		client := vk.NewAnonymousClient()

		client.OnMessage(func(msg vk.Message) { // TODO: обрабатывать, когда сообщение - ответ на другое
			var builder strings.Builder
			for _, subj := range msg.Data {
				builder.WriteString(subj.Content)
			}

			resultChan <- Message{
				Text:   builder.String(),
				Author: msg.Author,
				Time:   time.Unix(msg.Time, 0)}
		})

		//client.OnConnect(func() { //TODO: выводить пользователю connecting и connected
		//	fmt.Println("Connected")
		//})

		client.Join(tc.channelName)

		err := client.Connect()
		if err != nil {
			panic(err) // TODO: убрать панику и просто выбрасывать ошибку
		}
	}()

	return resultChan
}
