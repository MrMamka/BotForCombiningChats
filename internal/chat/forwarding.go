package chat

import (
	"errors"
	"fmt"
)

type Reciever struct {
	Channel
	AuthToken  string
	SenderName string
	SentMsgs   map[string]struct{} // TODO: поменять на лру кэш
}

type Sender interface {
	Send(string) error
	Stop()
}

func Forward(from Channel, to Reciever, stop <-chan struct{}) error {
	forwardChan := make(chan Message)
	var fromChat Chat

	switch from.Type {
	case TwitchChannelType:
		fromChat = NewTwitchChat(from.Name)
	case VkChannelType:
		fromChat = NewVkChat(from.Name)
	default:
		return errors.New("undefined channel type")
	}

	var sender Sender
	switch to.Type {
	case TwitchChannelType:
		sender = NewTwitchSender(to.SenderName, to.Name, to.AuthToken)
	case VkChannelType:
		sender = NewVkSender(to.Name, to.AuthToken)
	default:
		return errors.New("undefined channel type")
	}

	go func() {
		fromChat.Start(forwardChan)
	}()

	go func() {
		for {
			var msg Message
			select {
			case msg = <-forwardChan:
			case <-stop:
				fromChat.Stop()
				sender.Stop()
				return
			}

			msgText := MessageToText(msg)
			if to.SentMsgs != nil {
				if _, ok := to.SentMsgs[msg.Text]; ok {
					continue
				}

				to.SentMsgs[msgText] = struct{}{}
			}

			if err := sender.Send(msgText); err != nil {
				fmt.Printf("error in sending message: %v\n", err)
			}
		}
	}()

	return nil
}

func MessageToText(msg Message) string {
	return fmt.Sprintf("%s: %s", msg.Author, msg.Text)
}
