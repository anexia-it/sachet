package anexia

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/messagebird/sachet"
)

const defaultEndpoint = "https://engine.anexia-it.com/api/sms/v1/message.json"

// Config is the configuration struct for the Anexia provider.
type Config struct {
	URL            string `yaml:"url"`
	APIKey         string `yaml:"api_key"`
	Encoding       string `yaml:"encoding"`
	Sender         string `yaml:"sender"`
	DeliveryReport bool   `yaml:"delivery_report"`
}

var _ (sachet.Provider) = (*Anexia)(nil)

// Anexia contains the necessary values for the Anexia SMS provider.
type Anexia struct {
	URL            string
	APIKey         string
	Encoding       string
	Sender         string
	DeliveryReport bool
	HTTPClient     *http.Client
}

// NewAnexia creates and returns a new Anexia SMS provider.
func NewAnexia(config Config) *Anexia {
	if config.URL == "" {
		config.URL = defaultEndpoint
	}
	if config.Encoding == "" {
		config.Encoding = "GSM"
	}

	return &Anexia{
		URL:            config.URL,
		APIKey:         config.APIKey,
		Encoding:       strings.ToUpper(config.Encoding),
		Sender:         config.Sender,
		DeliveryReport: config.DeliveryReport,
		HTTPClient:     &http.Client{Timeout: 20 * time.Second},
	}
}

type requestPayload struct {
	Destination    string `json:"destination"`
	Message        string `json:"message"`
	Encoding       string `json:"encoding"`
	Sender         string `json:"sender,omitempty"`
	DeliveryReport bool   `json:"deliveryReport,omitempty"`
}

// Send sends a message using the Anexia SMS API.
func (a *Anexia) Send(message sachet.Message) error {
	if message.Type != "" && message.Type != "text" {
		return fmt.Errorf("unknown message type %s", message.Type)
	}
	if len(message.To) == 0 {
		return fmt.Errorf("no destination addresses provided")
	}

	encoding := strings.ToUpper(strings.TrimSpace(a.Encoding))
	switch encoding {
	case "", "GSM", "BINARY", "UCS2", "AUTO":
		// valid, default to GSM when empty
		slot := ""
		if encoding == "" {
			slot = "GSM"
		} else {
			slot = encoding
		}
		encoding = slot
	default:
		return fmt.Errorf("unsupported encoding %q", a.Encoding)
	}

	for _, recipient := range message.To {
		recipient = strings.TrimSpace(recipient)
		if recipient == "" {
			return fmt.Errorf("empty destination in message")
		}

		payload := requestPayload{
			Destination:    recipient,
			Message:        message.Text,
			Encoding:       encoding,
			DeliveryReport: a.DeliveryReport,
		}
		if message.From != "" {
			payload.Sender = message.From
		} else if a.Sender != "" {
			payload.Sender = a.Sender
		}

		body, err := json.Marshal(payload)
		if err != nil {
			return err
		}

		requestURL, err := url.Parse(a.URL)
		if err != nil {
			return err
		}
		query := requestURL.Query()
		if a.APIKey != "" {
			query.Set("api_key", a.APIKey)
		}
		requestURL.RawQuery = query.Encode()

		request, err := http.NewRequest(http.MethodPost, requestURL.String(), bytes.NewReader(body))
		if err != nil {
			return err
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Accept", "application/json")

		response, err := a.HTTPClient.Do(request)
		if err != nil {
			return err
		}
		responseBody, readErr := io.ReadAll(response.Body)
		response.Body.Close()
		if readErr != nil {
			return readErr
		}
		if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
			if len(responseBody) > 0 {
				return fmt.Errorf("anexia SMS send failed (status %d): %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
			}
			return fmt.Errorf("anexia SMS send failed (status %d)", response.StatusCode)
		}
	}

	return nil
}
