package main

import (
	websocket "code.google.com/p/go.net/websocket"
	"flag"
	"fmt"
	tail "github.com/ActiveState/tail"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"html"
)

func loadIndex(w http.ResponseWriter, r *http.Request) {
	file, err := os.Open("index.html")

	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	defer file.Close()

	io.Copy(w, file)
}

func tailHandler(filepath string) func(*websocket.Conn) {
	return func(ws *websocket.Conn) {
		log.Println("Connected from:", ws.Request().RemoteAddr)

		defer func() {
			log.Println("Close connection:", ws.Request().RemoteAddr)
			ws.Close()
		}()

		file, err := tail.TailFile(filepath, tail.Config{Follow: true, ReOpen: true})

		if err != nil {
			log.Println(err)
			return
		}

		tailf := func() {
			for line := range file.Lines {
				escapedText := html.EscapeString(line.Text)
				ws.Write([]byte(escapedText))
			}
		}

		go tailf()

		for {
			if err := websocket.Message.Receive(ws, nil); err != nil {
				file.Stop()
				break
			}
		}
	}
}

func main() {
	host := flag.String("-b", "0.0.0.0", "bind hostname")
	port := flag.String("-p", "8888", "bind port number")
	flag.Parse()

	if len(flag.Args()) != 1 {
		fmt.Println("Usage: tails [-b bind address] [-p bind port] filepath")
		return
	}

	logFile, err := os.OpenFile("log.txt", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)

	if err != nil {
		fmt.Println("Can't open tails.log.")
		return
	}

	defer func() { logFile.Close() }()

	log.SetOutput(logFile)

	filepath := flag.Args()[0]

	http.HandleFunc("/", loadIndex)
	http.Handle("/tail", websocket.Handler(tailHandler(filepath)))
	fmt.Printf("Start server on %v:%v and watching %v\n", *host, *port, filepath)

	go func() {
		if err := http.ListenAndServe(*host+":"+*port, nil); err != nil {
			fmt.Println("Could not start server. %v", err)
		}
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP, syscall.SIGKILL)
	for sig := range ch {
		log.Printf("Received signal: %v", sig)
		if sig == syscall.SIGINT {
			log.Printf("shutdown")
			return
		}
	}
}
