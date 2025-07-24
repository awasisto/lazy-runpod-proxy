package main

import (
	"bytes"
	"github.com/gorilla/websocket"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	listenPort           = os.Getenv("LISTEN_PORT")
	runpodApiKey         = os.Getenv("RUNPOD_API_KEY")
	podId                = os.Getenv("POD_ID")
	targetBaseUrl        = os.Getenv("TARGET_BASE_URL")
	targetBaseWsUrl      string
	podOrServiceStarting = false
	podRunning           = true
	inactivityLimit      = 20 * 60 * time.Second
	startTimeLimit       = 5 * 60 * time.Second
	retryInterval        = 5 * time.Second
	lastActivityTime     time.Time

	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	dialer = websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}
)

func main() {
	if inactivityLimitSeconds, err := strconv.Atoi(os.Getenv("INACTIVITY_LIMIT_SECONDS")); err == nil && inactivityLimitSeconds > 0 {
		inactivityLimit = time.Duration(inactivityLimitSeconds) * time.Second
	}

	if startTimeLimitSeconds, err := strconv.Atoi(os.Getenv("START_TIME_LIMIT_SECONDS")); err == nil && startTimeLimitSeconds > 0 {
		startTimeLimit = time.Duration(startTimeLimitSeconds) * time.Second
	}

	if retryIntervalSeconds, err := strconv.Atoi(os.Getenv("RETRY_INTERVAL_SECONDS")); err == nil && retryIntervalSeconds > 0 {
		retryInterval = time.Duration(retryIntervalSeconds) * time.Second
	}

	targetBaseWsUrl = strings.Replace(targetBaseUrl, "https://", "wss://", 1)
	targetBaseWsUrl = strings.Replace(targetBaseWsUrl, "http://", "ws://", 1)

	lastActivityTime = time.Now()

	go monitorInactivity()

	http.HandleFunc("/", proxyHandler)

	if listenPort == "" {
		listenPort = "8080"
	}
	log.Printf("Listening on 0.0.0.0:%s...", listenPort)
	log.Fatal(http.ListenAndServe("0.0.0.0:"+listenPort, nil))
}

func proxyHandler(w http.ResponseWriter, r *http.Request) {
	lastActivityTime = time.Now()

	if isWebSocketRequest(r) {
		log.Printf("Received %s %s from %s (WebSocket upgrade request)", r.Method, r.URL.Path, getRemoteAddress(r))
		proxyWebSocket(w, r)
		return
	}

	log.Printf("Received %s %s from %s", r.Method, r.URL.Path, getRemoteAddress(r))

	var body []byte
	if r.Body != nil {
		body, _ = io.ReadAll(r.Body)
		_ = r.Body.Close()
	}

	startTime := time.Now()

	for {
		if time.Since(startTime) > startTimeLimit {
			log.Println("Service did not start in time, giving up.")
			http.Error(w, "Service did not start in time", http.StatusGatewayTimeout)
			return
		}

		resp, err := forwardRequest(r.Method, r.URL.String(), r.Header, body)
		if err != nil {
			log.Printf("Error forwarding request: %v", err)
			http.Error(w, "Error forwarding request", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode == 502 || resp.StatusCode == 530 {
			if !podOrServiceStarting {
				podOrServiceStarting = true
				podRunning = false
				log.Printf("Received %d from target - Pod is not running, starting pod...", resp.StatusCode)
				startPod()
			} else {
				log.Printf("Received %d from target - Pod is still starting, retrying...", resp.StatusCode)
			}

			time.Sleep(retryInterval)
			continue
		}

		if resp.StatusCode == 503 {
			log.Printf("Received %d from target - Service is not ready, retrying...", resp.StatusCode)
			time.Sleep(retryInterval)
			continue
		}

		if podOrServiceStarting {
			podOrServiceStarting = false
			podRunning = true
			log.Println("Pod started and service is ready.")
		}

		err = copyResponse(w, resp)
		if err != nil {
			log.Printf("Error processing response: %v", err)
			http.Error(w, "Error processing response", http.StatusInternalServerError)
		}
		return
	}
}

func forwardRequest(method, path string, headers http.Header, body []byte) (*http.Response, error) {
	req, err := http.NewRequest(method, targetBaseUrl+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header = headers.Clone()
	client := &http.Client{}
	return client.Do(req)
}

func copyResponse(w http.ResponseWriter, resp *http.Response) error {
	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)

	if isStreamingResponse(resp) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			log.Println("Response writer does not support flushing, cannot stream response.")
			_, err := io.Copy(w, resp.Body)
			return err
		}
		return streamResponse(w, resp.Body, flusher)
	} else {
		_, err := io.Copy(w, resp.Body)
		return err
	}
}

func isStreamingResponse(resp *http.Response) bool {
	if contentType := resp.Header.Get("Content-Type"); strings.Contains(contentType, "text/event-stream") {
		return true
	}

	if contentType := resp.Header.Get("Content-Type"); strings.Contains(contentType, "application/json") {
		if resp.Header.Get("Transfer-Encoding") == "chunked" {
			return true
		}
		if resp.Header.Get("X-Accel-Buffering") == "no" {
			return true
		}
	}

	if resp.Header.Get("Transfer-Encoding") == "chunked" {
		return true
	}

	if resp.Header.Get("Cache-Control") == "no-cache" && resp.Header.Get("Connection") == "keep-alive" {
		return true
	}

	return false
}

func streamResponse(w http.ResponseWriter, body io.ReadCloser, flusher http.Flusher) error {
	buffer := make([]byte, 1024)

	for {
		n, err := body.Read(buffer)
		if n > 0 {
			if _, writeErr := w.Write(buffer[:n]); writeErr != nil {
				return writeErr
			}

			flusher.Flush()

			lastActivityTime = time.Now()
		}

		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

func isWebSocketRequest(r *http.Request) bool {
	return strings.ToLower(r.Header.Get("Connection")) == "upgrade" &&
		strings.ToLower(r.Header.Get("Upgrade")) == "websocket"
}

func filterWebSocketHeaders(src http.Header) http.Header {
	filtered := http.Header{}
	skip := map[string]bool{
		"Connection":               true,
		"Upgrade":                  true,
		"Sec-Websocket-Key":        true,
		"Sec-Websocket-Version":    true,
		"Sec-Websocket-Protocol":   true,
		"Sec-Websocket-Extensions": true,
	}

	for k, v := range src {
		if !skip[http.CanonicalHeaderKey(k)] {
			filtered[k] = v
		}
	}

	return filtered
}

func proxyWebSocket(w http.ResponseWriter, r *http.Request) {
	clientConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade client connection: %v", err)
		return
	}
	defer clientConn.Close()

	backendConn, resp, err := dialer.Dial(targetBaseWsUrl+r.URL.RequestURI(), filterWebSocketHeaders(r.Header))
	if err != nil {
		log.Printf("WebSocket dial error: %v", err)
		if resp != nil {
			log.Printf("WebSocket dial response status: %d", resp.StatusCode)
			log.Printf("WebSocket dial response headers: %v", resp.Header)

			if resp.Body != nil {
				body, readErr := io.ReadAll(resp.Body)
				if readErr == nil {
					log.Printf("WebSocket dial response body: %s", string(body))
				}
				resp.Body.Close()
			}
		} else {
			log.Printf("WebSocket dial failed with no response - network error: %v", err)
		}

		clientConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "Backend connection failed"))
		return
	}
	defer backendConn.Close()

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			messageType, message, err := clientConn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("Client read error: %v", err)
				}
				break
			}

			if err := backendConn.WriteMessage(messageType, message); err != nil {
				log.Printf("Backend write error: %v", err)
				break
			}
		}
	}()

	go func() {
		for {
			messageType, message, err := backendConn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("Backend read error: %v", err)
				}
				break
			}

			if err := clientConn.WriteMessage(messageType, message); err != nil {
				log.Printf("Client write error: %v", err)
				break
			}
		}
	}()

	<-done
}

func startPod() {
	req, _ := http.NewRequest("POST", "https://rest.runpod.io/v1/pods/"+podId+"/start", nil)
	req.Header.Set("Authorization", "Bearer "+runpodApiKey)
	client := &http.Client{}
	_, err := client.Do(req)
	if err != nil {
		podOrServiceStarting = false
		log.Println("Error starting pod:", err)
	}
}

func stopPod(retries int) {
	req, _ := http.NewRequest("POST", "https://rest.runpod.io/v1/pods/"+podId+"/stop", nil)
	req.Header.Set("Authorization", "Bearer "+runpodApiKey)
	client := &http.Client{}
	_, err := client.Do(req)
	if err != nil {
		if retries < 3 {
			log.Printf("Error stopping pod, retrying (%d/3)...", retries+1)
			time.Sleep(10 * time.Second)
			stopPod(retries + 1)
		} else {
			log.Println("Failed to stop pod after multiple attempts:", err)
		}
	} else {
		podRunning = false
	}
}

func monitorInactivity() {
	for {
		time.Sleep(1 * time.Minute)
		idle := time.Since(lastActivityTime)
		if idle > inactivityLimit {
			if podRunning {
				log.Println("Stopping pod due to inactivity...")
				stopPod(0)
			}
		}
	}
}

func getRemoteAddress(r *http.Request) string {
	forwardedFor := r.Header.Get("X-Forwarded-For")
	if forwardedFor != "" {
		return strings.TrimSpace(strings.Split(forwardedFor, ",")[0])
	}

	realIp := r.Header.Get("X-Real-IP")
	if realIp != "" {
		return realIp
	}

	if r.RemoteAddr != "" {
		if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
			return host
		} else {
			return r.RemoteAddr
		}
	}

	return "unknown"
}
