package tgclient

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/tg"
	"github.com/sirupsen/logrus"
)

const (
	defaultPageSize = 100
	maxPageSize     = 100
	stateFileName   = ".tgsync.state.json"
)

type syncState struct {
	OffsetID int `json:"offset_id"`
}

func SyncChannel(ctx context.Context, cfg Config, logger *logrus.Logger) (*Stats, error) {
	if cfg.AppID <= 0 || cfg.AppHash == "" || cfg.Phone == "" {
		return nil, errors.New("app_id/app_hash/phone are required")
	}
	if cfg.Channel == "" {
		cfg.Channel = "aigc1024"
	}
	if cfg.OutputDir == "" {
		cfg.OutputDir = "./frontend/dist/aigc1024"
	}
	if cfg.SessionFile == "" {
		cfg.SessionFile = "./data/tg/aigc1024.session.json"
	}
	if cfg.PageSize <= 0 || cfg.PageSize > maxPageSize {
		cfg.PageSize = defaultPageSize
	}
	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(cfg.OutputDir, "media"), 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(cfg.SessionFile), 0o755); err != nil {
		return nil, err
	}

	stats := &Stats{}
	client := telegram.NewClient(cfg.AppID, cfg.AppHash, telegram.Options{
		SessionStorage: &session.FileStorage{Path: cfg.SessionFile},
	})

	err := client.Run(ctx, func(ctx context.Context) error {
		if err := authWithRetry(ctx, client, cfg.Phone, cfg.Password); err != nil {
			return fmt.Errorf("telegram auth failed: %w", err)
		}

		api := client.API()
		channel, accessHash, err := resolveChannelWithRetry(ctx, api, cfg.Channel, logger)
		if err != nil {
			return err
		}

		logger.Infof("tgsync start channel=%s id=%d", cfg.Channel, channel)
		records, stat, err := collectMessages(ctx, api, cfg, channel, accessHash, logger)
		if err != nil {
			return err
		}
		*stats = *stat

		if err := renderMarkdown(filepath.Join(cfg.OutputDir, "index.md"), records); err != nil {
			return err
		}
		if err := renderHTML(filepath.Join(cfg.OutputDir, "index.html"), cfg.Channel, records); err != nil {
			return err
		}

		logger.Infof("tgsync done channel=%s total=%d kept=%d filtered=%d media=%d", cfg.Channel, stats.TotalMessages, stats.KeptMessages, stats.FilteredMessages, stats.DownloadedMedia)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return stats, nil
}

type terminalAuth struct {
	phone    string
	password string
}

func (a *terminalAuth) Phone(context.Context) (string, error) {
	return a.phone, nil
}

func (a *terminalAuth) Password(context.Context) (string, error) {
	if strings.TrimSpace(a.password) == "" {
		fmt.Print("Enter Telegram 2FA password (leave blank to cancel): ")
		pwd, err := readLineTrimmed()
		if err != nil {
			return "", err
		}
		a.password = pwd
	}
	if strings.TrimSpace(a.password) == "" {
		return "", auth.ErrPasswordNotProvided
	}
	return a.password, nil
}

func (a *terminalAuth) AcceptTermsOfService(context.Context, tg.HelpTermsOfService) error {
	return errors.New("sign up flow is not supported")
}

func (a *terminalAuth) SignUp(context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, errors.New("sign up flow is not supported")
}

func (a *terminalAuth) Code(context.Context, *tg.AuthSentCode) (string, error) {
	fmt.Print("Enter Telegram login code: ")
	return readLineTrimmed()
}

func termAuth(phone, password string) auth.UserAuthenticator {
	return &terminalAuth{phone: phone, password: password}
}

func readLineTrimmed() (string, error) {
	in := bufio.NewReader(os.Stdin)
	line, err := in.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func authWithRetry(ctx context.Context, client *telegram.Client, phone, password string) error {
	var lastErr error
	for i := 0; i < 3; i++ {
		err := client.Auth().IfNecessary(ctx, auth.NewFlow(termAuth(phone, password), auth.SendCodeOptions{}))
		if err == nil {
			return nil
		}
		lastErr = err
		errUpper := strings.ToUpper(err.Error())
		if strings.Contains(errUpper, "PHONE_CODE_EXPIRED") {
			fmt.Println("Telegram login code expired, please enter the latest code.")
			continue
		}
		if errors.Is(err, auth.ErrPasswordNotProvided) || strings.Contains(errUpper, "PASSWORD REQUESTED BUT NOT PROVIDED") {
			return fmt.Errorf("2FA password required: pass --password or set tg_client.password / TG_CLIENT_PASSWORD")
		}
		return err
	}
	return fmt.Errorf("auth flow failed after retries: %w", lastErr)
}

func resolveChannelWithRetry(ctx context.Context, api *tg.Client, channel string, logger *logrus.Logger) (int64, int64, error) {
	var lastErr error
	for i := 0; i < 4; i++ {
		res, err := api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{Username: strings.TrimPrefix(channel, "@")})
		if err == nil {
			for _, c := range res.Chats {
				if ch, ok := c.(*tg.Channel); ok {
					return ch.ID, ch.AccessHash, nil
				}
			}
			return 0, 0, errors.New("resolved chat is not a channel")
		}
		lastErr = err
		wait := floodWait(err)
		if wait <= 0 {
			wait = time.Duration(i+1) * 2 * time.Second
		}
		logger.Warnf("resolve channel retry %d err=%v wait=%s", i+1, err, wait)
		select {
		case <-ctx.Done():
			return 0, 0, ctx.Err()
		case <-time.After(wait):
		}
	}
	return 0, 0, fmt.Errorf("resolve channel failed: %w", lastErr)
}

func collectMessages(ctx context.Context, api *tg.Client, cfg Config, channelID, accessHash int64, logger *logrus.Logger) ([]MessageRecord, *Stats, error) {
	peer := &tg.InputPeerChannel{ChannelID: channelID, AccessHash: accessHash}
	dl := downloader.NewDownloader()
	stats := &Stats{}
	records := make([]MessageRecord, 0, 1024)
	statePath := filepath.Join(cfg.OutputDir, stateFileName)

	offsetID := 0
	if st, err := loadSyncState(statePath); err == nil && st.OffsetID > 0 {
		offsetID = st.OffsetID
		logger.Infof("tgsync resume from offset_id=%d", offsetID)
	}
	for {
		batch, err := getHistoryWithRetry(ctx, api, peer, offsetID, cfg.PageSize)
		if err != nil {
			return nil, nil, err
		}
		if len(batch.Messages) == 0 {
			break
		}

		for _, mc := range batch.Messages {
			msg, ok := mc.(*tg.Message)
			if !ok || msg.Out {
				continue
			}
			stats.TotalMessages++
			if cfg.Limit > 0 && stats.KeptMessages >= cfg.Limit {
				return records, stats, nil
			}

			text := strings.TrimSpace(pickMessageText(msg))
			if shouldFilter(text) {
				stats.FilteredMessages++
				continue
			}

			rec := MessageRecord{ID: msg.ID, Date: time.Unix(int64(msg.Date), 0).UTC(), Text: text, Permalink: fmt.Sprintf("https://t.me/%s/%d", strings.TrimPrefix(cfg.Channel, "@"), msg.ID)}

			media, err := downloadMessageMedia(ctx, api, dl, cfg.OutputDir, msg, logger)
			if err != nil {
				logger.Warnf("download media msg=%d err=%v", msg.ID, err)
			}
			rec.Media = media
			stats.DownloadedMedia += len(media)

			if rec.Text == "" && len(rec.Media) == 0 {
				stats.FilteredMessages++
				continue
			}

			records = append(records, rec)
			stats.KeptMessages++
		}

		minID := 0
		for _, mc := range batch.Messages {
			if m, ok := mc.(*tg.Message); ok {
				if minID == 0 || m.ID < minID {
					minID = m.ID
				}
			}
		}
		if minID == 0 || minID == offsetID {
			break
		}
		offsetID = minID
		if err := saveSyncState(statePath, syncState{OffsetID: offsetID}); err != nil {
			logger.Warnf("save sync state failed: %v", err)
		}
	}

	if err := os.Remove(statePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		logger.Warnf("remove sync state failed: %v", err)
	}
	return records, stats, nil
}

func getHistoryWithRetry(ctx context.Context, api *tg.Client, peer tg.InputPeerClass, offsetID, limit int) (*tg.MessagesChannelMessages, error) {
	var lastErr error
	for i := 0; i < 5; i++ {
		resp, err := api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{Peer: peer, OffsetID: offsetID, Limit: limit})
		if err == nil {
			if m, ok := resp.(*tg.MessagesChannelMessages); ok {
				return m, nil
			}
			if m, ok := resp.(*tg.MessagesMessages); ok {
				return &tg.MessagesChannelMessages{Messages: m.Messages, Chats: m.Chats, Users: m.Users, Count: len(m.Messages)}, nil
			}
			return nil, errors.New("unsupported messages history response")
		}
		lastErr = err
		wait := floodWait(err)
		if wait <= 0 {
			wait = time.Duration(i+1) * time.Second
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}
	return nil, fmt.Errorf("messages.getHistory failed: %w", lastErr)
}

func pickMessageText(msg *tg.Message) string {
	if strings.TrimSpace(msg.Message) != "" {
		return msg.Message
	}
	if msg.Media == nil {
		return ""
	}
	if media, ok := msg.Media.(*tg.MessageMediaDocument); ok {
		if media.Document != nil {
			if d, ok := media.Document.(*tg.Document); ok {
				for _, a := range d.Attributes {
					if c, ok := a.(*tg.DocumentAttributeFilename); ok {
						return c.FileName
					}
				}
			}
		}
	}
	return ""
}

func loadSyncState(path string) (*syncState, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var st syncState
	if err := json.Unmarshal(b, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

func saveSyncState(path string, st syncState) error {
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
