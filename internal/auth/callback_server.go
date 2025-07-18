package auth

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

// CallbackServer handles OAuth callbacks for CLI authentication
type CallbackServer struct {
	server       *http.Server
	callbackChan chan CallbackResult
}

// CallbackResult contains the result of an OAuth callback
type CallbackResult struct {
	Code  string
	State string
	Error error
}

// NewCallbackServer creates a new callback server
func NewCallbackServer(port int) *CallbackServer {
	cs := &CallbackServer{
		callbackChan: make(chan CallbackResult, 1),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", cs.handleCallback)

	cs.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	return cs
}

// Start starts the callback server
func (cs *CallbackServer) Start() error {
	listener, err := net.Listen("tcp", cs.server.Addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	go func() {
		if err := cs.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			cs.callbackChan <- CallbackResult{Error: fmt.Errorf("server error: %w", err)}
		}
	}()

	return nil
}

// WaitForCallback waits for an OAuth callback
func (cs *CallbackServer) WaitForCallback(timeout time.Duration) CallbackResult {
	select {
	case result := <-cs.callbackChan:
		return result
	case <-time.After(timeout):
		return CallbackResult{Error: fmt.Errorf("callback timeout")}
	}
}

// Shutdown shuts down the callback server
func (cs *CallbackServer) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return cs.server.Shutdown(ctx)
}

// handleCallback handles the OAuth callback
func (cs *CallbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	errorParam := r.URL.Query().Get("error")
	errorDesc := r.URL.Query().Get("error_description")

	// Check for errors
	if errorParam != "" {
		errMsg := errorParam
		if errorDesc != "" {
			errMsg = fmt.Sprintf("%s: %s", errorParam, errorDesc)
		}
		cs.callbackChan <- CallbackResult{Error: fmt.Errorf("OAuth error: %s", errMsg)}
		cs.writeErrorResponse(w, errMsg)
		return
	}

	// Check for code
	if code == "" {
		cs.callbackChan <- CallbackResult{Error: fmt.Errorf("no authorization code received")}
		cs.writeErrorResponse(w, "No authorization code received")
		return
	}

	// Send success result
	cs.callbackChan <- CallbackResult{
		Code:  code,
		State: state,
	}

	// Write success response
	cs.writeSuccessResponse(w)
}

// writeSuccessResponse writes a success HTML response
func (cs *CallbackServer) writeSuccessResponse(w http.ResponseWriter) {
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Authentication Successful</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            height: 100vh;
            margin: 0;
            background-color: #f5f5f5;
        }
        .message {
            text-align: center;
            padding: 2rem;
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        }
        .success {
            color: #22c55e;
            font-size: 3rem;
            margin-bottom: 1rem;
        }
        h1 {
            font-size: 1.5rem;
            margin-bottom: 0.5rem;
        }
        p {
            color: #666;
            margin: 0;
        }
    </style>
</head>
<body>
    <div class="message">
        <div class="success">✓</div>
        <h1>Authentication Successful</h1>
        <p>You can now close this window and return to the terminal.</p>
    </div>
</body>
</html>`
	
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, html)
}

// writeErrorResponse writes an error HTML response
func (cs *CallbackServer) writeErrorResponse(w http.ResponseWriter, errorMsg string) {
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Authentication Failed</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            height: 100vh;
            margin: 0;
            background-color: #f5f5f5;
        }
        .message {
            text-align: center;
            padding: 2rem;
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
            max-width: 500px;
        }
        .error {
            color: #ef4444;
            font-size: 3rem;
            margin-bottom: 1rem;
        }
        h1 {
            font-size: 1.5rem;
            margin-bottom: 0.5rem;
        }
        p {
            color: #666;
            margin: 0 0 1rem 0;
        }
        .error-detail {
            background: #fee;
            color: #c33;
            padding: 0.5rem 1rem;
            border-radius: 4px;
            font-family: monospace;
            font-size: 0.9rem;
        }
    </style>
</head>
<body>
    <div class="message">
        <div class="error">✗</div>
        <h1>Authentication Failed</h1>
        <p>An error occurred during authentication.</p>
        <div class="error-detail">%s</div>
    </div>
</body>
</html>`, url.QueryEscape(errorMsg))
	
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)
	fmt.Fprint(w, html)
}