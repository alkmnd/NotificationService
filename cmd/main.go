package main

import (
	"NotificationService"
	"net/http"
	"os"
	"sync"
)

func main() {

	serviceApiKey := os.Getenv("SERVICE_API_KEY")

	wsServer := NotificationService.NewWebsocketServer(serviceApiKey)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		wsServer.Run()
	}()

	go func() {
		defer wg.Done()
		http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
			NotificationService.ServeWs(wsServer, w, r)
		})
		http.ListenAndServe(":8081", nil)
	}()

	wg.Wait()

}
