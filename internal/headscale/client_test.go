package headscale

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	hsrpc "github.com/UruhaLushia/headdeck/internal/headscale/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestDiagnosticReadIsBoundedAndDoesNotForwardCredentials(t *testing.T) {
	c := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, header := range []string{"Authorization", "Cookie", "X-Forwarded-For"} {
			if r.Header.Get(header) != "" {
				t.Errorf("diagnostic request contains %s", header)
			}
		}
		switch r.URL.Path {
		case "/metrics":
			_, _ = w.Write([]byte("go_goroutines 42\n"))
		case "/large":
			_, _ = w.Write([]byte(strings.Repeat("x", maxResponseBytes+1)))
		case "/redirect":
			http.Redirect(w, r, "/metrics", http.StatusFound)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("sensitive upstream error body"))
		}
	}))
	defer c.Close()
	data, err := c.ReadDiagnostic(context.Background(), c.BaseURL(), "/metrics")
	if err != nil || string(data) != "go_goroutines 42\n" {
		t.Fatal("metrics read failed", err)
	}
	for _, path := range []string{"/large", "/redirect", "/missing"} {
		_, err := c.ReadDiagnostic(context.Background(), c.BaseURL(), path)
		if err == nil || strings.Contains(err.Error(), "sensitive") {
			t.Fatal("unsafe diagnostic response", path, err)
		}
	}
}

func testClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := New(Options{
		BaseURL:    srv.URL,
		APIKey:     "hskey-api-testkey-secret",
		Timeout:    3 * time.Second,
		MaxRetries: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestUnauthorizedPlainTextBody(t *testing.T) {
	c := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		if _, err := w.Write([]byte("Unauthorized")); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	_, err := c.ListUsers(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %v", err)
	}
	if !apiErr.Unauthorized() {
		t.Fatal("401 must be reported as unauthorized")
	}
	if apiErr.Retryable() {
		t.Fatal("401 must not be retryable")
	}
}

func TestAPIRejectsRedirects(t *testing.T) {
	var reached atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()
	c := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	if err := c.DeleteNode(context.Background(), "1"); err == nil || reached.Load() != 0 {
		t.Fatal("API mutation followed a redirect", err, reached.Load())
	}
}

func TestNotFoundShapes(t *testing.T) {
	t.Run("legacy grpc status", func(t *testing.T) {
		c := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			if _, err := w.Write([]byte(`{"code":5,"message":"node not found","details":[]}`)); err != nil {
				t.Errorf("write response: %v", err)
			}
		}))
		_, err := c.GetNode(context.Background(), "9")
		var apiErr *APIError
		if !errors.As(err, &apiErr) || !apiErr.NotFound() {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("legacy 500 with grpc code 2", func(t *testing.T) {
		// Headscale <= 0.29 maps unmapped domain errors to 500 + code 2.
		c := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			if _, err := w.Write([]byte(`{"code":2,"message":"record not found","details":[]}`)); err != nil {
				t.Errorf("write response: %v", err)
			}
		}))
		_, err := c.RenameNode(context.Background(), "9", "newname")
		var apiErr *APIError
		if !errors.As(err, &apiErr) || !apiErr.NotFound() {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("rfc7807", func(t *testing.T) {
		c := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			if _, err := w.Write([]byte(`{"title":"Not Found","status":404,"detail":"node not found"}`)); err != nil {
				t.Errorf("write response: %v", err)
			}
		}))
		_, err := c.GetNode(context.Background(), "9")
		var apiErr *APIError
		if !errors.As(err, &apiErr) || !apiErr.NotFound() {
			t.Fatalf("error = %v", err)
		}
		if !apiErr.RFC7807 {
			t.Fatal("RFC7807 responses must be recognized")
		}
	})
}

func TestGetsRetryButMutationsDoNot(t *testing.T) {
	var getCalls, deleteCalls atomic.Int32
	c := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if getCalls.Add(1) < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			if _, err := w.Write([]byte(`{"nodes":[]}`)); err != nil {
				t.Errorf("write response: %v", err)
			}
		default:
			deleteCalls.Add(1)
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	if _, err := c.ListNodes(context.Background(), ""); err != nil {
		t.Fatalf("a retryable GET should eventually succeed: %v", err)
	}
	if getCalls.Load() != 3 {
		t.Fatalf("get calls = %d, want 3", getCalls.Load())
	}
	if err := c.DeleteNode(context.Background(), "1"); err == nil {
		t.Fatal("a failing delete must report an error")
	}
	if deleteCalls.Load() != 1 {
		t.Fatalf("delete calls = %d, want exactly 1: mutations must not be retried blindly",
			deleteCalls.Load())
	}
}

func TestUnixManagementWithoutAPIKey(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "hs.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	var reads, writes, httpCalls atomic.Int32
	server := grpc.NewServer(grpc.UnknownServiceHandler(func(_ any, stream grpc.ServerStream) error {
		method, _ := grpc.MethodFromServerStream(stream)
		switch method {
		case "/headscale.v1.HeadscaleService/ListUsers":
			if err := stream.RecvMsg(&hsrpc.ListUsersRequest{}); err != nil {
				return err
			}
			if reads.Add(1) < 3 {
				return status.Error(codes.Unavailable, "temporarily unavailable")
			}
			return stream.SendMsg(&hsrpc.ListUsersResponse{Users: []*hsrpc.User{{Id: 7, Name: "alice", ProviderId: "https://control.example/oidc/account-id", Provider: "oidc"}}})
		case "/headscale.v1.HeadscaleService/RenameNode":
			var in hsrpc.RenameNodeRequest
			if err := stream.RecvMsg(&in); err != nil {
				return err
			}
			if in.NodeId != 42 || in.NewName != "new-name" {
				return status.Error(codes.InvalidArgument, "wrong node request")
			}
			return stream.SendMsg(&hsrpc.RenameNodeResponse{Node: &hsrpc.Node{Id: 42, GivenName: in.NewName, Tags: []string{"tag:server"}}})
		case "/headscale.v1.HeadscaleService/RegisterNode":
			var in hsrpc.RegisterNodeRequest
			if err := stream.RecvMsg(&in); err != nil {
				return err
			}
			if in.User != "alice" || in.Key != "hskey-authreq-test" {
				return status.Error(codes.InvalidArgument, "wrong registration request")
			}
			return stream.SendMsg(&hsrpc.RegisterNodeResponse{Node: &hsrpc.Node{Id: 43, User: &hsrpc.User{Id: 7, Name: "alice"}}})
		case "/headscale.v1.HeadscaleService/DeleteNode":
			if err := stream.RecvMsg(&hsrpc.DeleteNodeRequest{}); err != nil {
				return err
			}
			writes.Add(1)
			return status.Error(codes.Unavailable, "response lost")
		case "/headscale.v1.HeadscaleService/GetNode":
			if err := stream.RecvMsg(&hsrpc.GetNodeRequest{}); err != nil {
				return err
			}
			return status.Error(codes.NotFound, "node not found")
		case "/headscale.v1.HeadscaleService/Health":
			if err := stream.RecvMsg(&hsrpc.HealthRequest{}); err != nil {
				return err
			}
			return stream.SendMsg(&hsrpc.HealthResponse{DatabaseConnectivity: true})
		default:
			return status.Error(codes.Unimplemented, "unexpected management call")
		}
	}))
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpCalls.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer httpServer.Close()
	c, err := New(Options{UnixSocket: socket, BaseURL: httpServer.URL, Timeout: 3 * time.Second, MaxRetries: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx := context.Background()
	users, err := c.ListUsers(ctx)
	if err != nil || len(users) != 1 || users[0].ID != "7" || users[0].ProviderID != "https://control.example/oidc/account-id" || reads.Load() != 3 {
		t.Fatal("Unix read or identity conversion failed", err)
	}
	node, err := c.RenameNode(ctx, "42", "new-name")
	if err != nil || node.ID != "42" || node.GivenName != "new-name" || len(node.Tags) != 1 {
		t.Fatal("Unix mutation failed", err)
	}
	if err := c.DeleteNode(ctx, "42"); err == nil || writes.Load() != 1 {
		t.Fatal("Unix mutation was replayed after an uncertain outcome", err)
	}
	registered, err := c.RegisterNode(ctx, "alice", "hskey-authreq-test")
	if err != nil || registered.ID != "43" || registered.User == nil || registered.User.ID != "7" {
		t.Fatal("Unix registration or owner conversion failed", err)
	}
	_, err = c.GetNode(ctx, "99")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || !apiErr.NotFound() {
		t.Fatal("Unix not-found error was lost", err)
	}
	if err := c.Health(ctx); err != nil {
		t.Fatal("Unix health failed", err)
	}
	if _, err := c.APIKeyInfo(ctx); err != nil || httpCalls.Load() != 0 {
		t.Fatal("Unix management fell back to HTTP or required an API key", err)
	}
}
