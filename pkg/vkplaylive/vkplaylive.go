package vkplaylive

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	wsConnectionAddr = "wss://pubsub.live.vkplay.ru/connection/websocket"
	wsTokenURL       = "https://api.live.vkplay.ru/v1/ws/connect"
	originURL        = "https://live.vkplay.ru"
)

type wsMessage struct {
	ID     int         `json:"id"`
	Method int         `json:"method,omitempty"`
	Params interface{} `json:"params,omitempty"`
	Result interface{} `json:"result"`
}

type TokenResponse struct {
	Token string `json:"token"`
}

type BlogResponse struct {
	PublicWebSocketChannel string `json:"publicWebSocketChannel"`
	BlogUrl                string `json:"blogUrl"`
}

type MessageSubjectType string

const ( // TODO: add link type
	MessageSubjectTypeText  MessageSubjectType = "text"
	MessageSubjectTypeSmile MessageSubjectType = "smile"
)

type MessageSubject struct {
	Type    MessageSubjectType
	Content string
}

type Message struct {
	Data   []MessageSubject
	Author string
	Time   int64
}

type rawMessageData struct {
	Result struct {
		Data struct {
			Data struct {
				Data struct {
					Author struct {
						Name string `json:"displayName"`
					} `json:"author"`
					CreatedAt int64 `json:"createdAt"`
					Data      []struct {
						Type    string `json:"type"`
						Content string `json:"content"`
						Name    string `json:"name"`
					} `json:"data"`
				} `json:"data"`
			} `json:"data"`
		} `json:"data"`
	} `json:"result"`
}

func getBlog(channelUrl string) (*BlogResponse, error) {
	url := fmt.Sprintf("https://api.live.vkplay.ru/v1/blog/%s", channelUrl)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get response: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var blogResponse BlogResponse
	err = json.Unmarshal(body, &blogResponse)
	if err != nil {
		return nil, err
	}

	return &blogResponse, nil
}

func getWebSocketToken() (string, error) {
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodGet, wsTokenURL, nil)
	if err != nil {
		return "", err
	}

	fromID := uuid.New().String()
	req.Header.Set("X-From-Id", fromID)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get response: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var tokenResponse map[string]string
	err = json.Unmarshal(body, &tokenResponse)
	if err != nil {
		return "", err
	}
	if _, ok := tokenResponse["token"]; !ok {
		return "", errors.New("token not found")
	}

	return tokenResponse["token"], nil
}

func initWebSocket(c *websocket.Conn, token string) error {
	initMessage := wsMessage{
		Params: map[string]interface{}{
			"token": token,
			"name":  "js",
		},
	}
	return invokeMethod(c, &initMessage)
}

func invokeMethod(c *websocket.Conn, message *wsMessage) error {
	message.ID = 1
	err := c.WriteJSON(message)
	return err
}

func createMessage(rawMsg rawMessageData) (Message, error) {
	subjs := make([]MessageSubject, len(rawMsg.Result.Data.Data.Data.Data))
	for i, subj := range rawMsg.Result.Data.Data.Data.Data {
		subjs[i].Type = MessageSubjectType(subj.Type)
		if subjs[i].Type == MessageSubjectTypeSmile {
			subjs[i].Content = subj.Name
		} else if subjs[i].Type == MessageSubjectTypeText {
			if subj.Content == "" {
				continue
			}
			var contentSlice []interface{}
			if err := json.Unmarshal([]byte(subj.Content), &contentSlice); err != nil {
				return Message{}, fmt.Errorf("error decoding message content JSON: %w", err)
			}
			subjs[i].Content = contentSlice[0].(string)
		}
	}

	return Message{
		Author: rawMsg.Result.Data.Data.Data.Author.Name,
		Time:   rawMsg.Result.Data.Data.Data.CreatedAt,
		Data:   subjs,
	}, nil
}

type Client struct {
	msgHandler func(Message)
	channel    string
	authToken  string
	client     *http.Client
	ws         *websocket.Conn
}

func NewAnonymousClient() *Client {
	return &Client{}
}

func NewClient(authToken string) *Client {
	return &Client{
		authToken: authToken,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Add handler to new messages from chat.
//
// Handlers starts in one goroutine to guarantee message ordering.
func (c *Client) OnMessage(f func(msg Message)) {
	c.msgHandler = f
}

func (c *Client) Join(channel string) {
	c.channel = channel
}

func (c *Client) handleMessages() error {
	for {
		var rawMessage rawMessageData
		if err := c.ws.ReadJSON(&rawMessage); err != nil {
			return fmt.Errorf("read error: %w", err) // TODO: do continue, not return
		}

		msg, err := createMessage(rawMessage)
		if err != nil || msg.Time == 0 {
			continue
		}

		c.msgHandler(msg)
	}
}

func (c *Client) Connect() error {
	headers := http.Header{}
	headers.Add("Origin", originURL)

	var err error
	c.ws, _, err = websocket.DefaultDialer.Dial(wsConnectionAddr, headers)
	if err != nil {
		return fmt.Errorf("dial error: %w", err)
	}
	defer c.ws.Close()

	token, err := getWebSocketToken()
	if err != nil {
		return fmt.Errorf("unable to get web socket token: %w", err)
	}

	err = initWebSocket(c.ws, token)
	if err != nil {
		return fmt.Errorf("unable to initialize websocket: %w", err)
	}

	blog, err := getBlog(c.channel)
	if err != nil || !strings.Contains(blog.PublicWebSocketChannel, ":") {
		return fmt.Errorf("unable to get blog: %w", err)
	}
	WSChannel := strings.Split(blog.PublicWebSocketChannel, ":")[1]
	fmt.Println("Got WSChannel:", WSChannel)

	connectToChatPayload := wsMessage{
		ID: 0,
		Params: map[string]interface{}{
			"channel": fmt.Sprintf("public-chat:%s", WSChannel),
		},
		Method: 1,
	}
	if err := invokeMethod(c.ws, &connectToChatPayload); err != nil {
		return fmt.Errorf("connect to chat error: %w", err)
	}

	return c.handleMessages()
}

func (c *Client) Disconnect() error {
	return c.ws.Close()
}

type messageBlock struct {
	Type        string `json:"type"`
	Content     string `json:"content"`
	Modificator string `json:"modificator,omitempty"`
	URL         string `json:"url,omitempty"`
	Explicit    bool   `json:"explicit,omitempty"`
}

func (c *Client) SendMessage(message string) error {
	serializedMessage := c.serializeMessage(message)
	serializedMessageJSON, err := json.Marshal(serializedMessage)
	if err != nil {
		return fmt.Errorf("error marshaling message: %v", err)
	}

	body := url.Values{}
	body.Add("data", string(serializedMessageJSON))

	url := fmt.Sprintf("https://api.live.vkplay.ru/v1/blog/%s/public_video_stream/chat", c.channel)
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body.Encode()))
	if err != nil {
		return err
	}

	req.Header.Add("Authorization", "Bearer "+c.authToken)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	fmt.Printf("Got response from vk: %v\n", resp.Header)

	return nil
}

func (c *Client) serializeMessage(message string) []messageBlock {
	var serializedMessage []messageBlock

	if message != "" {
		serializedMessage = append(serializedMessage, getTextBlock(message))
	}

	return serializedMessage
}

func getTextBlock(text string) messageBlock {
	return messageBlock{Type: "text", Content: fmt.Sprintf(`["%s","unstyled",[]]`, text)}
}
