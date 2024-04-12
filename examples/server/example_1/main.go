package main

import (
	"context"
	"fmt"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-io/requests"
	"github.com/gorilla/websocket"
	"io"
	"log"
	"net/http"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func ws(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer conn.Close()
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Failed to read message: %v", err)
			break
		}

		log.Printf("Received message: %s", message)

		err = conn.WriteMessage(messageType, message)
		if err != nil {
			log.Printf("Failed to write message: %v", err)
			break
		}
	}
}

func main() {
	r := requests.NewServeMux(
		requests.URL("0.0.0.0:1234"),
		requests.Use(middleware.Recoverer, middleware.Logger),
		requests.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				next.ServeHTTP(w, r)
			})
		}),
	)
	r.Route("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("panic test")
	})
	r.Route("/echo", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(w, r.Body)
	})
	r.Route("/ping", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "pong\n")
	}, requests.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}))
	r.Route("/ws", ws)
	err := requests.ListenAndServe(context.Background(), r)
	fmt.Println(err)
}
