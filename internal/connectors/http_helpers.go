package connectors

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

func applyHTTPHeaders(request *http.Request, spec Spec) error {
	for k, v := range spec.Headers {
		request.Header.Set(k, v)
	}
	for k, e := range spec.HeaderEnv {
		e = strings.TrimSpace(e)
		v := strings.TrimSpace(os.Getenv(e))
		if v == "" {
			return fmt.Errorf("connector header %q environment %q is empty", k, e)
		}
		request.Header.Set(k, v)
	}
	if e := strings.TrimSpace(spec.BearerTokenEnv); e != "" {
		v := strings.TrimSpace(os.Getenv(e))
		if v == "" {
			return fmt.Errorf("connector bearer token environment %q is empty", e)
		}
		request.Header.Set("Authorization", "Bearer "+v)
	}
	return nil
}
func requireSecretEnvironment(spec Spec) error {
	for h, e := range spec.HeaderEnv {
		if strings.TrimSpace(os.Getenv(strings.TrimSpace(e))) == "" {
			return fmt.Errorf("connector header %q environment %q is empty", h, e)
		}
	}
	if e := strings.TrimSpace(spec.BearerTokenEnv); e != "" && strings.TrimSpace(os.Getenv(e)) == "" {
		return fmt.Errorf("connector bearer token environment %q is empty", e)
	}
	if e := strings.TrimSpace(spec.APIKeyEnv); e != "" && strings.TrimSpace(os.Getenv(e)) == "" {
		return fmt.Errorf("connector API key environment %q is empty", e)
	}
	return nil
}
func readinessGET(ctx context.Context, client *http.Client, spec Spec) error {
	if strings.TrimSpace(spec.HealthEndpoint) == "" {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, spec.HealthEndpoint, nil)
	if err != nil {
		return err
	}
	if err := applyHTTPHeaders(req, spec); err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 16*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &DeliveryError{Err: fmt.Errorf("connector health endpoint returned %s", resp.Status), Retryable: retryableStatus(resp.StatusCode), StatusCode: resp.StatusCode}
	}
	return nil
}
func classifyHTTPStatus(code int, status string) error {
	if code >= 200 && code < 300 {
		return nil
	}
	return &DeliveryError{Err: fmt.Errorf("connector endpoint returned %s", status), Retryable: retryableStatus(code), StatusCode: code}
}
func retryableStatus(status int) bool {
	if status == http.StatusRequestTimeout || status == http.StatusTooEarly || status == http.StatusTooManyRequests {
		return true
	}
	return status >= 500
}
