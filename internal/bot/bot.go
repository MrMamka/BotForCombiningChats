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

var ErrNotFoundEnv = errors.New("env not found")

type Stage int

// TODO: добавить настройку для вывода площадки и стрима в сообщении при объединении
const (
	NotWorkingStage Stage = iota
	ChooseModeStage

	PendingChatsStage
	WorkingCombiningStage

	PendingTwoChatsStage
	PendingDirectionStage
	PendingTokensStage
	WorkingForwardingStage
)

var helpMessages = map[Stage]string{
	NotWorkingStage: "Бот умеет объединять чаты стримов. В данный момент поддерживаются площадки: Vk Play Live и Twitch",
	ChooseModeStage: "В режиме объединения сообщения будут появляться в этом телеграм чате. " +
		"В режим пересылки сообщения будут отправляться из одного чата стрима в другой",

	PendingChatsStage: "Правильное написание ника стримера можно узнать в URL его стрима." +
		"Название площадок должно быть ровно такое, как было написано выше (в данный момент оно чувствительно к регистру)",
	WorkingCombiningStage: "Чтобы остановить поток сообщений напишите /stop или /restart",

	PendingTwoChatsStage: "Правильное написание ника стримера можно узнать в URL его стрима." +
		"Название площадок должно быть ровно такое, как было написано выше (в данный момент оно чувствительно к регистру)",
	PendingDirectionStage: "/first пересылает в первый чат из второго, /second - во второй из первого, /both - /first и /second одновременно",
	PendingTokensStage: `Ник можно узнать в URL, зайдя на свой канал.
	Vk токен можно узнать после входа в аккаунт vk play live в консоли разработчика, в Cookie Header'е одного из запросов. Он идёт после accessToken. Пример токена:
	7a41109f60fbb5aa16dcb4c4d3ea4a3ffac4af1d22aa8998b8a0209d0231faba (этот токен не настоящий)
	Twitch токен можно узнать на специальных сайтах. Например twitchtokengenerator.com. Пример токена:
	oauth:uy1tkpc8fer0xbh122ewrmq1cked2b (этот токен не настоящий)
	В данный момент нет валидации токенов, поэтому неправильность токенов можно узнать только по тому, что бот не будет работать. В такой ситуации можно написать /restart`,
	WorkingForwardingStage: "Чтобы остановить пересылку сообщений напишите /stop или /restart",
}

var Platforms = map[string]chat.ChannelType{
	"Twitch": chat.TwitchChannelType,
	"Vk":     chat.VkChannelType,
}

var PlatformsTypes = map[chat.ChannelType]string{
	chat.TwitchChannelType: "Twitch",
	chat.VkChannelType:     "Vk",
}

func AvailbalePlatforms() string {
	platforms := make([]string, 0, len(Platforms))
	for platform := range Platforms {
		platforms = append(platforms, platform)
	}
	return strings.Join(platforms, "/")
}

type Forwarding int

const (
	FirstForwarding Forwarding = iota
	SecondForwarding
	BothForwarding
)

type recieverInfo struct {
	token      string
	senderName string
}

type status struct {
	stage     Stage
	channels  []chat.Channel
	forwardTo Forwarding
	receivers []recieverInfo
	stop      chan struct{}
}

type TelegramBot struct {
	Token           string
	bot             *tgbotapi.BotAPI
	dialoguesStatus map[int64]*status
	stageHandlers   map[Stage]func(*tgbotapi.Message, *status) error
}

func NewTelegramBot() *TelegramBot {
	tb := new(TelegramBot)
	tb.dialoguesStatus = make(map[int64]*status)
	tb.stageHandlers = map[Stage]func(*tgbotapi.Message, *status) error{
		NotWorkingStage: tb.notWorkingHandler,
		ChooseModeStage: tb.chooseModeHandler,

		PendingChatsStage:     tb.pendingChatsHandler,
		WorkingCombiningStage: tb.workingCombiningHandler,

		PendingTwoChatsStage:   tb.pendingTwoChatsHandler,
		PendingDirectionStage:  tb.pendingDirectionHandler,
		PendingTokensStage:     tb.pendingTokensHandler,
		WorkingForwardingStage: tb.workingForwardingHandler,
	}

	return tb
}

func (tb *TelegramBot) notWorkingHandler(msgReq *tgbotapi.Message, stat *status) error {
	stat.stage = ChooseModeStage
	stat.channels = []chat.Channel{}
	stat.receivers = []recieverInfo{}
	stat.stop = make(chan struct{})

	return tb.sendMsg(msgReq.Chat.ID,
		"Выберите режим, в котором хотите использовать бота: персылка сообщений (/forwarding) или объединение чатов (/combining)")
}

func (tb *TelegramBot) chooseModeHandler(msgReq *tgbotapi.Message, stat *status) error {
	switch msgReq.Text {
	case "/combining":
		stat.stage = PendingChatsStage
		return tb.sendMsg(msgReq.Chat.ID, fmt.Sprintf(
			"Вводите названия платформ (%s) и ники стримеров, чаты которых хотите видеть, "+
				"отдельными сообщениями в формате \"*плафторма* *ник*\" (без кавычек и звёздочек). "+
				"Конец ввода подтвердите командой /done", AvailbalePlatforms()))
	case "/forwarding":
		stat.stage = PendingTwoChatsStage
		return tb.sendMsg(msgReq.Chat.ID, fmt.Sprintf(
			"Вводите названия платформ (%s) и ники стримеров, чаты которых хотите видеть, "+
				"отдельными сообщениями в формате \"*плафторма* *ник*\" (без кавычек и звёздочек).", AvailbalePlatforms()))
	default:
		return tb.sendMsg(msgReq.Chat.ID, "Неподдерживаемый режим. Выберите /forwarding или /combining")
	}
}

func (tb *TelegramBot) pendingChatsHandler(msgReq *tgbotapi.Message, stat *status) error {
	if msgReq.Text == "/done" {
		stat.stage = WorkingCombiningStage
		return tb.startChats(msgReq.Chat.ID, stat) // TODO: validate size of channels?
	}

	input := strings.Fields(msgReq.Text)
	if len(input) != 2 {
		return tb.sendMsg(msgReq.Chat.ID, "Неверный формат ввода. Ожидалось \"*плафторма* *ник*\" или /done")
	}

	platform, ok := Platforms[input[0]] // TODO: убрать чувствительность в регистру (добавить возможность написать по-русскими?)
	if !ok {
		tb.sendMsg(msgReq.Chat.ID, fmt.Sprintf("Неизвестная платформа. Доступные: %s", AvailbalePlatforms()))
		return nil
	}

	stat.channels = append(stat.channels, chat.Channel{Type: platform, Name: input[1]}) // TODO: validate name

	return tb.sendMsg(msgReq.Chat.ID, "Записано")
}

func (tb *TelegramBot) workingCombiningHandler(msgReq *tgbotapi.Message, stat *status) error {
	switch msgReq.Text {
	case "/stop":
		close(stat.stop)
		stat.stage = NotWorkingStage
		return tb.sendMsg(msgReq.Chat.ID, "Чат остановлен.")
	default:
		return tb.sendMsg(msgReq.Chat.ID, "Если хотите остановить чат - напишите /stop")
	}
}

func (tb *TelegramBot) pendingTwoChatsHandler(msgReq *tgbotapi.Message, stat *status) error {
	// TODO: вынести код, так как в pendingChatsHandler копипаст
	input := strings.Fields(msgReq.Text)
	if len(input) != 2 {
		return tb.sendMsg(msgReq.Chat.ID, "Неверный формат ввода. Ожидалось \"*плафторма* *ник*\" или /done")
	}

	platform, ok := Platforms[input[0]] // TODO: убрать чувствительность в регистру (добавить возможность написать по-русскими?)
	if !ok {
		return tb.sendMsg(msgReq.Chat.ID, fmt.Sprintf("Неизвестная платформа. Доступные: %s", AvailbalePlatforms()))
	}

	stat.channels = append(stat.channels, chat.Channel{Type: platform, Name: input[1]}) // TODO: validate name

	if len(stat.channels) == 1 {
		return tb.sendMsg(msgReq.Chat.ID, "Записал. Введите в том же формате ещё один чат, который хотите отслеживать")
	} else if len(stat.channels) == 2 {
		stat.stage = PendingDirectionStage
		return tb.sendMsg(msgReq.Chat.ID, fmt.Sprintf(
			`Записал. Теперь выбери, куда пересылать сообщения.
		/first - пересылать сообщения из %s %s в %s %s (из второго чата в первый)
		/second - пересылать сообщения из %s %s в %s %s (из первого чата во второй)
		/both - пересылать сообщения друг в друга`,
			PlatformsTypes[stat.channels[1].Type], stat.channels[1].Name, // TODO: убрать этот копипаст?
			PlatformsTypes[stat.channels[0].Type], stat.channels[0].Name,
			PlatformsTypes[stat.channels[0].Type], stat.channels[0].Name,
			PlatformsTypes[stat.channels[1].Type], stat.channels[1].Name))
	} else {
		return tb.sendMsg(msgReq.Chat.ID, "Что-то сломалось. Попробуйте перезапустить меня или обратиться к @mrmamka")
	}
}

func (tb *TelegramBot) pendingDirectionHandler(msgReq *tgbotapi.Message, stat *status) error {
	var channel chat.Channel
	switch msgReq.Text {
	case "/first":
		stat.forwardTo = FirstForwarding
		channel = stat.channels[0]
	case "/second":
		stat.forwardTo = SecondForwarding
		channel = stat.channels[1]
	case "/both":
		stat.forwardTo = BothForwarding
		channel = stat.channels[0]
	default:
		return tb.sendMsg(msgReq.Chat.ID, "Неизвестная команда. Введите /first, /second или /both")
	}
	stat.stage = PendingTokensStage

	return tb.sendMsg(msgReq.Chat.ID, fmt.Sprintf(
		"Введите имя и %s токен от аккаунта с которого будут отправляться сообщения в %s. В формате \"*имя* *токен*\"", // TODO: не просить, если платформа - вк
		PlatformsTypes[channel.Type], channel.Name))
}

// TODO: валидировать токен
func (tb *TelegramBot) pendingTokensHandler(msgReq *tgbotapi.Message, stat *status) error { // TODO: добавить /help (и написать, что он есть) про токены
	input := strings.Fields(msgReq.Text)
	if len(input) != 2 {
		return tb.sendMsg(msgReq.Chat.ID, "Неверный формат ввода. Ожидалось \"*имя* *токен*\"")
	}

	stat.receivers = append(stat.receivers, recieverInfo{senderName: input[0], token: input[1]})

	if stat.forwardTo == BothForwarding && len(stat.receivers) == 1 {
		return tb.sendMsg(msgReq.Chat.ID, fmt.Sprintf(
			"Записано. Теперь введите %s токен от аккаунта с которого будут отправляться сообщения в %s",
			PlatformsTypes[stat.channels[1].Type], stat.channels[1].Name))
	}

	tb.sendMsg(msgReq.Chat.ID, "Записано")
	stat.stage = WorkingForwardingStage
	return tb.startForwarding(msgReq.Chat.ID, stat)
}

func (tb *TelegramBot) workingForwardingHandler(msgReq *tgbotapi.Message, stat *status) error {
	switch msgReq.Text {
	case "/stop":
		close(stat.stop)
		stat.stage = NotWorkingStage
		return tb.sendMsg(msgReq.Chat.ID, "Пересылку остановлена")
	default:
		return tb.sendMsg(msgReq.Chat.ID, "Если хотите остановить пересылку - напишите /stop")
	}
}

func (tb *TelegramBot) startForwarding(chatID int64, stat *status) error {
	_ = tb.sendMsg(chatID, "Запускаю...")

	switch stat.forwardTo {
	case FirstForwarding:
		chat.Forward(stat.channels[1], chat.Reciever{
			Channel:    stat.channels[0],
			AuthToken:  stat.receivers[0].token,
			SenderName: stat.receivers[0].senderName},
			stat.stop)
	case SecondForwarding:
		chat.Forward(stat.channels[0], chat.Reciever{
			Channel:    stat.channels[1],
			AuthToken:  stat.receivers[0].token,
			SenderName: stat.receivers[0].senderName},
			stat.stop)
	case BothForwarding:
		sentMessage := make(map[string]struct{})

		chat.Forward(stat.channels[0], chat.Reciever{
			Channel:    stat.channels[1],
			AuthToken:  stat.receivers[1].token,
			SenderName: stat.receivers[1].senderName,
			SentMsgs:   sentMessage},
			stat.stop)

		chat.Forward(stat.channels[1], chat.Reciever{
			Channel:    stat.channels[0],
			AuthToken:  stat.receivers[0].token,
			SenderName: stat.receivers[0].senderName,
			SentMsgs:   sentMessage},
			stat.stop)
	}

	_ = tb.sendMsg(chatID, "Готово!")

	return nil
}

func (tb *TelegramBot) startChats(chatID int64, stat *status) error { //TODO: Добавить обработку ошибку (сейчас так: _, _)
	_ = tb.sendMsg(chatID, "Запускаю...")

	combChat, _ := chat.NewCombinedChat(stat.channels)
	outputChan := combChat.Start(stat.stop)

	_ = tb.sendMsg(chatID, "Готово!")

	go func() {
		for {
			select {
			case <-stat.stop:
				return
			case msg := <-outputChan:
				textResp := chat.MessageToText(msg) // TODO: Вынести в отдельную функцию?

				_ = tb.sendMsg(chatID, textResp)
			}
		}
	}()

	return nil
}

// TODO: выводить системные сообщения жирным.
func (tb *TelegramBot) sendMsg(chatId int64, msgText string) error {
	msgResp := tgbotapi.NewMessage(chatId, msgText)
	_, err := tb.bot.Send(msgResp)
	return err
}

func (tb *TelegramBot) handleMsg(msgReq *tgbotapi.Message) {
	if tb.dialoguesStatus[msgReq.Chat.ID] == nil {
		tb.dialoguesStatus[msgReq.Chat.ID] = &status{stage: NotWorkingStage}
	}

	if msgReq.Text == "/restart" {
		if tb.dialoguesStatus[msgReq.Chat.ID].stop != nil {
			close(tb.dialoguesStatus[msgReq.Chat.ID].stop)
		}
		tb.dialoguesStatus[msgReq.Chat.ID].stage = NotWorkingStage
	} else if msgReq.Text == "/help" {
		stage := tb.dialoguesStatus[msgReq.Chat.ID].stage
		tb.sendMsg(msgReq.Chat.ID, helpMessages[stage])
		return
	}

	stage := tb.dialoguesStatus[msgReq.Chat.ID].stage
	go tb.stageHandlers[stage](msgReq, tb.dialoguesStatus[msgReq.Chat.ID])
}

func (tb *TelegramBot) Start(isDebug bool) error {
	var err error
	tb.bot, err = tgbotapi.NewBotAPI(tb.Token)
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
		}
	}
	return nil
}

func (tb *TelegramBot) SetTokenFromEnv(env string) error {
	if err := godotenv.Load(); err != nil {
		return err
	}
	tb.Token = os.Getenv(env)
	if tb.Token == "" {
		return ErrNotFoundEnv
	}
	return nil
}
