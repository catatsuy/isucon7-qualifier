package main

import (
	"bytes"
	crand "crypto/rand"
	"crypto/sha1"
	"database/sql"
	"encoding/binary"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/go-sql-driver/mysql"
	"github.com/gorilla/sessions"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/middleware"
	_ "github.com/walf443/go-sql-tracer"
)

const (
	avatarMaxBytes = 1 * 1024 * 1024
)

var (
	db            *sqlx.DB
	ErrBadReqeust = echo.NewHTTPError(http.StatusBadRequest)
	develop       *bool
	pool          *redis.Pool

	davHost1 string
	davHost2 string
)

type Renderer struct {
	templates *template.Template
}

func (r *Renderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return r.templates.ExecuteTemplate(w, name, data)
}

func init() {
	develop = flag.Bool("develop", false, "if development, please enable")
	flag.Parse()
	seedBuf := make([]byte, 8)
	crand.Read(seedBuf)
	rand.Seed(int64(binary.LittleEndian.Uint64(seedBuf)))

	db_host := os.Getenv("ISUBATA_DB_HOST")
	if db_host == "" {
		db_host = "127.0.0.1"
	}
	db_port := os.Getenv("ISUBATA_DB_PORT")
	if db_port == "" {
		db_port = "3306"
	}
	db_user := os.Getenv("ISUBATA_DB_USER")
	if db_user == "" {
		db_user = "root"
	}
	db_password := os.Getenv("ISUBATA_DB_PASSWORD")
	if db_password != "" {
		db_password = ":" + db_password
	}

	dsn := fmt.Sprintf("%s%s@tcp(%s:%s)/isubata?parseTime=true&loc=Local&charset=utf8mb4&interpolateParams=true",
		db_user, db_password, db_host, db_port)

	log.Printf("Connecting to db: %q", dsn)
	if *develop {
		db, _ = sqlx.Connect("mysql:trace", dsn)
		davHost1 = "http://59.106.213.149:8080/"
		davHost2 = "http://59.106.215.202:8080/"
	} else {
		db, _ = sqlx.Connect("mysql", dsn)
		davHost1 = "http://192.168.101.1:8080/"
		davHost2 = "http://192.168.101.2:8080/"
	}
	for {
		err := db.Ping()
		if err == nil {
			break
		}
		log.Println(err)
		time.Sleep(time.Second * 3)
	}

	db.SetMaxOpenConns(20)
	db.SetConnMaxLifetime(5 * time.Minute)

	if *develop {
		pool = newPool("127.0.0.1:6379")
	} else {
		pool = newPool("192.168.101.3:6379")
	}
	conn := pool.Get()
	defer conn.Close()
	log.Printf("Succeeded to connect db.")
}

func newPool(addr string) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
		Dial:        func() (redis.Conn, error) { return redis.Dial("tcp", addr) },
	}
}

type User struct {
	ID          int64     `json:"-" db:"id"`
	Name        string    `json:"name" db:"name"`
	Salt        string    `json:"-" db:"salt"`
	Password    string    `json:"-" db:"password"`
	DisplayName string    `json:"display_name" db:"display_name"`
	AvatarIcon  string    `json:"avatar_icon" db:"avatar_icon"`
	CreatedAt   time.Time `json:"-" db:"created_at"`
}

func getUser(userID int64) (*User, error) {
	u := User{}
	if err := db.Get(&u, "SELECT * FROM user WHERE id = ?", userID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func addMessage(channelID, userID int64, content string) (int64, error) {
	res, err := db.Exec(
		"INSERT INTO message (channel_id, user_id, content, created_at) VALUES (?, ?, ?, NOW())",
		channelID, userID, content)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

type Message struct {
	ID        int64     `db:"id"`
	ChannelID int64     `db:"channel_id"`
	UserID    int64     `db:"user_id"`
	Content   string    `db:"content"`
	CreatedAt time.Time `db:"created_at"`
}

func queryMessages(chanID, lastID int64) ([]Message, error) {
	msgs := []Message{}
	err := db.Select(&msgs, "SELECT * FROM message WHERE id > ? AND channel_id = ? ORDER BY id DESC LIMIT 100",
		lastID, chanID)
	return msgs, err
}

func sessUserID(c echo.Context) int64 {
	sess, _ := session.Get("session", c)
	var userID int64
	if x, ok := sess.Values["user_id"]; ok {
		userID, _ = x.(int64)
	}
	return userID
}

func sessSetUserID(c echo.Context, id int64) {
	sess, _ := session.Get("session", c)
	sess.Options = &sessions.Options{
		HttpOnly: true,
		MaxAge:   360000,
	}
	sess.Values["user_id"] = id
	sess.Save(c.Request(), c.Response())
}

func ensureLogin(c echo.Context) (*User, error) {
	var user *User
	var err error

	userID := sessUserID(c)
	if userID == 0 {
		goto redirect
	}

	user, err = getUser(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		sess, _ := session.Get("session", c)
		delete(sess.Values, "user_id")
		sess.Save(c.Request(), c.Response())
		goto redirect
	}
	return user, nil

redirect:
	c.Redirect(http.StatusSeeOther, "/login")
	return nil, nil
}

const LettersAndDigits = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func randomString(n int) string {
	b := make([]byte, n)
	z := len(LettersAndDigits)

	for i := 0; i < n; i++ {
		b[i] = LettersAndDigits[rand.Intn(z)]
	}
	return string(b)
}

func register(name, password string) (int64, error) {
	salt := randomString(20)
	digest := fmt.Sprintf("%x", sha1.Sum([]byte(salt+password)))

	res, err := db.Exec(
		"INSERT INTO user (name, salt, password, display_name, avatar_icon, created_at)"+
			" VALUES (?, ?, ?, ?, ?, NOW())",
		name, salt, digest, name, "default.png")
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// request handlers

func getInitialize(c echo.Context) error {
	db.MustExec("DELETE FROM user WHERE id > 1000")
	db.MustExec("DELETE FROM image WHERE id > 1001")
	db.MustExec("DELETE FROM channel WHERE id > 10")
	db.MustExec("DELETE FROM message WHERE id > 10000")
	db.MustExec("DELETE FROM haveread")
	conn := pool.Get()
	defer conn.Close()
	conn.Do("FLUSHALL")
	return c.String(204, "")
}

func getIndex(c echo.Context) error {
	userID := sessUserID(c)
	if userID != 0 {
		return c.Redirect(http.StatusSeeOther, "/channel/1")
	}

	return c.Render(http.StatusOK, "index", map[string]interface{}{
		"ChannelID": nil,
	})
}

type ChannelInfo struct {
	ID   int64  `db:"id"`
	Name string `db:"name"`
}

type Description struct {
	Description string `db:"description"`
}

func getChannel(c echo.Context) error {
	user, err := ensureLogin(c)
	if user == nil {
		return err
	}
	cID, err := strconv.Atoi(c.Param("channel_id"))
	if err != nil {
		return err
	}
	channels := []ChannelInfo{}
	err = db.Select(&channels, "SELECT id, name FROM channel ORDER BY id")
	if err != nil {
		return err
	}

	descs := make([]Description, 0)
	err = db.Select(&descs, "SELECT description FROM channel WHERE id = ?", cID)
	if err != nil {
		return err
	}

	return c.Render(http.StatusOK, "channel", map[string]interface{}{
		"ChannelID":   cID,
		"Channels":    channels,
		"User":        user,
		"Description": descs[0].Description,
	})
}

func getRegister(c echo.Context) error {
	return c.Render(http.StatusOK, "register", map[string]interface{}{
		"ChannelID": 0,
		"Channels":  []ChannelInfo{},
		"User":      nil,
	})
}

func postRegister(c echo.Context) error {
	name := c.FormValue("name")
	pw := c.FormValue("password")
	if name == "" || pw == "" {
		return ErrBadReqeust
	}
	userID, err := register(name, pw)
	if err != nil {
		if merr, ok := err.(*mysql.MySQLError); ok {
			if merr.Number == 1062 { // Duplicate entry xxxx for key zzzz
				return c.NoContent(http.StatusConflict)
			}
		}
		return err
	}
	sessSetUserID(c, userID)
	return c.Redirect(http.StatusSeeOther, "/")
}

func getLogin(c echo.Context) error {
	return c.Render(http.StatusOK, "login", map[string]interface{}{
		"ChannelID": 0,
		"Channels":  []ChannelInfo{},
		"User":      nil,
	})
}

func postLogin(c echo.Context) error {
	name := c.FormValue("name")
	pw := c.FormValue("password")
	if name == "" || pw == "" {
		return ErrBadReqeust
	}

	var user User
	err := db.Get(&user, "SELECT * FROM user WHERE name = ?", name)
	if err == sql.ErrNoRows {
		return echo.ErrForbidden
	} else if err != nil {
		return err
	}

	digest := fmt.Sprintf("%x", sha1.Sum([]byte(user.Salt+pw)))
	if digest != user.Password {
		return echo.ErrForbidden
	}
	sessSetUserID(c, user.ID)
	return c.Redirect(http.StatusSeeOther, "/")
}

func getLogout(c echo.Context) error {
	sess, _ := session.Get("session", c)
	delete(sess.Values, "user_id")
	sess.Save(c.Request(), c.Response())
	return c.Redirect(http.StatusSeeOther, "/")
}

func postMessage(c echo.Context) error {
	user, err := ensureLogin(c)
	if user == nil {
		return err
	}

	message := c.FormValue("message")
	if message == "" {
		return echo.ErrForbidden
	}

	var chanID int64
	if x, err := strconv.Atoi(c.FormValue("channel_id")); err != nil {
		return echo.ErrForbidden
	} else {
		chanID = int64(x)
	}

	if _, err := addMessage(chanID, user.ID, message); err != nil {
		return err
	}

	conn := pool.Get()
	defer conn.Close()
	_, err = redis.Int(conn.Do("GET", "channel_message_count:"+strconv.FormatInt(chanID, 10)))
	if err == nil {
		_, err := conn.Do("INCR", "channel_message_count:"+strconv.FormatInt(chanID, 10))
		if err != nil {
			return err
		}
	} else {
		var cnt int64
		err = db.Get(&cnt, "SELECT COUNT(*) as cnt FROM message WHERE channel_id = ?", chanID)
		if err == nil {
			conn.Do("SET", "channel_message_count:"+strconv.FormatInt(chanID, 10), cnt)
		} else {
			return err
		}
	}

	return c.NoContent(204)
}

func jsonifyMessage(m Message) (map[string]interface{}, error) {
	u := User{}
	err := db.Get(&u, "SELECT name, display_name, avatar_icon FROM user WHERE id = ?",
		m.UserID)
	if err != nil {
		return nil, err
	}

	r := make(map[string]interface{})
	r["id"] = m.ID
	r["user"] = u
	r["date"] = m.CreatedAt.Format("2006/01/02 15:04:05")
	r["content"] = m.Content
	return r, nil
}

func jsonifyMessageWithUser(m Message, userMap map[int64]User) (map[string]interface{}, error) {
	u, ok := userMap[m.UserID]
	if !ok {
		err := db.Get(&u, "SELECT name, display_name, avatar_icon FROM user WHERE id = ?",
			m.UserID)
		if err != nil {
			return nil, err
		}
	}

	r := make(map[string]interface{})
	r["id"] = m.ID
	r["user"] = u
	r["date"] = m.CreatedAt.Format("2006/01/02 15:04:05")
	r["content"] = m.Content
	return r, nil
}

func getMessage(c echo.Context) error {
	userID := sessUserID(c)
	if userID == 0 {
		return c.NoContent(http.StatusForbidden)
	}

	chanID, err := strconv.ParseInt(c.QueryParam("channel_id"), 10, 64)
	if err != nil {
		return err
	}
	lastID, err := strconv.ParseInt(c.QueryParam("last_message_id"), 10, 64)
	if err != nil {
		return err
	}

	messages, err := queryMessages(chanID, lastID)
	if err != nil {
		return err
	}

	userMap := make(map[int64]User, 0)
	if len(messages) > 0 {
		userIDs := make([]string, 0, len(messages))
		for _, m := range messages {
			userIDs = append(userIDs, strconv.FormatInt(m.UserID, 10))
		}
		rows, err := db.Queryx(fmt.Sprintf("SELECT id, name, display_name, avatar_icon FROM user WHERE id IN %s", "("+strings.Join(userIDs, ",")) + ")")
		if err != nil {
			return err
		}
		for rows.Next() {
			var user User
			err := rows.StructScan(&user)
			if err != nil {
				return err
			}
			userMap[user.ID] = user
		}
	}

	response := make([]map[string]interface{}, 0)
	for i := len(messages) - 1; i >= 0; i-- {
		m := messages[i]
		r, err := jsonifyMessageWithUser(m, userMap)
		if err != nil {
			return err
		}
		response = append(response, r)
	}

	if len(messages) > 0 {
		_, err := db.Exec("INSERT INTO haveread (user_id, channel_id, message_id, updated_at, created_at)"+
			" VALUES (?, ?, ?, NOW(), NOW())"+
			" ON DUPLICATE KEY UPDATE message_id = ?, updated_at = NOW()",
			userID, chanID, messages[0].ID, messages[0].ID)
		if err != nil {
			return err
		}
	}

	return c.JSON(http.StatusOK, response)
}

func queryChannels() ([]int64, error) {
	res := []int64{}
	err := db.Select(&res, "SELECT id FROM channel")
	return res, err
}

type HaveReadMessage struct {
	ChannelID int64 `db:"channel_id"`
	MessageID int64 `db:"message_id"`
}

type ChannelMessageCount struct {
	ChannelID int64 `db:channel_id`
	Count     int64 `db:message_count`
}

func fetchUnread(c echo.Context) error {
	userID := sessUserID(c)
	if userID == 0 {
		return c.NoContent(http.StatusForbidden)
	}

	time.Sleep(time.Second)

	channels, err := queryChannels()
	if err != nil {
		return err
	}

	conn := pool.Get()
	defer conn.Close()
	if err != nil {
		return err
	}

	rows, err := db.Queryx("SELECT channel_id, message_id FROM haveread WHERE user_id = ?", userID)
	var h HaveReadMessage
	if err != nil {
		return err
	}
	channelOf := make(map[int64]HaveReadMessage)
	for rows.Next() {
		err := rows.StructScan(&h)
		if err != nil {
			return err
		}
		channelOf[h.ChannelID] = h
	}

	resp := []map[string]interface{}{}

	for _, chID := range channels {
		message, ok := channelOf[chID]
		lastID := message.MessageID

		var cnt int64
		if ok {
			err = db.Get(&cnt,
				"SELECT COUNT(*) as cnt FROM message WHERE channel_id = ? AND ? < id",
				chID, lastID)
		} else {
			count, err := redis.Int(conn.Do("GET", "channel_message_count:"+strconv.FormatInt(chID, 10)))
			if err == nil {
				cnt = int64(count)
			} else {
				err = db.Get(&cnt,
					"SELECT COUNT(*) as cnt FROM message WHERE channel_id = ?",
					chID)
				if err == nil {
					conn.Do("SET", "channel_message_count:"+strconv.FormatInt(chID, 10), cnt)
				}
			}
		}
		if err != nil {
			return err
		}
		r := map[string]interface{}{
			"channel_id": chID,
			"unread":     cnt}
		resp = append(resp, r)
	}

	return c.JSON(http.StatusOK, resp)
}

func getHistory(c echo.Context) error {
	chID, err := strconv.ParseInt(c.Param("channel_id"), 10, 64)
	if err != nil || chID <= 0 {
		return ErrBadReqeust
	}

	user, err := ensureLogin(c)
	if user == nil {
		return err
	}

	var page int64
	pageStr := c.QueryParam("page")
	if pageStr == "" {
		page = 1
	} else {
		page, err = strconv.ParseInt(pageStr, 10, 64)
		if err != nil || page < 1 {
			return ErrBadReqeust
		}
	}

	const N = 20
	var cnt int64
	err = db.Get(&cnt, "SELECT COUNT(*) as cnt FROM message WHERE channel_id = ?", chID)
	if err != nil {
		return err
	}
	maxPage := int64(cnt+N-1) / N
	if maxPage == 0 {
		maxPage = 1
	}
	if page > maxPage {
		return ErrBadReqeust
	}

	messages := []Message{}
	err = db.Select(&messages,
		"SELECT * FROM message WHERE channel_id = ? ORDER BY id DESC LIMIT ? OFFSET ?",
		chID, N, (page-1)*N)
	if err != nil {
		return err
	}

	mjson := make([]map[string]interface{}, 0)
	for i := len(messages) - 1; i >= 0; i-- {
		r, err := jsonifyMessage(messages[i])
		if err != nil {
			return err
		}
		mjson = append(mjson, r)
	}

	channels := []ChannelInfo{}
	err = db.Select(&channels, "SELECT id, name FROM channel ORDER BY id")
	if err != nil {
		return err
	}

	return c.Render(http.StatusOK, "history", map[string]interface{}{
		"ChannelID": chID,
		"Channels":  channels,
		"Messages":  mjson,
		"MaxPage":   maxPage,
		"Page":      page,
		"User":      user,
	})
}

func getProfile(c echo.Context) error {
	self, err := ensureLogin(c)
	if self == nil {
		return err
	}

	channels := []ChannelInfo{}
	err = db.Select(&channels, "SELECT id, name FROM channel ORDER BY id")
	if err != nil {
		return err
	}

	userName := c.Param("user_name")
	var other User
	err = db.Get(&other, "SELECT * FROM user WHERE name = ?", userName)
	if err == sql.ErrNoRows {
		return echo.ErrNotFound
	}
	if err != nil {
		return err
	}

	return c.Render(http.StatusOK, "profile", map[string]interface{}{
		"ChannelID":   0,
		"Channels":    channels,
		"User":        self,
		"Other":       other,
		"SelfProfile": self.ID == other.ID,
	})
}

func getAddChannel(c echo.Context) error {
	self, err := ensureLogin(c)
	if self == nil {
		return err
	}

	channels := []ChannelInfo{}
	err = db.Select(&channels, "SELECT id, name FROM channel ORDER BY id")
	if err != nil {
		return err
	}

	return c.Render(http.StatusOK, "add_channel", map[string]interface{}{
		"ChannelID": 0,
		"Channels":  channels,
		"User":      self,
	})
}

func postAddChannel(c echo.Context) error {
	self, err := ensureLogin(c)
	if self == nil {
		return err
	}

	name := c.FormValue("name")
	desc := c.FormValue("description")
	if name == "" || desc == "" {
		return ErrBadReqeust
	}

	res, err := db.Exec(
		"INSERT INTO channel (name, description, updated_at, created_at) VALUES (?, ?, NOW(), NOW())",
		name, desc)
	if err != nil {
		return err
	}
	lastID, _ := res.LastInsertId()
	return c.Redirect(http.StatusSeeOther,
		fmt.Sprintf("/channel/%v", lastID))
}

func postProfile(c echo.Context) error {
	self, err := ensureLogin(c)
	if self == nil {
		return err
	}

	avatarName := ""
	var avatarData []byte

	if fh, err := c.FormFile("avatar_icon"); err == http.ErrMissingFile {
		// no file upload
	} else if err != nil {
		return err
	} else {
		dotPos := strings.LastIndexByte(fh.Filename, '.')
		if dotPos < 0 {
			return ErrBadReqeust
		}
		ext := fh.Filename[dotPos:]
		switch ext {
		case ".jpg", ".jpeg", ".png", ".gif":
			break
		default:
			return ErrBadReqeust
		}

		file, err := fh.Open()
		if err != nil {
			return err
		}
		avatarData, _ = ioutil.ReadAll(file)
		file.Close()

		if len(avatarData) > avatarMaxBytes {
			return ErrBadReqeust
		}

		avatarName = fmt.Sprintf("%x%s", sha1.Sum(avatarData), ext)
	}

	if avatarName != "" && len(avatarData) > 0 {
		var davURL string

		if avatarName[0] >= '0' && avatarName[0] <= '7' {
			davURL = davHost1 + "icons/" + avatarName
		} else {
			davURL = davHost2 + "icons/" + avatarName
		}

		req, err := http.NewRequest("PUT", davURL, bytes.NewBuffer(avatarData))
		if err != nil {
			return err
		}

		_, err = http.DefaultClient.Do(req)
		if err != nil {
			return err
		}

		_, err = db.Exec("UPDATE user SET avatar_icon = ? WHERE id = ?", avatarName, self.ID)
		if err != nil {
			return err
		}
	}

	if name := c.FormValue("display_name"); name != "" {
		_, err := db.Exec("UPDATE user SET display_name = ? WHERE id = ?", name, self.ID)
		if err != nil {
			return err
		}
	}

	return c.Redirect(http.StatusSeeOther, "/")
}

func getIcon(c echo.Context) error {
	var name string
	var data []byte
	err := db.QueryRow("SELECT name, data FROM image WHERE name = ?",
		c.Param("file_name")).Scan(&name, &data)
	if err == sql.ErrNoRows {
		return echo.ErrNotFound
	}
	if err != nil {
		return err
	}

	mime := ""
	switch true {
	case strings.HasSuffix(name, ".jpg"), strings.HasSuffix(name, ".jpeg"):
		mime = "image/jpeg"
	case strings.HasSuffix(name, ".png"):
		mime = "image/png"
	case strings.HasSuffix(name, ".gif"):
		mime = "image/gif"
	default:
		return echo.ErrNotFound
	}
	return c.Blob(http.StatusOK, mime, data)
}

func dumpIcon(c echo.Context) error {
	for i := 1; i <= 1001; i += 10 {
		tmps := make([]struct {
			Name string `db:"name"`
			Data []byte `db:"data"`
		}, 0, 10)

		err := db.Select(&tmps, "SELECT name, data FROM image WHERE id >= ? AND id < ?", i, i+10)
		if err != nil {
			return err
		}
		// for _, t := range tmps {
		// err = ioutil.WriteFile(imageDir+t.Name, t.Data, 0666)
		// if err != nil {
		// 	return err
		// }
		// }
	}
	return nil
}

func tAdd(a, b int64) int64 {
	return a + b
}

func tRange(a, b int64) []int64 {
	r := make([]int64, b-a+1)
	for i := int64(0); i <= (b - a); i++ {
		r[i] = a + i
	}
	return r
}

func main() {
	e := echo.New()
	funcs := template.FuncMap{
		"add":    tAdd,
		"xrange": tRange,
	}
	e.Renderer = &Renderer{
		templates: template.Must(template.New("").Funcs(funcs).ParseGlob("views/*.html")),
	}
	e.Use(session.Middleware(sessions.NewCookieStore([]byte("secretonymoris"))))
	// e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
	// 	Format: "request:\"${method} ${uri}\" status:${status} latency:${latency} (${latency_human}) bytes:${bytes_out}\n",
	// }))
	e.Use(middleware.Static("../public"))

	e.GET("/initialize", getInitialize)
	e.GET("/", getIndex)
	e.GET("/register", getRegister)
	e.POST("/register", postRegister)
	e.GET("/login", getLogin)
	e.POST("/login", postLogin)
	e.GET("/logout", getLogout)

	e.GET("/channel/:channel_id", getChannel)
	e.GET("/message", getMessage)
	e.POST("/message", postMessage)
	e.GET("/fetch", fetchUnread)
	e.GET("/history/:channel_id", getHistory)

	e.GET("/profile/:user_name", getProfile)
	e.POST("/profile", postProfile)

	e.GET("add_channel", getAddChannel)
	e.POST("add_channel", postAddChannel)
	e.GET("/icons/:file_name", getIcon)
	e.GET("/dump", dumpIcon)

	e.Start(":5000")
}
