package main

import (
	"github.com/gorilla/websocket"
	"log"
	"os"
	"os/signal"
	"time"
)

func main() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	conn, _, err := websocket.DefaultDialer.Dial("ws://localhost:1234/ws", nil)
	if err != nil {
		log.Fatalf("Failed to connect to WebSocket server: %v", err)
	}
	defer conn.Close()

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Printf("Failed to read message: %v", err)
				return
			}
			log.Printf("Received message: %s", message)
		}
	}()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			err := conn.WriteMessage(websocket.TextMessage, []byte("Hello, WebSocket server!"))
			if err != nil {
				log.Printf("Failed to write message: %v", err)
				return
			}
		case <-interrupt:
			log.Println("Interrupted, closing connection...")

			err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Printf("Failed to write close message: %v", err)
				return
			}

			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return
		}
	}
}
