package main

import (
	"bytes"
	"crypto/sha512"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"github.com/jinzhu/configor"

	"github.com/bwmarrin/discordgo"
	"github.com/gidoBOSSftw5731/log"
	_ "github.com/lib/pq"
)

var (
	Config struct {
		DB struct {
			User     string `default:"discordmessagelog"`
			Password string `required:"true" env:"DBPassword" default:"disGOURD"`
			Port     string `default:"5432"`
			IP       string `default:"127.0.0.1"`
		}
		Token     string `required:"true"`
		FileStore string `required:"true"`
	}
	db         *sql.DB
	insertStmt *sql.Stmt
)

func main() {
	log.SetCallDepth(4)
	err := configor.Load(&Config, "config.yml")
	fmt.Println(Config)
	errCheck("Config Error", err)

	go createFileDir()
	go MkDB()

	println(Config.Token)

	discord, err := discordgo.New(Config.Token)
	errCheck("error creating discord session", err)
	//user, err := discord.User("@me")
	errCheck("error retrieving account", err)

	discord.AddHandler(newMessage)

	err = discord.Open()
	errCheck("Error opening connection to Discord", err)
	defer discord.Close()

	<-make(chan struct{})

}

func errCheck(msg string, err error) {
	if err != nil {
		log.Fatalf("%s: %+v", msg, err)
	}

}

func newMessage(discord *discordgo.Session, message *discordgo.MessageCreate) {
	var hashlist []string
	h := sha512.New()
	for _, i := range message.Attachments {
		h.Reset()
		resp, err := http.Get(i.ProxyURL)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)

		_, err = io.Copy(h, buf)
		if err != nil {
			log.Errorf("Error hashing file: %v", err)
			continue
		}

		hash := string(h.Sum(nil)) + i.Filename

		fpath := filepath.Join(Config.FileStore, string(hash[0]), string(hash[1]),
			string(hash[2]), string(hash[3]), hash)

		ioutil.WriteFile(fpath, buf.Bytes(), 0)

		hashlist = append(hashlist, hash)
	}

	embeds, err := json.Marshal(message.Embeds)
	if err != nil {
		log.Errorf("JSON error: %v", err)
	}

	insertStmt.Exec(message.ID, message.ChannelID, message.GuildID, message.Content,
		message.Timestamp, message.Tts, message.MentionEveryone, message.Author.ID, message.Author.Username,
		message.Author.Discriminator, message.Author.Bot, message.Author.Verified, message.Author.MFAEnabled,
		hashlist, string(embeds), message.Type)
}

// createImgDir creates all image storage directories
func createFileDir() {
	for f := 0; f < 16; f++ {
		for s := 0; s < 16; s++ {
			for t := 0; s < 16; t++ {
				for iv := 0; s < 16; iv++ {
					os.MkdirAll(path.Join(Config.FileStore, fmt.Sprintf("%x/%x/%x/%x", f, s, t, iv)), 0755)
				}
			}
		}
	}
	fmt.Println("Finished making/Verifying the directories!")
}

//MkDB is a function that takes a config struct and returns a pointer to a database.
func MkDB() {
	var err error
	db, err = sql.Open("postgres", fmt.Sprintf("user=%v password=%v dbname=discordmessagelog host=%v port=%v",
		Config.DB.User, Config.DB.Password, Config.DB.IP, Config.DB.Port))
	errCheck("DB issue", err)

	insertStmt, err = db.Prepare(
		"INSERT INTO messages VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)")
	errCheck("Stmt issue", err)
	/*
		create database discordmessagelog;
		create user discordmessagelog with encrypted password 'disGOURD';
		create table messages (
			id text,
			channelid text,
			serverid text,
			content text,
			timestamp time,
			tts bool,
			everyone bool,
			authorid text,
			authorname text,
			authordiscrim text,
			isbot bool,
			emailverified bool,
			mfa bool,
			filehashes text[],
			embedJSON text,
			messagetype int
		);
		GRANT ALL ON ALL TABLES IN SCHEMA public TO discordmessagelog;
	*/
}
