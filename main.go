package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

type ChatMessage struct {
	User string `json:"user"`
	Text string `json:"text"`
	Time int64  `json:"time"`
}

var (
	ctx       = context.Background()
	rdb       *redis.Client
	upgrader  = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	clients   = make(map[*websocket.Conn]bool)
	userNames = make(map[*websocket.Conn]string)
)

func initRedis() {
	rdb = redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if err := rdb.Ping(ctx).Err(); err != nil {
		panic(err)
	}
	fmt.Println("✅ Connected to Redis")
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrader error:", err)
		return
	}

	fmt.Println("💬 New WebSocket connection")
	clients[conn] = true

	defer func() {
		name := userNames[conn]
		if name != "" {
			rdb.SRem(ctx, "chat:members", name)
			rdb.Publish(ctx, "member_remove", name)
		}
		delete(userNames, conn)
		delete(clients, conn)
		conn.Close()
	}()

	members, _ := rdb.SMembers(ctx, "chat:members").Result()
	rawHistory, _ := rdb.ZRange(ctx, "chat:messages", -20, -1).Result()

	var history []ChatMessage
	for _, h := range rawHistory {
		var msg ChatMessage
		json.Unmarshal([]byte(h), &msg)
		history = append(history, msg)
	}

	conn.WriteJSON(map[string]interface{}{
		"type":    "init",
		"members": members,
		"history": history,
	})

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("❌ Read error:", err)
			break
		}

		text := string(msg)

		if strings.HasPrefix(text, "join:") {
			name := strings.TrimSpace(text[5:])
			if name == "" {
				continue
			}
			userNames[conn] = name
			rdb.SAdd(ctx, "chat:members", name)
			rdb.Publish(ctx, "member_add", name)
			conn.WriteMessage(websocket.TextMessage, []byte("Welcome "+name+"!"))

			go subscribeToDM(name, conn)
			continue
		}

		// Direct message format: dm:sender:receiver:message
		if strings.HasPrefix(text, "dm:") {
			parts := strings.SplitN(text[3:], ":", 3)
			if len(parts) < 3 {
				continue
			}
			sender := parts[0]
			receiver := parts[1]
			message := parts[2]

			msgObj := ChatMessage{
				User: sender,
				Text: message,
				Time: time.Now().Unix(),
			}
			jsonMsg, _ := json.Marshal(msgObj)

			key := fmt.Sprintf("chat:dm:%s:%s", sender, receiver)
			rdb.ZAdd(ctx, key, redis.Z{Score: float64(msgObj.Time), Member: jsonMsg})

			rdb.Publish(ctx, "dm:"+receiver, jsonMsg)

			conn.WriteMessage(websocket.TextMessage, jsonMsg)
			continue
		}

		// Public message format: msg:username:text
		if strings.HasPrefix(text, "msg:") {
			parts := strings.SplitN(text[4:], ":", 2)
			if len(parts) < 2 {
				continue
			}
			user := parts[0]
			message := parts[1]

			msgObj := ChatMessage{
				User: user,
				Text: message,
				Time: time.Now().Unix(),
			}

			jsonMsg, _ := json.Marshal(msgObj)
			rdb.ZAdd(ctx, "chat:messages", redis.Z{Score: float64(msgObj.Time), Member: jsonMsg})
			rdb.Publish(ctx, "messages", jsonMsg)
		}
	}
}

func listenPublicMessages() {
	pubsub := rdb.Subscribe(ctx, "messages")
	ch := pubsub.Channel()
	for msg := range ch {
		for c := range clients {
			c.WriteMessage(websocket.TextMessage, []byte(msg.Payload))
		}
	}
}

func listenMemberAdd() {
	pubsub := rdb.Subscribe(ctx, "member_add")
	ch := pubsub.Channel()
	for msg := range ch {
		for c := range clients {
			c.WriteJSON(map[string]string{
				"type": "member_add",
				"name": msg.Payload,
			})
		}
	}
}

func listenMemberRemove() {
	pubsub := rdb.Subscribe(ctx, "member_remove")
	ch := pubsub.Channel()
	for msg := range ch {
		for c := range clients {
			c.WriteJSON(map[string]string{
				"type": "member_remove",
				"name": msg.Payload,
			})
		}
	}
}

func subscribeToDM(username string, conn *websocket.Conn) {
	pubsub := rdb.Subscribe(ctx, "dm:"+username)
	ch := pubsub.Channel()
	for msg := range ch {
		conn.WriteMessage(websocket.TextMessage, []byte(msg.Payload))
	}
}

func main() {
	initRedis()
	go listenPublicMessages()
	go listenMemberAdd()
	go listenMemberRemove()

	http.HandleFunc("/ws", handleWebSocket)
	fmt.Println("🚀 Server running at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
