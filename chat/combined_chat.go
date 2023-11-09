package chat

import "time"

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

func NewCombinedChat(channelName string) (*CombinedChat, error) { //TODO: изменить конструктор, чтобы он принимал список название платформ и каналов
	result := new(CombinedChat)

	result.chats = append(result.chats, NewTwitchChat(channelName))

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
