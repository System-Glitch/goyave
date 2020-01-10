package goyave

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/System-Glitch/goyave/v2/database"
	"github.com/System-Glitch/goyave/v2/helper/filesystem"

	"github.com/System-Glitch/goyave/v2/config"
	"github.com/System-Glitch/goyave/v2/lang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	testify "github.com/stretchr/testify/suite"
)

// ITestSuite is an extension of testify's Suite for
// Goyave-specific testing.
type ITestSuite interface {
	RunServer(func(*Router), func())
	Timeout() time.Duration
	SetTimeout(time.Duration)
	Middleware(Middleware, *Request, Handler) *http.Response

	Get(string, map[string]string) (*http.Response, error)
	Post(string, map[string]string, io.Reader) (*http.Response, error)
	Put(string, map[string]string, io.Reader) (*http.Response, error)
	Patch(string, map[string]string, io.Reader) (*http.Response, error)
	Delete(string, map[string]string, io.Reader) (*http.Response, error)
	Request(string, string, map[string]string, io.Reader) (*http.Response, error)

	T() *testing.T
	SetT(*testing.T)

	GetBody(*http.Response) []byte
	GetJSONBody(*http.Response, interface{}) error
	CreateTestFiles(paths ...string) []filesystem.File
	WriteFile(*multipart.Writer, string, string, string)
	WriteField(*multipart.Writer, string, string)
	CreateTestRequest(*http.Request) *Request
	CreateTestResponse(http.ResponseWriter) *Response
	getHTTPClient() *http.Client
}

// TestSuite is an extension of testify's Suite for
// Goyave-specific testing.
type TestSuite struct {
	suite.Suite
	timeout     time.Duration // Timeout for functional tests
	httpClient  *http.Client
	previousEnv string
	mu          sync.Mutex
}

// Timeout get the timeout for test failure when using RunServer or requests.
func (s *TestSuite) Timeout() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.timeout
}

// SetTimeout set the timeout for test failure when using RunServer or requests.
func (s *TestSuite) SetTimeout(timeout time.Duration) {
	s.mu.Lock()
	s.timeout = timeout
	s.mu.Unlock()
}

// CreateTestRequest create a "goyave.Request" from the given raw request.
// This function is aimed at making it easier to unit test Requests.
//
// If passed request is "nil", a default GET request to "/" is used.
//
//  rawRequest := httptest.NewRequest("GET", "/test-route", nil)
//  rawRequest.Header.Set("Content-Type", "application/json")
//  request := goyave.CreateTestRequest(rawRequest)
//  request.Lang = "en-US"
//  request.Data = map[string]interface{}{"field": "value"}
func (s *TestSuite) CreateTestRequest(rawRequest *http.Request) *Request {
	if rawRequest == nil {
		rawRequest = httptest.NewRequest("GET", "/", nil)
	}
	return &Request{
		httpRequest: rawRequest,
		Data:        nil,
		Rules:       nil,
		Lang:        "en-US",
		Params:      map[string]string{},
	}
}

// CreateTestResponse create an empty response with the given response writer.
// This function is aimed at making it easier to unit test Responses.
//
//  writer := httptest.NewRecorder()
//  response := goyave.CreateTestResponse(writer)
//  response.Status(http.StatusNoContent)
//  result := writer.Result()
//  fmt.Println(result.StatusCode) // 204
func (s *TestSuite) CreateTestResponse(recorder http.ResponseWriter) *Response {
	return &Response{
		ResponseWriter: recorder,
		empty:          true,
	}
}

// RunServer start the application and run the given functional test procedure.
//
// This function is the equivalent of "goyave.Start()".
// The test fails if the suite's timeout is exceeded.
// The server automatically shuts down when the function ends.
// This function is synchronized, that means that the server is properly stopped
// when the function returns.
func (s *TestSuite) RunServer(routeRegistrer func(*Router), procedure func()) {
	c := make(chan bool, 1)
	c2 := make(chan bool, 1)
	ctx, cancel := context.WithTimeout(context.Background(), s.Timeout())
	defer cancel()

	RegisterStartupHook(func() {
		procedure()
		if ctx.Err() == nil {
			Stop()
			ClearStartupHooks()
			c <- true
		}
	})

	go func() {
		Start(routeRegistrer)
		c2 <- true
	}()

	select {
	case <-ctx.Done():
		s.Fail("Timeout exceeded in goyave.TestSuite.RunServer")
		Stop()
		ClearStartupHooks()
	case sig := <-c:
		s.True(sig)
	}
	<-c2
}

// Middleware executes the given middleware and returns the HTTP response.
// Core middleware (recovery, parsing and language) is not executed.
func (s *TestSuite) Middleware(middleware Middleware, request *Request, procedure Handler) *http.Response {
	recorder := httptest.NewRecorder()
	response := s.CreateTestResponse(recorder)
	router := newRouter()
	router.Middleware(middleware)
	middleware(procedure)(response, request)
	router.finalize(response, request)

	return recorder.Result()
}

// Get execute a GET request on the given route.
// Headers are optional.
func (s *TestSuite) Get(route string, headers map[string]string) (*http.Response, error) {
	return s.Request("GET", route, headers, nil)
}

// Post execute a POST request on the given route.
// Headers and body are optional.
func (s *TestSuite) Post(route string, headers map[string]string, body io.Reader) (*http.Response, error) {
	return s.Request("POST", route, headers, body)
}

// Put execute a PUT request on the given route.
// Headers and body are optional.
func (s *TestSuite) Put(route string, headers map[string]string, body io.Reader) (*http.Response, error) {
	return s.Request("PUT", route, headers, body)
}

// Patch execute a PATCH request on the given route.
// Headers and body are optional.
func (s *TestSuite) Patch(route string, headers map[string]string, body io.Reader) (*http.Response, error) {
	return s.Request("PATCH", route, headers, body)
}

// Delete execute a DELETE request on the given route.
// Headers and body are optional.
func (s *TestSuite) Delete(route string, headers map[string]string, body io.Reader) (*http.Response, error) {
	return s.Request("DELETE", route, headers, body)
}

// Request execute a request on the given route.
// Headers and body are optional.
func (s *TestSuite) Request(method, route string, headers map[string]string, body io.Reader) (*http.Response, error) {
	protocol := config.GetString("protocol")
	req, err := http.NewRequest(method, getAddress(protocol)+route, body)
	if err != nil {
		return nil, err
	}
	if headers != nil {
		for k, v := range headers {
			req.Header.Set(k, v)
		}
	}
	return s.getHTTPClient().Do(req)
}

// GetBody read the whole body of a response.
// If read failed, test fails and return empty byte slice.
func (s *TestSuite) GetBody(response *http.Response) []byte {
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		s.Fail("Couldn't read response body", err)
	}
	return body
}

// GetJSONBody read the whole body of a response and decode it as JSON.
// If read or decode failed, test fails.
func (s *TestSuite) GetJSONBody(response *http.Response, data interface{}) error {
	err := json.NewDecoder(response.Body).Decode(&data)
	if err != nil {
		s.Fail("Couldn't read response body as JSON", err)
		return err
	}
	return nil
}

// CreateTestFiles create a slice of "filesystem.File" from the given paths.
// Files are passed to a temporary http request and parsed as Multipart form,
// to reproduce the way files are obtained in real scenarios.
func (s *TestSuite) CreateTestFiles(paths ...string) []filesystem.File {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for _, p := range paths {
		s.WriteFile(writer, p, "file", filepath.Base(p))
	}
	err := writer.Close()
	if err != nil {
		panic(err)
	}

	req, _ := http.NewRequest("POST", "/test-route", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.ParseMultipartForm(10 << 20)
	return filesystem.ParseMultipartFiles(req, "file")
}

// WriteFile write a file to the given writer.
// This function is handy for file upload testing.
// The test fails if an error occurred.
func (s *TestSuite) WriteFile(writer *multipart.Writer, path, fieldName, fileName string) {
	file, err := os.Open(path)
	if err != nil {
		s.Fail(err.Error())
		return
	}
	defer file.Close()
	part, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		s.Fail(err.Error())
		return
	}
	_, err = io.Copy(part, file)
	if err != nil {
		s.Fail(err.Error())
	}
}

// WriteField create and write a new multipart form field.
// The test fails if the field couldn't be written.
func (s *TestSuite) WriteField(writer *multipart.Writer, fieldName, value string) {
	if err := writer.WriteField(fieldName, value); err != nil {
		s.Fail(err.Error())
	}
}

// getHTTPClient get suite's http client or create it if it doesn't exist yet.
// The HTTP client is created with a timeout, disabled redirect and disabled TLS cert checking.
func (s *TestSuite) getHTTPClient() *http.Client {
	config := &tls.Config{
		InsecureSkipVerify: true,
	}

	return &http.Client{
		Timeout:   s.Timeout(),
		Transport: &http.Transport{TLSClientConfig: config},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// ClearDatabase delete all records in all tables.
// This function only clears the tables of registered models.
func (s *TestSuite) ClearDatabase() {
	db := database.GetConnection()
	for _, m := range database.GetRegisteredModels() {
		db.Unscoped().Delete(m)
	}
}

// RunTest run a test suite with prior initialization of a test environment.
// The GOYAVE_ENV environment variable is automatically set to "test" and restored
// to its original value at the end of the test run.
// All tests are run using your project's root as working directory. This directory is determined
// by the presence of a "go.mod" file.
func RunTest(t *testing.T, suite ITestSuite) bool {
	if suite.Timeout() == 0 {
		suite.SetTimeout(5 * time.Second)
	}
	oldEnv := os.Getenv("GOYAVE_ENV")
	os.Setenv("GOYAVE_ENV", "test")
	defer os.Setenv("GOYAVE_ENV", oldEnv)
	setRootWorkingDirectory()
	if err := config.Load(); err != nil {
		return assert.Fail(t, "Failed to load config", err)
	}
	lang.LoadDefault()
	lang.LoadAllAvailableLanguages()

	testify.Run(t, suite)

	database.Close()
	return !t.Failed()
}

func setRootWorkingDirectory() {
	sep := string(os.PathSeparator)
	_, filename, _, _ := runtime.Caller(2)
	directory := path.Dir(filename) + sep
	for !filesystem.FileExists(directory + sep + "go.mod") {
		directory += ".." + sep
		if !filesystem.IsDirectory(directory) {
			panic("Couldn't find project's root directory.")
		}
	}
	os.Chdir(directory)
}
