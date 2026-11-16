package main

import (
	"context"
	"database/sql"
	"image"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
	_ "modernc.org/sqlite"
)

type Message struct {
	ID     string
	Chat   string
	Sender string
	Text   string
	FromMe bool
	Time   time.Time
	Thumb  []byte
}

type Chat struct {
	JID     string
	Name    string
	Preview string
	Unread  int
}

type App struct {
	Client  *whatsmeow.Client
	Updates chan struct{}

	dir         string
	db          *sql.DB
	blockCache  sync.Map
	sem         chan struct{}
	mu          sync.Mutex
	status      string
	current     string
	focused     bool
	lastNotify  map[string]time.Time
	lastAny     time.Time
	quietUntil  time.Time
	notifier    func(title, body string) error
	groups      map[string]string
	fetched     map[string]bool
	opened      map[string]bool
	avatars     map[string]image.Image
	avatarTried map[string]bool
}

func OpenApp(dir string) (*App, error) {
	ctx := context.Background()
	pragmas := "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"

	container, err := sqlstore.New(ctx, "sqlite", "file:"+filepath.Join(dir, "session.db")+pragmas, waLog.Noop)
	if err != nil {
		return nil, err
	}
	device, err := container.GetFirstDevice(ctx)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", "file:"+filepath.Join(dir, "messages.db")+pragmas)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS messages (
		id TEXT, chat TEXT, sender TEXT, text TEXT, from_me INTEGER, ts INTEGER,
		PRIMARY KEY (chat, id)
	)`)
	if err != nil {
		return nil, err
	}
	db.Exec(`ALTER TABLE messages ADD COLUMN thumb BLOB`)
	db.Exec(`CREATE TABLE IF NOT EXISTS reads (chat TEXT PRIMARY KEY, ts INTEGER)`)
	db.Exec(`INSERT OR IGNORE INTO reads (chat, ts) SELECT chat, MAX(ts) FROM messages GROUP BY chat`)

	app := &App{
		Updates:     make(chan struct{}, 1),
		dir:         dir,
		db:          db,
		sem:         make(chan struct{}, 3),
		status:      "menghubungkan...",
		focused:     true,
		notifier:    notify,
		lastNotify:  map[string]time.Time{},
		groups:      map[string]string{},
		fetched:     map[string]bool{},
		opened:      map[string]bool{},
		avatars:     map[string]image.Image{},
		avatarTried: map[string]bool{},
	}
	app.Client = whatsmeow.NewClient(device, waLog.Noop)
	app.Client.AddEventHandler(app.handle)
	app.mergeChats(app.resolveLID)
	return app, nil
}

func (a *App) Status() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.status
}

func (a *App) setStatus(s string) {
	a.mu.Lock()
	a.status = s
	a.mu.Unlock()
}

func (a *App) notify() {
	select {
	case a.Updates <- struct{}{}:
	default:
	}
}

func (a *App) handle(evt any) {
	switch e := evt.(type) {
	case *events.Message:
		a.save(e, true)
	case *events.HistorySync:
		for _, conv := range e.Data.GetConversations() {
			jid, err := types.ParseJID(conv.GetID())
			if err != nil {
				continue
			}
			if jid.Server == types.GroupServer {
				name := conv.GetName()
				if name == "" {
					name = conv.GetDisplayName()
				}
				if name != "" {
					a.mu.Lock()
					a.groups[jid.String()] = name
					a.mu.Unlock()
				}
			}
			for _, item := range conv.GetMessages() {
				if msg, err := a.Client.ParseWebMessage(jid, item.GetMessage()); err == nil {
					a.save(msg, false)
				}
			}
		}
	case *events.Connected:
		a.setStatus("online")
		a.mu.Lock()
		a.quietUntil = time.Now().Add(15 * time.Second)
		a.mu.Unlock()
		a.mu.Lock()
		a.fetched = map[string]bool{}
		a.avatarTried = map[string]bool{}
		a.mu.Unlock()
		go a.loadGroups()
	case *events.Disconnected:
		a.setStatus("terputus, mencoba lagi...")
	case *events.LoggedOut:
		a.setStatus("keluar dari HP, hapus folder data lalu login ulang")
	}
	a.notify()
}

func (a *App) loadGroups() {
	groups, err := a.Client.GetJoinedGroups(context.Background())
	if err != nil {
		return
	}
	a.mu.Lock()
	for _, g := range groups {
		a.groups[g.JID.String()] = g.Name
	}
	a.mu.Unlock()
	a.notify()
}

func (a *App) fetchGroup(jid types.JID) {
	if !a.Client.IsConnected() {
		return
	}
	info, err := a.Client.GetGroupInfo(context.Background(), jid)
	if err != nil || info.Name == "" {
		return
	}
	a.mu.Lock()
	a.groups[jid.String()] = info.Name
	a.mu.Unlock()
	a.notify()
}

func textOf(m *waE2E.Message) string {
	switch {
	case m.GetConversation() != "":
		return m.GetConversation()
	case m.GetExtendedTextMessage() != nil:
		return m.GetExtendedTextMessage().GetText()
	case m.GetImageMessage() != nil:
		return strings.TrimSpace("[foto] " + m.GetImageMessage().GetCaption())
	case m.GetVideoMessage() != nil:
		return strings.TrimSpace("[video] " + m.GetVideoMessage().GetCaption())
	case m.GetAudioMessage() != nil:
		return "[audio]"
	case m.GetDocumentMessage() != nil:
		return "[dokumen] " + m.GetDocumentMessage().GetFileName()
	case m.GetStickerMessage() != nil:
		return "[stiker]"
	}
	return ""
}

func (a *App) save(e *events.Message, live bool) {
	if e.Info.Chat.Server == types.BroadcastServer {
		return
	}
	text := textOf(e.Message)
	if text == "" {
		return
	}
	thumb := thumbOf(e.Message)
	chat := a.resolveLID(e.Info.Chat.String())
	a.insert(Message{
		ID:     e.Info.ID,
		Chat:   chat,
		Sender: e.Info.Sender.String(),
		Text:   text,
		FromMe: e.Info.IsFromMe,
		Time:   e.Info.Timestamp,
		Thumb:  thumb,
	})
	if live && downloadable(e.Message) != nil {
		go a.fetchThumb(e, chat)
	}
	if live && !e.Info.IsFromMe {
		a.alert(chat, e.Info.Sender.String(), text)
	}
}

func (a *App) setThumb(chat, id string, thumb []byte) {
	a.db.Exec(`UPDATE messages SET thumb = ? WHERE chat = ? AND id = ?`, thumb, chat, id)
}

func (a *App) insert(m Message) {
	var thumb any
	if len(m.Thumb) > 0 {
		thumb = m.Thumb
	}
	a.db.Exec(
		`INSERT INTO messages (id, chat, sender, text, from_me, ts, thumb) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (chat, id) DO UPDATE SET thumb = excluded.thumb
		WHERE messages.thumb IS NULL AND excluded.thumb IS NOT NULL`,
		m.ID, m.Chat, m.Sender, m.Text, m.FromMe, m.Time.Unix(), thumb,
	)
}

func (a *App) Chats() []Chat {
	var chats []Chat
	seen := map[string]bool{}

	a.mu.Lock()
	for jid := range a.opened {
		chats = append(chats, Chat{JID: jid, Name: ""})
		seen[jid] = true
	}
	a.mu.Unlock()

	rows, err := a.db.Query(`SELECT chat,
		(SELECT text FROM messages m2 WHERE m2.chat = m.chat ORDER BY ts DESC, rowid DESC LIMIT 1),
		(SELECT COUNT(*) FROM messages m3 WHERE m3.chat = m.chat AND m3.from_me = 0
			AND m3.ts > COALESCE((SELECT ts FROM reads r WHERE r.chat = m.chat), 0))
		FROM messages m GROUP BY chat ORDER BY MAX(ts) DESC`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var jid, preview string
			var unread int
			rows.Scan(&jid, &preview, &unread)
			if !seen[jid] {
				chats = append(chats, Chat{JID: jid, Preview: strings.ReplaceAll(preview, "\n", " "), Unread: unread})
			}
		}
	}
	for i := range chats {
		chats[i].Name = a.Name(chats[i].JID)
	}
	return chats
}

func (a *App) Messages(chat string, limit int) []Message {
	rows, err := a.db.Query(
		`SELECT id, sender, text, from_me, ts, thumb FROM messages WHERE chat = ? ORDER BY ts DESC, rowid DESC LIMIT ?`,
		chat, limit,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		m := Message{Chat: chat}
		var ts int64
		rows.Scan(&m.ID, &m.Sender, &m.Text, &m.FromMe, &ts, &m.Thumb)
		m.Time = time.Unix(ts, 0)
		msgs = append([]Message{m}, msgs...)
	}
	return msgs
}

func (a *App) Name(jidStr string) string {
	jid, err := types.ParseJID(jidStr)
	if err != nil {
		return jidStr
	}
	if jid.Server == types.GroupServer {
		key := jid.String()
		a.mu.Lock()
		name := a.groups[key]
		fetch := name == "" && !a.fetched[key]
		if fetch {
			a.fetched[key] = true
		}
		a.mu.Unlock()
		if fetch {
			go a.fetchGroup(jid)
		}
		if name != "" {
			return name
		}
		return "Grup " + jid.User
	}

	ctx := context.Background()
	lookup := jid
	store := a.Client.Store
	if jid.Server == types.HiddenUserServer && store.LIDs != nil {
		if pn, err := store.LIDs.GetPNForLID(ctx, jid); err == nil && !pn.IsEmpty() {
			lookup = pn
		}
	}
	if store.Contacts == nil {
		return lookup.User
	}
	if info, err := store.Contacts.GetContact(ctx, lookup); err == nil {
		for _, name := range []string{info.FullName, info.FirstName, info.PushName, info.BusinessName} {
			if name != "" {
				return name
			}
		}
	}
	return lookup.User
}

func (a *App) Open(number string) string {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, number)
	if len(digits) < 8 {
		return ""
	}
	jid := types.NewJID(digits, types.DefaultUserServer).String()
	a.mu.Lock()
	a.opened[jid] = true
	a.mu.Unlock()
	return jid
}

func (a *App) Send(chat, text string) error {
	jid, err := types.ParseJID(chat)
	if err != nil {
		return err
	}
	resp, err := a.Client.SendMessage(context.Background(), jid, &waE2E.Message{Conversation: proto.String(text)})
	if err != nil {
		return err
	}
	a.insert(Message{ID: resp.ID, Chat: chat, Text: text, FromMe: true, Time: resp.Timestamp})
	a.notify()
	return nil
}

func (a *App) MarkRead(chat string) {
	a.db.Exec(
		`INSERT INTO reads (chat, ts) VALUES (?, (SELECT COALESCE(MAX(ts), 0) FROM messages WHERE chat = ?))
		ON CONFLICT (chat) DO UPDATE SET ts = excluded.ts`,
		chat, chat,
	)
}

func (a *App) resolveLID(chat string) string {
	jid, err := types.ParseJID(chat)
	if err != nil || jid.Server != types.HiddenUserServer || a.Client.Store.LIDs == nil {
		return chat
	}
	pn, err := a.Client.Store.LIDs.GetPNForLID(context.Background(), jid)
	if err != nil || pn.IsEmpty() {
		return chat
	}
	return pn.ToNonAD().String()
}

func (a *App) mergeChats(resolve func(string) string) {
	rows, err := a.db.Query(`SELECT DISTINCT chat FROM messages WHERE chat LIKE '%@lid'`)
	if err != nil {
		return
	}
	var lids []string
	for rows.Next() {
		var chat string
		rows.Scan(&chat)
		lids = append(lids, chat)
	}
	rows.Close()

	for _, lid := range lids {
		pn := resolve(lid)
		if pn == lid {
			continue
		}
		a.db.Exec(`INSERT OR IGNORE INTO messages (id, chat, sender, text, from_me, ts, thumb)
			SELECT id, ?, sender, text, from_me, ts, thumb FROM messages WHERE chat = ?`, pn, lid)
		a.db.Exec(`DELETE FROM messages WHERE chat = ?`, lid)
		a.db.Exec(`INSERT INTO reads (chat, ts) SELECT ?, ts FROM reads WHERE chat = ?
			ON CONFLICT (chat) DO UPDATE SET ts = MAX(reads.ts, excluded.ts)`, pn, lid)
		a.db.Exec(`DELETE FROM reads WHERE chat = ?`, lid)
	}
}
