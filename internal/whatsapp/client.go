package whatsapp

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"

	_ "github.com/lib/pq"

	"github.com/innomon/whatsadk/internal/agent"
	"github.com/innomon/whatsadk/internal/auth"
	"github.com/innomon/whatsadk/internal/config"
	"github.com/innomon/whatsadk/internal/verification"
)

const indiaCountryCode = "91"

type Client struct {
	wac           *whatsmeow.Client
	adkClient     *agent.Client
	verifyHandler *verification.Handler
	cfg           *config.Config
	log           waLog.Logger
}

func New(ctx context.Context, cfg *config.Config, adkClient *agent.Client, verifyHandler *verification.Handler) (*Client, error) {
	log := waLog.Stdout("WhatsApp", cfg.WhatsApp.LogLevel, true)

	container, err := sqlstore.New(ctx, "postgres", cfg.WhatsApp.StoreDSN, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create store: %w", err)
	}

	deviceStore, err := container.GetFirstDevice(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get device: %w", err)
	}

	wac := whatsmeow.NewClient(deviceStore, log)

	client := &Client{
		wac:           wac,
		adkClient:     adkClient,
		verifyHandler: verifyHandler,
		cfg:           cfg,
		log:           log,
	}

	wac.AddEventHandler(client.handleEvent)

	return client, nil
}

func (c *Client) Connect(ctx context.Context) error {
	if c.wac.Store.ID == nil {
		qrChan, err := c.wac.GetQRChannel(ctx)
		if err != nil {
			return fmt.Errorf("failed to get QR channel: %w", err)
		}

		if err := c.wac.Connect(); err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}

		for evt := range qrChan {
			switch evt.Event {
			case "code":
				fmt.Println("\n📱 Scan this QR code with WhatsApp:")
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
				fmt.Println()
			case "success":
				fmt.Println("✅ Successfully logged in!")
				return nil
			case "timeout":
				return fmt.Errorf("QR code scan timeout")
			default:
				if evt.Error != nil {
					return fmt.Errorf("QR error: %w", evt.Error)
				}
			}
		}
	} else {
		if err := c.wac.Connect(); err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
		fmt.Println("✅ Connected using existing session")
	}

	return nil
}

func (c *Client) Run(ctx context.Context) error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	fmt.Println("🤖 WhatsApp-ADK Gateway is running. Press Ctrl+C to stop.")

	select {
	case <-ctx.Done():
		c.log.Infof("Context cancelled, disconnecting...")
	case <-sigChan:
		c.log.Infof("Received interrupt signal, disconnecting...")
	}

	c.wac.Disconnect()
	return nil
}

func (c *Client) handleEvent(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		c.handleMessage(v)
	case *events.Connected:
		c.log.Infof("Connected to WhatsApp")
	case *events.Disconnected:
		c.log.Infof("Disconnected from WhatsApp")
	case *events.LoggedOut:
		c.log.Warnf("Logged out from WhatsApp")
	}
}

func (c *Client) handleMessage(msg *events.Message) {
	if msg.Info.IsFromMe {
		return
	}

	if msg.Info.IsGroup {
		return
	}

	text := extractText(msg)
	if text == "" {
		return
	}

	userID := msg.Info.Sender.User
	c.log.Infof("Received message from %s: %s", userID, truncate(text, 80))

	if c.verifyHandler != nil && auth.IsVerificationToken(text) != nil {
		ctx := context.Background()
		response := c.verifyHandler.Handle(ctx, userID, text)
		if response != "" {
			_, err := c.wac.SendMessage(ctx, msg.Info.Chat, &waE2E.Message{
				Conversation: proto.String(response),
			})
			if err != nil {
				c.log.Errorf("Failed to send verification response: %v", err)
			}
			return
		}
	}

	if !c.isUserAllowed(userID) {
		c.log.Infof("Blocked message from non-allowed user %s", userID)
		ctx := context.Background()
		_, err := c.wac.SendMessage(ctx, msg.Info.Chat, &waE2E.Message{
			Conversation: proto.String("Sorry, we only entertain friends from India."),
		})
		if err != nil {
			c.log.Errorf("Failed to send rejection message: %v", err)
		}
		return
	}

	ctx := context.Background()
	response, err := c.adkClient.Chat(ctx, userID, text)
	if err != nil {
		c.log.Errorf("Failed to get agent response: %v", err)
		response = "Sorry, I encountered an error processing your message. Please try again."
	}

	if response == "" {
		return
	}

	_, err = c.wac.SendMessage(ctx, msg.Info.Chat, &waE2E.Message{
		Conversation: proto.String(response),
	})
	if err != nil {
		c.log.Errorf("Failed to send message: %v", err)
	} else {
		c.log.Infof("Sent response to %s: %s", userID, truncate(response, 50))
	}
}

func (c *Client) isUserAllowed(userID string) bool {
	if c.cfg.IsUserWhitelisted(userID) {
		return true
	}
	return strings.HasPrefix(userID, indiaCountryCode)
}

func extractText(msg *events.Message) string {
	if msg.Message == nil {
		return ""
	}

	if msg.Message.Conversation != nil {
		return *msg.Message.Conversation
	}

	if msg.Message.ExtendedTextMessage != nil && msg.Message.ExtendedTextMessage.Text != nil {
		return *msg.Message.ExtendedTextMessage.Text
	}

	return ""
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
