This project is a real-time chat application built with **Go**, **WebSockets**, and **Redis**. It leverages Redis Pub/Sub for message broadcasting and Redis Sorted Sets to maintain a persistent chat history.

---

## 🚀 Features

* **Real-time Messaging**: Instant communication via WebSockets.
* **Public Chat**: Messages broadcasted to all connected users.
* **Direct Messaging (DM)**: Private messages between specific users using dedicated Redis channels.
* **Presence Tracking**: Notifies the community when users join or leave.
* **Persistent History**: Stores the last 20 public messages and DM history in Redis.
* **Concurrency**: Uses Go routines to handle multiple Pub/Sub listeners simultaneously.

---

## 🛠 Prerequisites

* **Go**: 1.18 or higher
* **Redis**: Running on `localhost:6379`
* **Dependencies**:
* `github.com/gorilla/websocket`
* `github.com/redis/go-redis/v9`



---

## 📦 Installation & Setup

1. **Clone the repository**:
```bash
git clone <repository-url>
cd <repository-folder>

```


2. **Install dependencies**:
```bash
go mod tidy

```


3. **Run the server**:
```bash
go run main.go

```


The server will start at `http://localhost:8080`.

---

## 💬 Protocol Definitions

The server communicates via a simple string-prefix protocol over WebSockets:

| Action | Format | Description |
| --- | --- | --- |
| **Join** | `join:username` | Registers your name and joins the chat. |
| **Public Msg** | `msg:username:text` | Sends a message to everyone. |
| **Direct Msg** | `dm:sender:receiver:text` | Sends a private message to a specific user. |

---

## 🏗 Architecture

The system is built on a distributed-ready architecture:

1. **WebSocket Handler**: Manages individual client connections and upgrades HTTP requests.
2. **Redis Pub/Sub**: Acts as the message bus. Even if you run multiple server instances, Redis ensures all clients receive the messages.
3. **State Management**:
* `chat:members` (Set): Stores active usernames.
* `chat:messages` (Sorted Set): Stores public message history with timestamps.
* `chat:dm:sender:receiver` (Sorted Set): Stores private conversation history.

