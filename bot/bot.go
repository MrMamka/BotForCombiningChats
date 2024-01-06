package bot

import (
	"BotForCombiningChats/chat"
	"errors"
	"fmt"
	"log"
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

const welcomeText string = "Здесь должно быть приветственное сообщение"

var NotFoundEnvError = errors.New("env not found")

const (
	notWorkingStage  = 0
	pendingNameStage = iota
	workingStage
)

type TelegramBot struct {
	token         string
	bot           *tgbotapi.BotAPI
	stop          chan struct{}
	stages        map[int64]int
	stageHandlers map[int]func(*tgbotapi.Message) error
}

func NewTelegramBot() *TelegramBot {
	tb := new(TelegramBot)
	tb.stop = make(chan struct{})
	tb.stages = make(map[int64]int)
	tb.stageHandlers = map[int]func(*tgbotapi.Message) error{
		notWorkingStage: tb.notWorkingHandler,
		pendingNameStage: tb.pendingNameHandler,
		workingStage: tb.workingHandler,
	}
	return tb
}

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
		return NotFoundEnvError
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

			tb.handleMsg(msgReq)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (tb *TelegramBot) handleMsg(msgReq *tgbotapi.Message) error {
	stage := tb.stages[msgReq.Chat.ID]
	return tb.stageHandlers[stage](msgReq)
}

func (tb *TelegramBot) notWorkingHandler(msgReq *tgbotapi.Message) error {
	var err error
	switch msgReq.Text {
	case "/work":
		tb.stages[msgReq.Chat.ID] = pendingNameStage
		tb.sendMsg(msgReq.Chat.ID, "Введите имя пользователя, чат которого хотите видеть.")
	default:
		tb.sendMsg(msgReq.Chat.ID, "Если хотите отслеживать чат - напишите /work")
	}
	return err
}

func (tb *TelegramBot) pendingNameHandler(msgReq *tgbotapi.Message) error {
	tb.stages[msgReq.Chat.ID] = workingStage
	return tb.startChats(msgReq)
}

func (tb *TelegramBot) workingHandler(msgReq *tgbotapi.Message) error {
	var err error
	switch msgReq.Text {
	case "/stop":
		tb.stop <- struct{}{}
		tb.stages[msgReq.Chat.ID] = notWorkingStage
		return tb.sendMsg(msgReq.Chat.ID, "Чат остановлен.")
	default:
		tb.sendMsg(msgReq.Chat.ID, "Если хотите остановить чат - напишите /stop")
	}
	return err
}

func (tb *TelegramBot) startMsgHandler(msgReq *tgbotapi.Message) error {
	tb.stages[msgReq.Chat.ID] = notWorkingStage

	return tb.sendMsg(msgReq.Chat.ID, welcomeText)
}

func (tb *TelegramBot) startChats(msgReq *tgbotapi.Message) error { //TODO: Добавить обработку ошибку (сейчас так: _, _)
	_ = tb.sendMsg(msgReq.Chat.ID, "Запускаю...")

	combChat, _ := chat.NewCombinedChat(msgReq.Text) //TODO: сделать, чтобы это вводил пользователь
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

func (tb *TelegramBot) defaultMsgHandler(msgReq *tgbotapi.Message) error {
	msgResp := tgbotapi.NewMessage(msgReq.Chat.ID, msgReq.Text)
	msgResp.ReplyToMessageID = msgReq.MessageID

	_, err := tb.bot.Send(msgResp)
	if err != nil {
		return err
	}
	return nil
}
