package sse

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

//SSE name constants
const (
	eName = "event"
	dName = "data"
)

var (
	//ErrNilChan will be returned by Notify if it is passed a nil channel
	ErrNilChan = fmt.Errorf("nil channel given")
)

//Client is the default client used for requests.
var Client *http.Client = http.DefaultClient
var token string

func liveReq(method, uri string, body io.Reader) (*http.Request, error) {
	req, err := GetReq(method, uri, body)

	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "text/event-stream")
	if token != "" {
		fmt.Print("Adding token..")
		// uri = uri + "?auth=" + token
		req.Header.Set("Authorization", "Bearer "+token)
	}

	// log.Printf("Request is %#v", req.Header)
	return req, nil

}

type FBData struct {
	Path string          `json:"path"`
	Data json.RawMessage `json:"data"`
}

//Event is a go representation of an http server-sent event
type Event struct {
	URI  string
	Type string
	Data io.Reader
	RTDB FBData
}

//GetReq is a function to return a single request. It will be used by notify to
//get a request and can be replaces if additional configuration is desired on
//the request. The "Accept" header will necessarily be overwritten.
var GetReq = func(method, uri string, body io.Reader) (*http.Request, error) {
	return http.NewRequest(method, uri, body)
}

func NotifyCallBack(uri string, fn func(Event)) error {

	// log.Println("Watching .. ", uri)
	req, err := liveReq("GET", uri, nil)
	if err != nil {
		return fmt.Errorf("error getting sse request: %v", err)
	}

	res, err := Client.Do(req)
	if err != nil {
		return fmt.Errorf("error performing request for %s: %v", uri, err)
	}
	// log.Printf("\nResponse headers %#v\n", res.Header)
	br := bufio.NewReader(res.Body)
	defer res.Body.Close()

	delim := []byte{':', ' '}

	currEvent := Event{}
	for {
		bs, err := br.ReadBytes('\n')

		if err != nil && err != io.EOF {
			return err
		}

		if len(bs) < 2 {
			continue
		}

		spl := bytes.Split(bs, delim)

		if len(spl) < 2 {
			continue
		}

		currEvent.URI = uri
		switch string(spl[0]) {
		case eName:
			currEvent.Type = string(bytes.TrimSpace(spl[1]))
		case dName:
			// currEvent.Data = bytes.NewBuffer(bytes.TrimSpace(spl[1]))
			// currEvent.FBData = bytes.TrimSpace(spl[1])
			json.Unmarshal(bytes.TrimSpace(spl[1]), &currEvent.RTDB)

			// log.Printf("\nWriting to event %#v", currEvent)
			fn(currEvent)
			// evCh <- currEvent
			// log.Printf("\nDone writing  ..")
		}

		if err == io.EOF {
			break
		}
	}

	return nil
}

//Notify takes the uri of an SSE stream and channel, and will send an Event
//down the channel when recieved, until the stream is closed. It will then
//close the stream.
// This is blocking, and so you will likely want to call this
//in a new goroutine (via `go Notify(..)`)
func Notify(uri string, evCh chan<- *Event) error {

	if evCh == nil {
		return ErrNilChan
	}
	log.Println("Watching .. ", uri)
	req, err := liveReq("GET", uri, nil)
	if err != nil {
		return fmt.Errorf("error getting sse request: %v", err)
	}

	res, err := Client.Do(req)
	if err != nil {
		return fmt.Errorf("error performing request for %s: %v", uri, err)
	}
	// log.Printf("\nResponse headers %#v\n", res.Header)
	br := bufio.NewReader(res.Body)
	defer res.Body.Close()

	delim := []byte{':', ' '}

	currEvent := &Event{}
	for {
		bs, err := br.ReadBytes('\n')
		// log.Printf("\nGot bytes [%d] : %s ", len(bs), bs)
		if err != nil && err != io.EOF {
			return err
		}

		if len(bs) < 2 {
			continue
		}

		spl := bytes.Split(bs, delim)

		if len(spl) < 2 {
			continue
		}

		currEvent.URI = uri
		switch string(spl[0]) {
		case eName:
			currEvent.Type = string(bytes.TrimSpace(spl[1]))
		case dName:
			currEvent.Data = bytes.NewBuffer(bytes.TrimSpace(spl[1]))
			// log.Printf("\nWriting to event %#v", currEvent)
			evCh <- currEvent
			// log.Printf("\nDone writing  ..")
		}

		if err == io.EOF {
			break
		}
	}

	return nil
}
