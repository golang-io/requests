module main

go 1.22.1

require (
	github.com/go-chi/chi/v5 v5.0.12
	github.com/golang-io/requests v0.0.0-00010101000000-000000000000
	github.com/gorilla/websocket v1.5.1
)

require golang.org/x/net v0.23.0 // indirect

replace github.com/golang-io/requests => ../../../
