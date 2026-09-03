package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeJSONRejectsMalformedAndOversizedBodiesWithStructuredErrors(t *testing.T) {
	for _, test := range []struct {
		name       string
		body       string
		statusCode int
	}{
		{name: "trailing value", body: `{"name":"orca"} {"name":"again"}`, statusCode: http.StatusBadRequest},
		{name: "non object", body: `[]`, statusCode: http.StatusBadRequest},
		{name: "unknown field", body: `{"extra":true}`, statusCode: http.StatusBadRequest},
		{name: "oversized", body: `{"name":"` + strings.Repeat("x", maxRequestBodyBytes) + `"}`, statusCode: http.StatusRequestEntityTooLarge},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/request", strings.NewReader(test.body))
			var payload struct {
				Name string `json:"name"`
			}
			err := DecodeJSON(response, request, &payload)
			if err == nil {
				t.Fatal("DecodeJSON unexpectedly accepted request")
			}
			WriteRequestError(response, err, "invalid request", "Provide one request object.")
			if response.Code != test.statusCode {
				t.Fatalf("status = %d, want %d: %s", response.Code, test.statusCode, response.Body.String())
			}
			if !strings.Contains(response.Body.String(), `"type":"bad_request"`) || !strings.Contains(response.Body.String(), `"user_message"`) {
				t.Fatalf("unstructured error response: %s", response.Body.String())
			}
		})
	}
}
