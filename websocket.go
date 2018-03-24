package bitstamp

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
)

var _socketurl string = "wss://ws.pusherapp.com/app/de504dc5763aeef9ff52?protocol=7&client=js&version=2.1.6&flash=false"

type WebSocket struct {
	ws     *websocket.Conn
	quit   chan bool
	Stream chan *Event
	Errors chan error
}

type EventOrderBookData struct {
	Time time.Time
	Bids []OrderBookItem
	Asks []OrderBookItem
}

type Event struct {
	Time  time.Time
	Event string      `json:"event"`
	Data  interface{} `json:"data"`
}

func (e *Event) AsOrderBookData() (*EventOrderBookData, error) {
	res := &EventOrderBookData{}
	res.Time = e.Time
	res.Bids = make([]OrderBookItem, 0)
	res.Asks = make([]OrderBookItem, 0)

	tmp := &struct {
		Bids [][]string `json:"bids"`
		Asks [][]string `json:"asks"`
	}{}
	err := json.Unmarshal([]byte(e.Data.(string)), tmp)
	if err != nil {
		return nil, err
	}
	for _, val := range tmp.Bids {
		p, err := strconv.ParseFloat(val[0], 64)
		if err != nil {
			return nil, err
		}
		a, err := strconv.ParseFloat(val[1], 64)
		if err != nil {
			return nil, err
		}

		res.Bids = append(res.Bids, OrderBookItem{Price: p, Amount: a})
	}
	for _, val := range tmp.Asks {
		p, err := strconv.ParseFloat(val[0], 64)
		if err != nil {
			return nil, err
		}
		a, err := strconv.ParseFloat(val[1], 64)
		if err != nil {
			return nil, err
		}

		res.Asks = append(res.Asks, OrderBookItem{Price: p, Amount: a})
	}
	return res, nil
}

func (s *WebSocket) Close() {
	s.quit <- true
}

func (s *WebSocket) LiveTrades(currencyPair string) {
	s.Subscribe(fmt.Sprintf("live_trades_%s", currencyPair))
}

func (s *WebSocket) LiveOrders(currencyPair string) {
	s.Subscribe(fmt.Sprintf("live_orders_%s", currencyPair))
}

func (s *WebSocket) OrderBook(currencyPair string) {
	s.Subscribe(fmt.Sprintf("order_book_%s", currencyPair))
}

func (s *WebSocket) DiffOrderBook(currencyPair string) {
	s.Subscribe(fmt.Sprintf("diff_order_book_%s", currencyPair))
}

func (s *WebSocket) Subscribe(channel string) {
	a := &Event{
		Event: "pusher:subscribe",
		Data: map[string]interface{}{
			"channel": channel,
		},
	}
	s.ws.WriteJSON(a)
}

func (s *WebSocket) SendTextMessage(message []byte) {
	s.ws.WriteMessage(websocket.TextMessage, message)
}

func (s *WebSocket) Ping() {
	a := &Event{
		Event: "pusher:ping",
	}
	s.ws.WriteJSON(a)
}

func (s *WebSocket) Pong() {
	a := &Event{
		Event: "pusher:pong",
	}
	s.ws.WriteJSON(a)
}

func (b Bitstamp) NewWebSocket(t time.Duration) (*WebSocket, error) {
	var err error
	s := &WebSocket{
		quit:   make(chan bool, 1),
		Stream: make(chan *Event),
		Errors: make(chan error),
	}

	// set up websocket
	s.ws, _, err = websocket.DefaultDialer.Dial(_socketurl, nil)
	if err != nil {
		return nil, fmt.Errorf("error dialing websocket: %s", err)
	}

	go func() {
		defer s.ws.Close()
		for {
			runtime.Gosched()
			s.ws.SetReadDeadline(time.Now().Add(t))
			select {
			case <-s.quit:
				return
			default:
				var message []byte
				var err error
				_, message, err = s.ws.ReadMessage()
				if err != nil {
					s.Errors <- err
					continue
				}
				e := &Event{}
				err = json.Unmarshal(message, e)
				if err != nil {
					s.Errors <- err
					continue
				}
				e.Time = time.Now()
				s.Stream <- e
			}
		}
	}()

	return s, nil
}
