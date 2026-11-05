package anexia

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/messagebird/sachet"
)

func TestSendUsesAnexiaPayload(t *testing.T) {
	var mu sync.Mutex
	requests := make([]struct {
		Destination    string `json:"destination"`
		Message        string `json:"message"`
		Encoding       string `json:"encoding"`
		Sender         string `json:"sender,omitempty"`
		DeliveryReport bool   `json:"deliveryReport,omitempty"`
	}, 0, 2)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if got := r.URL.Query().Get("api_key"); got != "secret" {
			t.Fatalf("unexpected api_key query param: %q", got)
		}

		var body struct {
			Destination    string `json:"destination"`
			Message        string `json:"message"`
			Encoding       string `json:"encoding"`
			Sender         string `json:"sender,omitempty"`
			DeliveryReport bool   `json:"deliveryReport,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		mu.Lock()
		requests = append(requests, body)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	provider := NewAnexia(Config{
		URL:            srv.URL,
		APIKey:         "secret",
		Encoding:       "AUTO",
		Sender:         "TestSender",
		DeliveryReport: true,
	})

	if err := provider.Send(sachet.Message{
		To:   []string{"+4366412345", "+4366412346"},
		From: "TestSender",
		Text: "hello world",
		Type: "text",
	}); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	if len(requests) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(requests))
	}

	if requests[0].Destination != "+4366412345" || requests[0].Message != "hello world" || requests[0].Encoding != "AUTO" {
		t.Fatalf("unexpected first request payload: %#v", requests[0])
	}
	if requests[1].Destination != "+4366412346" || requests[1].Message != "hello world" || requests[1].Encoding != "AUTO" {
		t.Fatalf("unexpected second request payload: %#v", requests[1])
	}
	for _, req := range requests {
		if req.Sender != "TestSender" || !req.DeliveryReport {
			t.Fatalf("unexpected sender/delivery settings: %#v", req)
		}
	}
}
