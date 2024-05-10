package chat

import (
	"fmt"
	"time"
)

type Message struct {
	Text   string
	Author string
	Time   time.Time
}

type Chat interface {
	Start(chan<- Message)
	Stop()
}

type CombinedChat struct {
	chats []Chat
}

type ChannelType int

const (
	TwitchChannelType ChannelType = iota
	VkChannelType
)

type Channel struct {
	Type ChannelType
	Name string
}

// TODO: сообщать пользователю, если пользователя не существует
func NewCombinedChat(channels []Channel) (*CombinedChat, error) {
	result := new(CombinedChat)

	for _, channel := range channels {
		switch {
		case channel.Type == TwitchChannelType:
			result.chats = append(result.chats, NewTwitchChat(channel.Name))
		case channel.Type == VkChannelType:
			result.chats = append(result.chats, NewVkChat(channel.Name))
		default:
			return nil, fmt.Errorf("undefined channel type") // TODO: just ignore?
		}
	}
	return result, nil
}

func (cc *CombinedChat) Start(stop <-chan struct{}) <-chan Message {
	resultChan := make(chan Message)

	go func() {
		for _, chat := range cc.chats {
			go func(chat Chat) {
				chat.Start(resultChan)
			}(chat)
		}

		<-stop
		for _, chat := range cc.chats {
			chat.Stop()
		}
	}()

	return resultChan
}
