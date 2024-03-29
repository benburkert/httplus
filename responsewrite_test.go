// Copyright 2010 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package httplus

import (
	"bytes"
	"io/ioutil"
	"strings"
	"testing"
)

type respWriteTest struct {
	Resp Response
	Raw  string
}

func TestResponseWrite(t *testing.T) {
	respWriteTests := []respWriteTest{
		// HTTP/1.0, identity coding; no trailer
		{
			Response{
				StatusCode:    503,
				ProtoMajor:    1,
				ProtoMinor:    0,
				Request:       dummyReq("GET"),
				Header:        Header{},
				Body:          ioutil.NopCloser(strings.NewReader("abcdef")),
				ContentLength: 6,
			},

			"HTTP/1.0 503 Service Unavailable\r\n" +
				"Content-Length: 6\r\n\r\n" +
				"abcdef",
		},
		// Unchunked response without Content-Length.
		{
			Response{
				StatusCode:    200,
				ProtoMajor:    1,
				ProtoMinor:    0,
				Request:       dummyReq("GET"),
				Header:        Header{},
				Body:          ioutil.NopCloser(strings.NewReader("abcdef")),
				ContentLength: -1,
			},
			"HTTP/1.0 200 OK\r\n" +
				"\r\n" +
				"abcdef",
		},
		// HTTP/1.1 response with unknown length and Connection: close
		{
			Response{
				StatusCode:    200,
				ProtoMajor:    1,
				ProtoMinor:    1,
				Request:       dummyReq("GET"),
				Header:        Header{},
				Body:          ioutil.NopCloser(strings.NewReader("abcdef")),
				ContentLength: -1,
				Close:         true,
			},
			"HTTP/1.1 200 OK\r\n" +
				"Connection: close\r\n" +
				"\r\n" +
				"abcdef",
		},
		// HTTP/1.1 response with unknown length and not setting connection: close
		{
			Response{
				StatusCode:    200,
				ProtoMajor:    1,
				ProtoMinor:    1,
				Request:       dummyReq11("GET"),
				Header:        Header{},
				Body:          ioutil.NopCloser(strings.NewReader("abcdef")),
				ContentLength: -1,
				Close:         false,
			},
			"HTTP/1.1 200 OK\r\n" +
				"Connection: close\r\n" +
				"\r\n" +
				"abcdef",
		},
		// HTTP/1.1 response with unknown length and not setting connection: close, but
		// setting chunked.
		{
			Response{
				StatusCode:       200,
				ProtoMajor:       1,
				ProtoMinor:       1,
				Request:          dummyReq11("GET"),
				Header:           Header{},
				Body:             ioutil.NopCloser(strings.NewReader("abcdef")),
				ContentLength:    -1,
				TransferEncoding: []string{"chunked"},
				Close:            false,
			},
			"HTTP/1.1 200 OK\r\n" +
				"Transfer-Encoding: chunked\r\n\r\n" +
				"6\r\nabcdef\r\n0\r\n\r\n",
		},
		// HTTP/1.1 response 0 content-length, and nil body
		{
			Response{
				StatusCode:    200,
				ProtoMajor:    1,
				ProtoMinor:    1,
				Request:       dummyReq11("GET"),
				Header:        Header{},
				Body:          nil,
				ContentLength: 0,
				Close:         false,
			},
			"HTTP/1.1 200 OK\r\n" +
				"Content-Length: 0\r\n" +
				"\r\n",
		},
		// HTTP/1.1 response 0 content-length, and non-nil empty body
		{
			Response{
				StatusCode:    200,
				ProtoMajor:    1,
				ProtoMinor:    1,
				Request:       dummyReq11("GET"),
				Header:        Header{},
				Body:          ioutil.NopCloser(strings.NewReader("")),
				ContentLength: 0,
				Close:         false,
			},
			"HTTP/1.1 200 OK\r\n" +
				"Content-Length: 0\r\n" +
				"\r\n",
		},
		// HTTP/1.1 response 0 content-length, and non-nil non-empty body
		{
			Response{
				StatusCode:    200,
				ProtoMajor:    1,
				ProtoMinor:    1,
				Request:       dummyReq11("GET"),
				Header:        Header{},
				Body:          ioutil.NopCloser(strings.NewReader("foo")),
				ContentLength: 0,
				Close:         false,
			},
			"HTTP/1.1 200 OK\r\n" +
				"Connection: close\r\n" +
				"\r\nfoo",
		},
		// HTTP/1.1, chunked coding; empty trailer; close
		{
			Response{
				StatusCode:       200,
				ProtoMajor:       1,
				ProtoMinor:       1,
				Request:          dummyReq("GET"),
				Header:           Header{},
				Body:             ioutil.NopCloser(strings.NewReader("abcdef")),
				ContentLength:    6,
				TransferEncoding: []string{"chunked"},
				Close:            true,
			},

			"HTTP/1.1 200 OK\r\n" +
				"Connection: close\r\n" +
				"Transfer-Encoding: chunked\r\n\r\n" +
				"6\r\nabcdef\r\n0\r\n\r\n",
		},

		// Header value with a newline character (Issue 914).
		// Also tests removal of leading and trailing whitespace.
		{
			Response{
				StatusCode: 204,
				ProtoMajor: 1,
				ProtoMinor: 1,
				Request:    dummyReq("GET"),
				Header: Header{
					"Foo": []string{" Bar\nBaz "},
				},
				Body:             nil,
				ContentLength:    0,
				TransferEncoding: []string{"chunked"},
				Close:            true,
			},

			"HTTP/1.1 204 No Content\r\n" +
				"Connection: close\r\n" +
				"Foo: Bar Baz\r\n" +
				"\r\n",
		},

		// Want a single Content-Length header. Fixing issue 8180 where
		// there were two.
		{
			Response{
				StatusCode:       StatusOK,
				ProtoMajor:       1,
				ProtoMinor:       1,
				Request:          &Request{Method: "POST"},
				Header:           Header{},
				ContentLength:    0,
				TransferEncoding: nil,
				Body:             nil,
			},
			"HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n",
		},
	}

	for i := range respWriteTests {
		tt := &respWriteTests[i]
		var braw bytes.Buffer
		err := tt.Resp.Write(&braw)
		if err != nil {
			t.Errorf("error writing #%d: %s", i, err)
			continue
		}
		sraw := braw.String()
		if sraw != tt.Raw {
			t.Errorf("Test %d, expecting:\n%q\nGot:\n%q\n", i, tt.Raw, sraw)
			continue
		}
	}
}
