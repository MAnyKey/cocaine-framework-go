package cocaine

import (
	"fmt"
	"log"
	"time"

	"github.com/ugorji/go/codec"
)

const DEFAULT_LOCATOR_PORT = 10053

type Endpoint struct {
	Host string
	Port int
}

func (endpoint *Endpoint) AsString() string {
	return fmt.Sprintf("%s:%d", endpoint.Host, endpoint.Port)
}

type ResolveResult struct {
	success  bool
	Endpoint `codec:",omitempty"`
	Version  int
	API      map[int]string
}

type Locator struct {
	unpacker *StreamUnpacker
	SocketIO
}

func NewLocator(args ...interface{}) (*Locator, error) {
	var endpoint string = "localhost:10053"

	if len(args) == 1 {
		if _endpoint, ok := args[0].(string); ok {
			endpoint = _endpoint
		}
	}

	sock, err := NewASocket("tcp", endpoint, time.Second*5)
	if err != nil {
		return nil, err
	}
	return &Locator{NewStreamUnpacker(), sock}, nil
}

func (locator *Locator) unpackchunk(chunk RawMessage) ResolveResult {
	defer func() {
		if err := recover(); err != nil {
			log.Println("defer", err)
		}
	}()
	var res ResolveResult
	err := codec.NewDecoderBytes(chunk, h).Decode(&res)
	if err != nil {
		log.Println("unpack chunk error", err)
	}
	return res
}

func (locator *Locator) Resolve(name string) chan ResolveResult {
	Out := make(chan ResolveResult)
	go func() {
		var resolveresult ResolveResult
		resolveresult.success = false
		msg := ServiceMethod{MessageInfo{0, 0}, []interface{}{name}}
		raw := Pack(&msg)
		locator.SocketIO.Write() <- raw
		closed := false
		for !closed {
			answer := <-locator.SocketIO.Read()
			msgs := locator.unpacker.Feed(answer)
			for _, item := range msgs {
				switch id := item.GetTypeID(); id {
				case CHUNK:
					resolveresult = locator.unpackchunk(item.GetPayload()[0].([]byte))
					resolveresult.success = true
				case CHOKE:
					closed = true
				}
			}
		}
		Out <- resolveresult
	}()
	return Out
}

func (locator *Locator) Close() {
	locator.SocketIO.Close()
}
