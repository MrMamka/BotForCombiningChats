package chat

import (
	"strings"
	"time"

	vk "github.com/MrMamka/combchats/pkg/vkplaylive"
)

type VkChat struct {
	channelName string
	client      *vk.Client
	stoped      bool
}

func NewVkChat(channelName string) *VkChat {
	return &VkChat{
		channelName: channelName,
	}
}

func (vc *VkChat) Start(output chan<- Message) {
	go func() {
		vc.client = vk.NewAnonymousClient()

		vc.client.OnMessage(func(msg vk.Message) { // TODO: обрабатывать, когда сообщение - ответ на другое?
			if vc.stoped {
				return
			}

			var builder strings.Builder
			for _, subj := range msg.Data {
				builder.WriteString(subj.Content)
			}

			output <- Message{
				Text:   builder.String(),
				Author: msg.Author,
				Time:   time.Unix(msg.Time, 0)}
		})

		vc.client.Join(vc.channelName)

		vc.client.Connect()
		// if err != nil {
		// 	panic(err) // TODO: убрать панику и просто выбрасывать ошибку
		// }
	}()
}

func (vc *VkChat) Stop() {
	vc.client.Disconnect()
	vc.stoped = true
}

type VkSender struct {
	client *vk.Client
}

func NewVkSender(channelName, authToken string) *VkSender {
	client := vk.NewClient(authToken)
	client.Join(channelName)
	return &VkSender{
		client: client,
	}
}

func (vs *VkSender) Send(msg string) error {
	return vs.client.SendMessage(msg)
}

func (vc *VkSender) Stop() {}
