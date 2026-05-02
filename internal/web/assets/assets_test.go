package assets

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"tic-tac-chec/internal/web/config"
)

func TestResolveAnalyticsConfigDisabledByDefault(t *testing.T) {
	t.Setenv("ANALYTICS_ENABLED", "")
	t.Setenv("POSTHOG_KEY", "")
	t.Setenv("POSTHOG_HOST", "")

	got, err := config.NewConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if got.Analytics.Enabled {
		t.Fatal("expected analytics to be disabled")
	}
	if got.Analytics.PostHog.Key != "" {
		t.Fatalf("expected empty PostHog key, got %q", got.Analytics.PostHog.Key)
	}
	if got.Analytics.PostHog.Host != "" {
		t.Fatalf("expected empty PostHog host, got %q", got.Analytics.PostHog.Host)
	}
}

func TestResolveAnalyticsConfigEnabled(t *testing.T) {
	t.Setenv("ANALYTICS_ENABLED", "true")
	t.Setenv("POSTHOG_KEY", "phc_test")
	t.Setenv("POSTHOG_HOST", "https://eu.i.posthog.com")

	got, err := config.NewConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if !got.Analytics.Enabled {
		t.Fatal("expected analytics to be enabled")
	}
	if got.Analytics.PostHog.Key != "phc_test" {
		t.Fatalf("expected PostHog key %q, got %q", "phc_test", got.Analytics.PostHog.Key)
	}
	if got.Analytics.PostHog.Host != "https://eu.i.posthog.com" {
		t.Fatalf("expected PostHog host %q, got %q", "https://eu.i.posthog.com", got.Analytics.PostHog.Host)
	}
}

func TestWriteTemplatedAssetInjectsAnalyticsConfig(t *testing.T) {
	config := config.Analytics{
		Enabled: true,
		PostHog: &config.PostHog{
			Key:  "phc_test",
			Host: "https://eu.i.posthog.com",
		},
	}

	rr := httptest.NewRecorder()
	WriteTemplatedAsset(
		rr,
		"text/html; charset=utf-8",
		[]byte(`enabled=__ANALYTICS_ENABLED__; key=__POSTHOG_KEY__; host=__POSTHOG_HOST__;`),
		config,
	)

	body := rr.Body.String()
	if !strings.Contains(body, `enabled=true`) {
		t.Fatalf("expected analytics enabled flag in body, got %q", body)
	}
	if !strings.Contains(body, `key="phc_test"`) {
		t.Fatalf("expected quoted PostHog key in body, got %q", body)
	}
	if !strings.Contains(body, `host="https://eu.i.posthog.com"`) {
		t.Fatalf("expected quoted PostHog host in body, got %q", body)
	}
}
