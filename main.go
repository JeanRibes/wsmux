package main

import (
	"container/list"
	"flag"
	"fmt"
	"github.com/gorilla/websocket"
	"io"
	"net/http"
	"os"
	"sync"
)

var httpToWs = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	}, Subprotocols: []string{"broadway"},
}

//var clients = []*websocket.Conn{}
var clients = list.New()
var broadwayServer *websocket.Conn
var broadwayLock sync.Mutex

var broadway_addr = flag.String("broadway-addr", "127.0.0.1:8080", "address of the Broadway display server, in the form of ip:port")
var listen_addr = flag.String("listen-addr", ":8090", "address to listen on, ip:port")

func he(err error) {
	if err != nil {
		panic(err)
	}
}

func connectBroadway() *websocket.Conn {

	dialer := websocket.DefaultDialer
	dialer.Subprotocols = []string{"broadway"}
	header := http.Header{}
	header.Set("Origin", "http://"+*broadway_addr)
	conn, r, err := dialer.Dial("ws://"+*broadway_addr+"/socket", header)
	he(err)
	if r != nil {
		fmt.Printf("status %s\n", r.Status)
		if r.StatusCode == 400 {
			r.Write(os.Stdout)
			os.Exit(1)
		}
	}
	if conn == nil {
		panic(err)
	}
	return conn
}

func handleWs(w http.ResponseWriter, r *http.Request) {
	conn, err := httpToWs.Upgrade(w, r, nil)
	he(err)
	var el *list.Element
	if conn != nil {
		//clients = append(clients, conn)
		el = clients.PushBack(conn)
	}
	fmt.Printf("client %s connected\n", conn.RemoteAddr().String())
	for {
		msg, msgtype, err1 := conn.ReadMessage()
		if err1 != nil {
			break
		}
		broadwayLock.Lock()
		if broadwayServer.WriteMessage(msg, msgtype) != nil {
			broadwayLock.Unlock()
			break
		}
		broadwayLock.Unlock()
	}
	fmt.Printf("client %s: disconnected\n", conn.RemoteAddr().String())
	clients.Remove(el)
}
func broadwayDispatchLoop() {
	for {
		msg, msgtype, err2 := broadwayServer.ReadMessage()
		he(err2)
		for e := clients.Front(); e != nil; e = e.Next() {
			client := e.Value.(*websocket.Conn)
			if client != nil {
				err3 := client.WriteMessage(msg, msgtype)
				if err3 != nil {
					fmt.Println(err3)
					clients.Remove(e)
				}
			}
		}
	}
}
func main() {
	flag.Parse()

	broadwayServer = connectBroadway()
	go broadwayDispatchLoop()

	http.HandleFunc("/socket", handleWs)
	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		res, err := http.Get("http://" + *broadway_addr + "/")
		he(err)
		writer.Header().Set("Content-Type", res.Header.Get("Content-Type"))
		_, werr := io.Copy(writer, res.Body)
		he(werr)
	})
	http.HandleFunc("/broadway.js", func(writer http.ResponseWriter, request *http.Request) {
		res, err := http.Get("http://" + *broadway_addr + "/broadway.js")
		he(err)
		writer.Header().Set("Content-Type", res.Header.Get("Content-Type"))
		_, werr := io.Copy(writer, res.Body)
		he(werr)
	})
	http.HandleFunc("/count", func(writer http.ResponseWriter, request *http.Request) {
		fmt.Fprintf(writer, "%d clients connected", clients.Len())
	})
	he(http.ListenAndServe(*listen_addr, nil))
}
