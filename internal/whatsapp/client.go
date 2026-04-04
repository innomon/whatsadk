package whatsapp

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"

	_ "github.com/lib/pq"

	"github.com/innomon/whatsadk/internal/agent"
	"github.com/innomon/whatsadk/internal/auth"
	"github.com/innomon/whatsadk/internal/config"
	"github.com/innomon/whatsadk/internal/store"
	"github.com/innomon/whatsadk/internal/verification"
)

const indiaCountryCode = "91"

type Client struct {
	wac           *whatsmeow.Client
	adkClient     *agent.Client
	verifyHandler *verification.Handler
	oauthHandler  *auth.OAuthHandler
	store         *store.Store
	cfg           *config.Config
	log           waLog.Logger
}

func New(ctx context.Context, cfg *config.Config, adkClient *agent.Client, verifyHandler *verification.Handler, oauthHandler *auth.OAuthHandler, store *store.Store) (*Client, error) {
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
		oauthHandler:  oauthHandler,
		store:         store,
		cfg:           cfg,
		log:           log,
	}

	wac.AddEventHandler(client.handleEvent)

	return client, nil
}

func (c *Client) storeRequest(ctx context.Context, userID, uniqueID string, content []byte, ts time.Time) {
	if c.store == nil {
		return
	}
	path := fmt.Sprintf("whatsmeow/%s/%s/request", userID, uniqueID)
	metadata := map[string]interface{}{
		"mime_type": "text/plain",
		"metadata":  map[string]interface{}{},
	}
	if err := c.store.PutFile(ctx, path, metadata, content, ts); err != nil {
		c.log.Errorf("Failed to store request to filesys: %v", err)
	}
}

func (c *Client) storeResponse(ctx context.Context, userID, uniqueID string, content []byte, ts time.Time, errStr string) {
	if c.store == nil {
		return
	}
	path := fmt.Sprintf("whatsmeow/%s/%s/response", userID, uniqueID)
	innerMetadata := map[string]interface{}{}
	if errStr != "" {
		innerMetadata["error"] = errStr
	}
	metadata := map[string]interface{}{
		"mime_type": "text/plain",
		"metadata":  innerMetadata,
	}
	if err := c.store.PutFile(ctx, path, metadata, content, ts); err != nil {
		c.log.Errorf("Failed to store response to filesys: %v", err)
	}
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
	case *events.HistorySync:
		c.handleHistorySync(v)
	case *events.Connected:
		c.log.Infof("Connected to WhatsApp")
	case *events.Disconnected:
		c.log.Infof("Disconnected from WhatsApp")
	case *events.LoggedOut:
		c.log.Warnf("Logged out from WhatsApp")
	}
}

func (c *Client) handleHistorySync(v *events.HistorySync) {
	c.log.Infof("Received history sync (type: %s)", v.Data.SyncType)
	for _, conv := range v.Data.GetConversations() {
		chatJID, _ := types.ParseJID(conv.GetID())
		if chatJID.IsEmpty() || chatJID.Server != types.DefaultUserServer {
			continue
		}

		for _, historyMsg := range conv.GetMessages() {
			msg, err := c.wac.ParseWebMessage(chatJID, historyMsg.GetMessage())
			if err != nil {
				continue
			}

			text := extractText(msg)
			if text == "" {
				continue
			}

			ctx := context.Background()
			if msg.Info.IsFromMe {
				// Store as response
				userID := msg.Info.Chat.User
				c.storeResponse(ctx, userID, msg.Info.ID, []byte(text), msg.Info.Timestamp, "")
			} else {
				// Store as request
				userID := msg.Info.Sender.User
				c.storeRequest(ctx, userID, msg.Info.ID, []byte(text), msg.Info.Timestamp)
			}
		}
	}
}

func (c *Client) handleMessage(msg *events.Message) {
	if msg.Info.IsGroup {
		return
	}

	text := extractText(msg)
	if text == "" {
		return
	}

	// Handle messages sent from me (e.g., from another device)
	if msg.Info.IsFromMe {
		userID := msg.Info.Chat.User
		uniqueID := msg.Info.ID
		ctx := context.Background()
		c.storeResponse(ctx, userID, uniqueID, []byte(text), msg.Info.Timestamp, "")
		return
	}

	// Handle LID resolution to phone number (PN)
	sender := msg.Info.Sender
	if sender.Server == types.HiddenUserServer {
		ctx := context.Background()
		resolved := c.resolveLID(ctx, sender)
		if resolved.Server == types.DefaultUserServer {
			c.log.Infof("Resolved LID %s to PN %s", sender.String(), resolved.String())
			sender = resolved
		}
	}

	// Prefer phone number for internal logic/blocking
	userID := sender.User
	displayID := sender.String()
	uniqueID := msg.Info.ID

	c.log.Infof("Received message from %s: %s", displayID, truncate(text, 80))

	// Global Blacklist Check
	if c.store != nil {
		ctx := context.Background()
		// Check both raw ID and full JID string
		blocked, _ := c.store.IsBlacklisted(ctx, userID)
		if !blocked {
			blocked, _ = c.store.IsBlacklisted(ctx, displayID)
		}

		if blocked {
			c.log.Warnf("Blocking message from blacklisted user: %s", displayID)
			return
		}
	}

	// Store the incoming request
	ctx := context.Background()
	c.storeRequest(ctx, userID, uniqueID, []byte(text), msg.Info.Timestamp)

	if c.verifyHandler != nil && auth.IsVerificationToken(text) != nil {
		response := c.verifyHandler.Handle(ctx, userID, text)
		if response != "" {
			resp, err := c.wac.SendMessage(ctx, msg.Info.Chat, &waE2E.Message{
				Conversation: proto.String(response),
			})
			if err != nil {
				c.log.Errorf("Failed to send verification response: %v", err)
				c.storeResponse(ctx, userID, uniqueID, []byte(response), time.Now(), err.Error())
			} else {
				c.storeResponse(ctx, userID, uniqueID, []byte(response), resp.Timestamp, "")
			}
			return
		}
	}

	if c.oauthHandler != nil && auth.IsAuthCommand(text) {
		response, err := c.oauthHandler.Handle(userID, text)
		var errStr string
		if err != nil {
			c.log.Errorf("OAuth handler error: %v", err)
			response = "⚠️ Something went wrong processing your AUTH request. Please try again."
			errStr = err.Error()
		}
		if response != "" {
			resp, err := c.wac.SendMessage(ctx, msg.Info.Chat, &waE2E.Message{
				Conversation: proto.String(response),
			})
			if err != nil {
				c.log.Errorf("Failed to send OAuth response: %v", err)
				c.storeResponse(ctx, userID, uniqueID, []byte(response), time.Now(), err.Error())
			} else {
				c.storeResponse(ctx, userID, uniqueID, []byte(response), resp.Timestamp, errStr)
			}
		}
		return
	}

	if !c.isUserAllowed(msg.Info.Sender) {
		c.log.Infof("Blocked message from non-allowed user %s", msg.Info.Sender.String())
		response := "Sorry, we only entertain friends from India."
		resp, err := c.wac.SendMessage(ctx, msg.Info.Chat, &waE2E.Message{
			Conversation: proto.String(response),
		})
		if err != nil {
			c.log.Errorf("Failed to send rejection message: %v", err)
			c.storeResponse(ctx, userID, uniqueID, []byte(response), time.Now(), err.Error())
		} else {
			c.storeResponse(ctx, userID, uniqueID, []byte(response), resp.Timestamp, "")
		}
		return
	}

	response, err := c.adkClient.Chat(ctx, userID, text)
	var errStr string
	if err != nil {
		c.log.Errorf("Failed to get agent response: %v", err)
		response = "Sorry, I encountered an error processing your message. Please try again."
		errStr = err.Error()
	}

	if response == "" {
		return
	}

	resp, err := c.wac.SendMessage(ctx, msg.Info.Chat, &waE2E.Message{
		Conversation: proto.String(response),
	})
	if err != nil {
		c.log.Errorf("Failed to send message: %v", err)
		c.storeResponse(ctx, userID, uniqueID, []byte(response), time.Now(), err.Error())
	} else {
		c.log.Infof("Sent response to %s: %s", userID, truncate(response, 50))
		c.storeResponse(ctx, userID, uniqueID, []byte(response), resp.Timestamp, errStr)
	}
}

func (c *Client) isUserAllowed(jid types.JID) bool {
	// If it's a LID, try resolving it to PN first for better whitelist/country checking
	if jid.Server == types.HiddenUserServer {
		ctx := context.Background()
		resolved := c.resolveLID(ctx, jid)
		if resolved.Server == types.DefaultUserServer {
			jid = resolved
		}
	}

	// 1. If whitelisting is active, check it
	if len(c.cfg.WhatsApp.WhitelistedUsers) > 0 {
		if c.cfg.IsUserWhitelisted(jid.User) || c.cfg.IsUserWhitelisted(jid.String()) {
			return true
		}
	} else {
		// 2. If NO whitelisting is active, we allow everyone except if we want to enforce 91 prefix
		return true
	}

	// 3. Fallback to country check if whitelist exists but user is not in it
	if jid.Server == types.DefaultUserServer && strings.HasPrefix(jid.User, indiaCountryCode) {
		return true
	}

	// 4. Handle LID (Linked ID) if still not resolved
	if jid.Server == types.HiddenUserServer {
		c.log.Infof("LID detected and unresolved: %s. Allowing LID for whitelisted mode.", jid.String())
		return true
	}

	return false
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

func (c *Client) resolveLID(ctx context.Context, lid types.JID) types.JID {
	if lid.Server != types.HiddenUserServer {
		return lid
	}

	// 1. Check local store first (LIDStore is direct for PN/LID mapping)
	if c.wac.Store.LIDs != nil {
		pn, err := c.wac.Store.LIDs.GetPNForLID(ctx, lid)
		if err == nil && !pn.IsEmpty() {
			return pn
		}
	}

	// 2. Not in store, try fetching from WhatsApp servers (one-time discovery)
	info, err := c.wac.GetUserInfo(ctx, []types.JID{lid})
	if err != nil {
		c.log.Warnf("Failed to get user info for LID %s: %v", lid, err)
		return lid
	}

	if ui, ok := info[lid]; ok && !ui.LID.IsEmpty() {
		// When querying with a LID, the LID field in UserInfo typically contains the PN
		if ui.LID.Server == types.DefaultUserServer {
			return ui.LID
		}
	}

	return lid
}
