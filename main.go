package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

	_ "github.com/go-sql-driver/mysql"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"google.golang.org/grpc"
	"tlgsimple/message"
)

type UpdateData struct {
	UserID    int64
	Nickname  string
	Message   string
	MessageTs time.Time
}

type GrpcClient struct {
	client message.MessageServiceClient
	conn   *grpc.ClientConn
}

// Configs  TODO external config file
const (
	DB_HOST     = "localhost"
	DB_USERNAME = "root"
	DB_PASSWORD = "root"
	DB_NAME     = "tlgsimple"
	DB_PORT     = "33061"
	BOT_TOKEN 	= "5952958681:AAHou9wioP-iFbkjandvHgllKB3-PR7mSQM"
	BOT_URL     = "https://c5ba-141-136-89-124.eu.ngrok.io/"
	BOT_DEBUG   = true
)

func main() {

	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", DB_USERNAME, DB_PASSWORD, DB_HOST, DB_PORT, DB_NAME))
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	bot, err := tgbotapi.NewBotAPI(BOT_TOKEN)
	if err != nil {
		log.Fatalf("Failed to create Telegram bot: %v", err)
	}
	bot.Debug = BOT_DEBUG

	log.Printf("Authorized on account %s", bot.Self.UserName)

	grpcClient, err := newGrpcClient("localhost:9000")
	if err != nil {
		log.Fatalf("Failed to create gRPC client: %v", err)
	}
	defer grpcClient.conn.Close()

	wh, _ := tgbotapi.NewWebhook(BOT_URL+"webhook")

	_, err = bot.Request(wh)
	if err != nil {
		log.Fatal(err)
	}

	info, err := bot.GetWebhookInfo()
	if err != nil {
		log.Fatal(err)
	}

	if info.LastErrorDate != 0 {
		log.Printf("Telegram callback failed: %s", info.LastErrorMessage)
	}

	http.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		update, err := bot.HandleUpdate(r)
		if err != nil {
			log.Println(err)
			return
		}

		if update.Message == nil {
			return
		}

		userID := update.Message.From.ID
		nickname := update.Message.From.UserName
		messageContent := update.Message.Text
		messageTime := update.Message.Time()

		updateData := UpdateData{userID, nickname, messageContent, messageTime}

		if messageContent == "/grpc" {
			grpcResponse := GrpcFunction(grpcClient.client, updateData)
			log.Println("gRPC response:", grpcResponse)
			err = storeUpdate(db, updateData)
		} else {
			err = storeUpdate(db, updateData)
			if err != nil {
				log.Printf("Failed to store update in database: %v", err)
			}
		}
	})

	http.ListenAndServe(":8080", nil)
}

func storeUpdate(db *sql.DB, data UpdateData) error {
	query := "INSERT INTO messages (user_id, nickname, message_content, message_time) VALUES (?, ?, ?, ?)"
	_, err := db.Exec(query, data.UserID, data.Nickname, data.Message, data.MessageTs)
	return err
}

func newGrpcClient(target string) (*GrpcClient, error) {
	conn, err := grpc.Dial(target, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	client := message.NewMessageServiceClient(conn)
	return &GrpcClient{client: client, conn: conn}, nil
}

func GrpcFunction(client message.MessageServiceClient, data UpdateData) string {
	messageTs := data.MessageTs.Unix()

	dataReq := &message.DataRequest{
		UserId:    int64(data.UserID),
		Nickname:  data.Nickname,
		Message:   data.Message,
		MessageTs: messageTs,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.SendData(ctx, dataReq)
	if err != nil {
		log.Printf("Failed to send data: %v", err)
		return "Error"
	}

	return resp.GetStatus()
}
