// Copyright 2016 Afshin Darian. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package sleuth

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/ursiform/logger"
)

func init() {
	// Tests should run using a different group than production.
	group = "SLEUTH-vT"
}

// testCodes compares the error codes in an error with a list of wanted codes.
func testCodes(t *testing.T, err error, want []int) {
	codes := err.(*Error).Codes
	if len(codes) != len(want) {
		t.Errorf("expected codes length %d to be %d", len(codes), len(want))
		return
	}
	for i, code := range codes {
		if code != want[i] {
			t.Errorf("expected code [%d] to be [%d]", code, want[i])
			return
		}
	}
}

// Test client.go units.

func TestClientAddBadMember(t *testing.T) {
	log, _ := logger.New(logger.Silent)
	c := newClient(nil, log)
	err := c.add(group, "foo", "bar", "", "")
	if err == nil {
		t.Errorf("expected client dispatch to fail on bad action")
	}
	testCodes(t, err, []int{errAdd})
}

func TestClientDispatchBadAction(t *testing.T) {
	log, _ := logger.New(logger.Silent)
	c := newClient(nil, log)
	err := c.dispatch([]byte(group + "FAIL"))
	if err == nil {
		t.Errorf("expected client dispatch to fail on bad action")
	}
	testCodes(t, err, []int{errDispatchAction})
}

func TestClientDispatchEmpty(t *testing.T) {
	log, _ := logger.New(logger.Silent)
	c := newClient(nil, log)
	err := c.dispatch([]byte{})
	if err == nil {
		t.Errorf("expected client dispatch to fail on empty payload")
	}
	testCodes(t, err, []int{errDispatchHeader})
}

func TestClientDoUnknownScheme(t *testing.T) {
	log, _ := logger.New(logger.Silent)
	c := newClient(nil, log)
	req, _ := http.NewRequest("POST", "foo://bar/baz", nil)
	_, err := c.Do(req)
	if err == nil {
		t.Errorf("expected client Do to fail on unknown scheme")
	}
	testCodes(t, err, []int{errScheme})
}

func TestClientDoUnknownService(t *testing.T) {
	log, _ := logger.New(logger.Silent)
	c := newClient(nil, log)
	req, _ := http.NewRequest("POST", "sleuth://foo/bar", nil)
	_, err := c.Do(req)
	if err == nil {
		t.Errorf("expected client Do to fail on unknown service")
	}
	testCodes(t, err, []int{errUnknownService})
}

func TestClientReceiveBadHandle(t *testing.T) {
	log, _ := logger.New(logger.Silent)
	c := newClient(nil, log)
	res := &response{Handle: "foo"}
	err := c.receive(marshalResponse(res)[len(group)+len(recv):])
	if err == nil {
		t.Errorf("expected client receive to fail on bad handle")
	}
	testCodes(t, err, []int{errRECV})
}

func TestClientReceiveBadPayload(t *testing.T) {
	log, _ := logger.New(logger.Silent)
	c := newClient(nil, log)
	err := c.receive([]byte(""))
	if err == nil {
		t.Errorf("expected client receive to fail on bad payload")
	}
	testCodes(t, err, []int{errUnzip, errResUnmarshal, errRECV})
}

func TestClientReplyBadPayload(t *testing.T) {
	log, _ := logger.New(logger.Silent)
	c := newClient(nil, log)
	err := c.reply([]byte(""))
	if err == nil {
		t.Errorf("expected client reply to fail on bad payload")
	}
	testCodes(t, err, []int{errUnzip, errReqUnmarshal, errREPL})
}

// Test error.go units.

func TestError(t *testing.T) {
	code := 1
	message := "test"
	want := "sleuth: test [1]"
	err := newError(code, message)
	if err.Error() != want {
		t.Errorf("expected error to be formatted as: %s", want)
	}
}

// Test request.go units.

func TestUnmarshalRequestBadJSON(t *testing.T) {
	payload := zip([]byte("{bad json}"))
	_, _, err := unmarshalRequest(payload)
	if err == nil {
		t.Errorf("expected unmarshalRequest to fail on bad json")
	}
	testCodes(t, err, []int{errReqUnmarshalJSON})
}

// Test response.go units.

func TestUnmarshalResponseBadJSON(t *testing.T) {
	payload := zip([]byte("{bad json}"))
	_, _, err := unmarshalResponse(payload)
	if err == nil {
		t.Errorf("expected unmarshalResponse to fail on bad json")
	}
	testCodes(t, err, []int{errResUnmarshalJSON})
}

// Test sleuth.go units.

func TestNewBadInterface(t *testing.T) {
	_, err := New(&Config{Interface: "foo"})
	if err == nil {
		t.Errorf("expected New to fail on start with bad interface")
	}
	testCodes(t, err, []int{errStart, errCreate, errNew})
}

func TestNewBadLogLevel(t *testing.T) {
	c, _ := New(&Config{LogLevel: "foo"})
	if c.log.Level() != logger.Debug {
		t.Errorf("expected log level 'foo' to be coerced to 'debug'")
		return
	}
}

func TestNewBadPort(t *testing.T) {
	_, err := New(&Config{Port: 1})
	if err == nil {
		t.Errorf("expected New to fail on start with bad port")
	}
	testCodes(t, err, []int{errStart, errCreate, errNew})
}

func TestNewBadService(t *testing.T) {
	_, err := New(&Config{Handler: http.FileServer(http.Dir("."))})
	if err == nil {
		t.Errorf("expected New to fail with bad service")
	}
	testCodes(t, err, []int{errService})
}

// Test workers.go units.

func TestWorkersAddDuplicate(t *testing.T) {
	w := newWorkers()
	p := &peer{name: "foo", node: "bar", service: "baz"}
	if n := w.add(p); n != w.add(p) {
		t.Error("expected duplicate addition to be ignored")
	}
}

func TestWorkersAvailable(t *testing.T) {
	w := newWorkers()
	p := &peer{name: "foo", node: "bar", service: "baz"}
	if w.add(p); !w.available() {
		t.Error("expected workers to be available")
	}
}

func TestWorkersNext(t *testing.T) {
	w := newWorkers()
	w.add(&peer{name: "foo", node: "bar", service: "baz"})
	w.add(&peer{name: "qux", node: "quux", service: "corge"})
	if w.next().name != "foo" {
		t.Error("expected next to return first added item")
	}
	if w.next().name != "qux" {
		t.Error("expected next to return second added item")
	}
	if w.next().name != "foo" {
		t.Error("expected next to loop back to first added item")
	}
}

func TestWorkersNextNonexistent(t *testing.T) {
	w := newWorkers()
	if p := w.next(); p != nil {
		t.Error("expected nonexistent worker to be nil")
	}
}

func TestWorkersRemove(t *testing.T) {
	w := newWorkers()
	w.add(&peer{name: "foo", node: "bar", service: "baz"})
	if _, p := w.remove("foo"); p == nil || p.name != "foo" {
		t.Error("expected worker to be removed")
	}
}

func TestWorkersRemoveNonexistent(t *testing.T) {
	w := newWorkers()
	if _, p := w.remove("foo"); p != nil {
		t.Error("expected nonexistent worker removal to return nil")
	}
}

// Test writer.go units.

type goodWhisperer struct{}

func (g *goodWhisperer) Whisper(addr string, payload []byte) error {
	return nil
}

type badWhisperer struct{}

func (b *badWhisperer) Whisper(addr string, payload []byte) error {
	return errors.New("mock whisper error")
}

func TestWriterWrite(t *testing.T) {
	data := []byte("foo bar baz")
	w := newWriter(new(goodWhisperer), &destination{node: "qux", handle: "quux"})
	if n, err := w.Write(data); err != nil {
		t.Errorf("expected write to succeed: %s", err.Error())
	} else if n <= 0 {
		t.Errorf("expected (%d) to be greater than 0", n)
	}
}

func TestWriterErrWhisper(t *testing.T) {
	data := []byte("foo bar baz")
	w := newWriter(new(badWhisperer), &destination{node: "qux", handle: "quux"})
	if _, err := w.Write(data); err == nil {
		t.Errorf("expected writer to fail")
	}
}

// Test zip.go units.

func TestZipUnzip(t *testing.T) {
	in := []byte("a value that should be zipped")
	zipped := zip(in)
	if out, err := unzip(zipped); err != nil {
		t.Errorf("unzip failed: ", err.Error())
	} else if string(out) != string(in) {
		t.Errorf("zip failed")
	}
}

func TestUnzipErr(t *testing.T) {
	in := []byte("a value that cannot be unzipped")
	_, err := unzip(in)
	testCodes(t, err, []int{errUnzip})
}

// Test integrated package.

type echoHandler struct{}

func (h *echoHandler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	body, _ := ioutil.ReadAll(req.Body)
	res.Write(body)
}

type silentHandler struct{}

func (*silentHandler) ServeHTTP(http.ResponseWriter, *http.Request) {
	// Bad handler does nothing.
}

func TestRequestResponseCycle(t *testing.T) {
	// Create client.
	client, err := New(nil)
	if err != nil {
		t.Errorf("client instantiation failed: %s", err.Error())
		return
	}
	defer func(client *Client, t *testing.T) {
		if err := client.Close(); err != nil {
			t.Errorf("client close failed: %s", err.Error())
		}
	}(client, t)
	// Create server.
	addr := "sleuth-test"
	server, err := New(&Config{
		Handler: new(echoHandler), Service: addr})
	if err != nil {
		t.Errorf("server instantiation failed: %s", err.Error())
		return
	}
	defer func(server *Client, t *testing.T) {
		if err := server.Close(); err != nil {
			t.Errorf("server close failed: %s", err.Error())
		}
	}(server, t)
	// Wait until the server has been added to the client pool.
	client.WaitFor(addr)
	// Set timeout to 10 seconds to accommodate slow test spin-up.
	client.Timeout = time.Second * 10
	if client.block(addr) {
		t.Errorf("call to block should have returned immediately")
	}
	body := "foo bar baz"
	buffer := bytes.NewBuffer([]byte(body))
	request, err := http.NewRequest("GET", scheme+"://"+addr+"/", buffer)
	if err != nil {
		t.Errorf("request instantiation failed: %s", err.Error())
		return
	}
	response, err := client.Do(request)
	if err != nil {
		t.Errorf("client.Do failed: %s", err.Error())
		return
	}
	if response.StatusCode != http.StatusOK {
		t.Errorf("client.Do expected %d got %d", http.StatusOK, response.StatusCode)
		return
	}
	output, _ := ioutil.ReadAll(response.Body)
	// It's not really necessary to close a sleuth response.
	response.Body.Close()
	if string(output) != body {
		t.Errorf("client.Do expected %s to equal %s", string(output), body)
		return
	}
}

func TestTimeout(t *testing.T) {
	// Create server.
	addr := "sleuth-test-server-three"
	server, err := New(&Config{Handler: new(silentHandler), Service: addr})
	if err != nil {
		t.Errorf("server instantiation failed: %s", err.Error())
		return
	}
	defer func(server *Client, t *testing.T) {
		if err := server.Close(); err != nil {
			t.Errorf("server close failed: %s", err.Error())
		}
	}(server, t)
	// Create client.
	client, err := New(nil)
	if err != nil {
		t.Errorf("client instantiation failed: %s", err.Error())
		return
	}
	defer func(client *Client, t *testing.T) {
		if err := client.Close(); err != nil {
			t.Errorf("client close failed: %s", err.Error())
		}
	}(client, t)
	// Wait long enough that the server should be ready (but not guaranteed).
	// If the server is ready, then WaitFor will not block.
	<-time.After(2000 * time.Millisecond)
	// Wait until the server has been added to the client pool.
	client.WaitFor(addr)
	request, err := http.NewRequest("GET", scheme+"://"+addr+"/", nil)
	if err != nil {
		t.Errorf("request instantiation failed: %s", err.Error())
		return
	}
	_, err = client.Do(request)
	if err == nil {
		t.Errorf("client request should have timed out")
		return
	}
}
