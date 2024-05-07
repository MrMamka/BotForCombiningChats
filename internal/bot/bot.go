package bot

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/MrMamka/combchats/internal/chat"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

// const welcomeText = "Здесь должно быть приветственное сообщение"

var ErrNotFoundEnv = errors.New("env not found")

const (
	notWorkingStage = iota
	pendingPlatformStage
	pendingNameStage
	workingStage
)

var Platforms = map[string]chat.ChannelType{
	"Twitch": chat.ChannelTypeTwitch,
	"Vk":     chat.ChannelTypeVk,
}

func AvailbalePlatforms() string {
	platforms := make([]string, 0, len(Platforms))
	for platform := range Platforms {
		platforms = append(platforms, platform)
	}
	return strings.Join(platforms, "/")
}

type status struct {
	stage    int
	platform chat.ChannelType
}

type TelegramBot struct {
	token           string
	bot             *tgbotapi.BotAPI
	stop            chan struct{}
	dialoguesStatus map[int64]*status
	stageHandlers   map[int]func(*tgbotapi.Message) error
}

func NewTelegramBot() *TelegramBot {
	tb := new(TelegramBot)
	tb.stop = make(chan struct{})
	tb.dialoguesStatus = make(map[int64]*status)
	tb.stageHandlers = map[int]func(*tgbotapi.Message) error{
		notWorkingStage:      tb.notWorkingHandler,
		pendingPlatformStage: tb.pendingPlatformHandler,
		pendingNameStage:     tb.pendingNameHandler,
		workingStage:         tb.workingHandler,
	}
	return tb
}

// TODO: выводить системные сообщения жирным.
func (tb *TelegramBot) sendMsg(chatId int64, msgText string) error {
	msgResp := tgbotapi.NewMessage(chatId, msgText)
	_, err := tb.bot.Send(msgResp)
	return err
}

func (tb *TelegramBot) SetTokenFromEnv(env string) error {
	if err := godotenv.Load(); err != nil {
		return err
	}
	tb.token = os.Getenv(env)
	if tb.token == "" {
		return ErrNotFoundEnv
	}
	return nil
}

func (tb *TelegramBot) Start(isDebug bool) error {
	var err error
	tb.bot, err = tgbotapi.NewBotAPI(tb.token)
	if err != nil {
		return err
	}

	tb.bot.Debug = isDebug

	log.Printf("Authorized on account %s", tb.bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 180

	updates := tb.bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil {
			msgReq := update.Message
			log.Printf("[%s] %s", msgReq.From.UserName, msgReq.Text)

			err = tb.handleMsg(msgReq)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (tb *TelegramBot) handleMsg(msgReq *tgbotapi.Message) error {
	if tb.dialoguesStatus[msgReq.Chat.ID] == nil {
		tb.dialoguesStatus[msgReq.Chat.ID] = &status{stage: notWorkingStage}
	}
	stage := tb.dialoguesStatus[msgReq.Chat.ID].stage
	return tb.stageHandlers[stage](msgReq)
}

func (tb *TelegramBot) notWorkingHandler(msgReq *tgbotapi.Message) error {
	switch msgReq.Text {
	case "/work":
		tb.dialoguesStatus[msgReq.Chat.ID].stage = pendingPlatformStage
		err := tb.sendMsg(msgReq.Chat.ID,
			fmt.Sprintf("Введите название платформы (%s), чат которой хотите видеть.", AvailbalePlatforms()))
		if err != nil {
			return err
		}
	default:
		err := tb.sendMsg(msgReq.Chat.ID, "Если хотите отслеживать чат - напишите /work")
		if err != nil {
			return err
		}
	}
	return nil
}

func (tb *TelegramBot) pendingNameHandler(msgReq *tgbotapi.Message) error {
	tb.dialoguesStatus[msgReq.Chat.ID].stage = workingStage
	return tb.startChats(msgReq)
}

func (tb *TelegramBot) pendingPlatformHandler(msgReq *tgbotapi.Message) error {
	platform, ok := Platforms[msgReq.Text]
	if !ok {
		tb.sendMsg(msgReq.Chat.ID, fmt.Sprintf("Неизвестная платформа. Доступные: %s", AvailbalePlatforms()))
		return nil
	}

	err := tb.sendMsg(msgReq.Chat.ID, "Введите имя пользователя, чат которого хотите видеть.")
	if err != nil {
		return err
	}
	tb.dialoguesStatus[msgReq.Chat.ID].platform = platform
	tb.dialoguesStatus[msgReq.Chat.ID].stage = pendingNameStage
	return nil
}

func (tb *TelegramBot) workingHandler(msgReq *tgbotapi.Message) error {
	var err error
	switch msgReq.Text {
	case "/stop":
		tb.stop <- struct{}{}
		tb.dialoguesStatus[msgReq.Chat.ID].stage = notWorkingStage
		return tb.sendMsg(msgReq.Chat.ID, "Чат остановлен.")
	default:
		tb.sendMsg(msgReq.Chat.ID, "Если хотите остановить чат - напишите /stop")
	}
	return err
}

// func (tb *TelegramBot) startMsgHandler(msgReq *tgbotapi.Message) error {
// 	tb.stages[msgReq.Chat.ID] = notWorkingStage

// 	return tb.sendMsg(msgReq.Chat.ID, welcomeText)
// }

func (tb *TelegramBot) startChats(msgReq *tgbotapi.Message) error { //TODO: Добавить обработку ошибку (сейчас так: _, _)
	_ = tb.sendMsg(msgReq.Chat.ID, "Запускаю...")

	channel := chat.Channel{Type: tb.dialoguesStatus[msgReq.Chat.ID].platform, Name: msgReq.Text}
	combChat, _ := chat.NewCombinedChat(channel)
	outputChan := combChat.Start()

	_ = tb.sendMsg(msgReq.Chat.ID, "Готово!")

	go func() {
		for {
			select {
			case <-tb.stop:
				return
			case msg := <-outputChan:
				textResp := fmt.Sprintf("%s: %s", msg.Author, msg.Text) // TODO: Вынести в отдельную функцию

				_ = tb.sendMsg(msgReq.Chat.ID, textResp)
			}
		}
	}()

	return nil
}

// func (tb *TelegramBot) defaultMsgHandler(msgReq *tgbotapi.Message) error {
// 	msgResp := tgbotapi.NewMessage(msgReq.Chat.ID, msgReq.Text)
// 	msgResp.ReplyToMessageID = msgReq.MessageID

// 	_, err := tb.bot.Send(msgResp)
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }
