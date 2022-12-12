package scan

import (
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	log "github.com/sirupsen/logrus"
)

func TestRawRequest(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	testTitle := "TITLE for TESTING"
	listener, err := net.Listen("tcp", "127.0.0.1:8080")
	if err != nil {
		t.Log(err)
	}
	// Start a local HTTP server
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		// t.Log(req)
		// Send response to be tested
		if req.URL.Path == "/favicon.ico" {
			rw.Write([]byte{80, 97, 115, 115, 119, 111, 114, 100, 49, 50, 51})
		} else {
			rw.Write([]byte(` <html> <head> <title>` + testTitle + `</title> </head> </html> `))
		}
	}))
	// Replace the listener created by NewUnstartedServer
	server.Listener.Close()
	server.Listener = listener

	// Start the server
	server.Start()
	// Close the server when test finishes
	defer server.Close()

	var test Scan
	RawRequest(&test, "127.0.0.1", "8080")
	t.Logf("%+v\n", test)
	if test.HTTPstatus != 200 {
		t.Fatalf(`Fail HTTPstatus on RawRequest(scan, host, port)`)
		return
	}
	if test.HTTPtitle != testTitle {
		t.Fatalf(`Fail HTTPtitle on RawRequest(scan, host, port)`)
		return
	}
	if test.HTTPfavicon != 237778598 {
		t.Fatalf(`Fail HTTPfavicon on RawRequest(scan, host, port)`)
		return
	}
}
