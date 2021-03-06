package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"net"

	"github.com/Sirupsen/logrus"
	"github.com/getcarina/carina/version"
)

// HTTPLog satisfies the http.RoundTripper interface and is used to
// customize the default Gophercloud RoundTripper to allow for logging.
type HTTPLog struct {
	Logger *logrus.Logger
	rt     http.RoundTripper
}

// NewHTTPClient return a custom HTTP client that allows for logging relevant
// information before and after the HTTP request.
func NewHTTPClient() *http.Client {
	timeout := 10 * time.Second
	return &http.Client{
		Timeout: timeout,
		Transport: &HTTPLog{
			rt: &http.Transport{
				Proxy:             http.ProxyFromEnvironment,
				DisableKeepAlives: true, // KeepAlive was causing "connection reset by peer" errors when issuing multiple requests
				Dial: (&net.Dialer{
					Timeout: timeout,
				}).Dial,
				TLSHandshakeTimeout:   timeout,
				ExpectContinueTimeout: 1 * time.Second,
			},
			Logger: Log.Logger,
		},
	}
}

// RoundTrip performs a round-trip HTTP request and logs relevant information about it.
func (hl *HTTPLog) RoundTrip(request *http.Request) (*http.Response, error) {
	defer func() {
		if request.Body != nil {
			request.Body.Close()
		}
	}()

	var err error

	// Inject user agent
	request.Header.Add("User-Agent", "getcarina/carina "+version.Version)

	if hl.Logger.Level == logrus.DebugLevel && request.Body != nil {
		request.Body, err = hl.logRequestBody(request.Body, request.Header)
		if err != nil {
			return nil, err
		}
	}

	// Don't log the token embedded in a cached auth token check
	url := request.URL.String()
	if strings.Contains(url, "tokens") {
		url = fmt.Sprintf("%s://%s/***", request.URL.Scheme, request.URL.Host)
	}

	hl.Logger.Debugf("Request: %s %s", request.Method, url)

	response, err := hl.rt.RoundTrip(request)
	if response == nil {
		return nil, err
	}

	responseBody, _ := hl.logResponseBody(response.Body, response.Header)
	response.Body = responseBody

	if response.StatusCode >= 400 {
		hl.Logger.Debugf("Response Code: %d %s", response.StatusCode, response.Status)
		buf := bytes.NewBuffer([]byte{})
		body, _ := ioutil.ReadAll(io.TeeReader(response.Body, buf))
		hl.Logger.Debugf("Response Error: %+v", string(body))
		bufWithClose := ioutil.NopCloser(buf)
		response.Body = bufWithClose
	}

	return response, err
}

func (hl *HTTPLog) logRequestBody(original io.ReadCloser, headers http.Header) (io.ReadCloser, error) {
	defer original.Close()

	var bs bytes.Buffer
	_, err := io.Copy(&bs, original)
	if err != nil {
		return nil, err
	}

	contentType := headers.Get("Content-Type")
	if strings.HasPrefix(contentType, "application/json") {
		debugInfo := hl.formatJSON(bs.Bytes())
		hl.Logger.Debugf("Request Options: %s", debugInfo)
	} else {
		hl.Logger.Debugf("Request Options: %s", bs.String())
	}

	return ioutil.NopCloser(strings.NewReader(bs.String())), nil
}

func (hl *HTTPLog) logResponseBody(original io.ReadCloser, headers http.Header) (io.ReadCloser, error) {
	defer original.Close()

	// Log the request id, if present
	for key, value := range headers {
		if strings.Contains(strings.ToLower(key), "request-id") {
			requestID := value[0]
			hl.Logger.Debugf("Request ID: %s", requestID)
			Log.ErrorContext["Request ID"] = requestID
			break
		}
	}

	var bs bytes.Buffer
	_, err := io.Copy(&bs, original)
	if err != nil {
		return nil, err
	}

	contentType := headers.Get("Content-Type")
	if strings.HasPrefix(contentType, "application/json") {
		debugInfo := hl.formatJSON(bs.Bytes())
		if debugInfo != "" {
			hl.Logger.Debugf("Response Body: %s", debugInfo)
		}
	} else {
		hl.Logger.Debugf("Not logging because response body isn't JSON")
	}

	return ioutil.NopCloser(strings.NewReader(bs.String())), nil
}

func (hl *HTTPLog) formatJSON(raw []byte) string {
	var data map[string]interface{}

	err := json.Unmarshal(raw, &data)
	if err != nil {
		return string(raw)
	}

	pretty, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return string(raw)
	}

	return string(pretty)
}
