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
	Start() <-chan Message
}

type CombinedChat struct {
	chats []Chat
}

type ChannelType int

const (
	ChannelTypeTwitch ChannelType = iota
	ChannelTypeVk
)

type Channel struct {
	Type ChannelType
	Name string
}

// TODO: сообщать пользователя, если пользователя не существует
func NewCombinedChat(channel Channel) (*CombinedChat, error) { //TODO: изменить конструктор, чтобы он принимал список название платформ и каналов
	result := new(CombinedChat)
	switch {
	case channel.Type == ChannelTypeTwitch:
		result.chats = append(result.chats, NewTwitchChat(channel.Name))
	case channel.Type == ChannelTypeVk:
		result.chats = append(result.chats, NewVkChat(channel.Name))
	default:
		return nil, fmt.Errorf("undefined channel type")
	}

	return result, nil
}

func (cc *CombinedChat) Start() <-chan Message {
	resultChan := make(chan Message)

	go func() {
		for _, chat := range cc.chats {
			go func(chat Chat) {
				inputChan := chat.Start()
				for {
					resultChan <- <-inputChan
				}
			}(chat)
		}
	}()

	return resultChan
}
