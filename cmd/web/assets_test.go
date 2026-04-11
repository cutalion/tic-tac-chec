package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResolveAnalyticsConfigDisabledByDefault(t *testing.T) {
	t.Setenv("ANALYTICS_ENABLED", "")
	t.Setenv("POSTHOG_KEY", "")
	t.Setenv("POSTHOG_HOST", "")

	got := resolveAnalyticsConfig()

	if got.Enabled {
		t.Fatal("expected analytics to be disabled")
	}
	if got.PostHogKey != "" {
		t.Fatalf("expected empty PostHog key, got %q", got.PostHogKey)
	}
	if got.PostHogHost != "" {
		t.Fatalf("expected empty PostHog host, got %q", got.PostHogHost)
	}
}

func TestResolveAnalyticsConfigRequiresKeyAndHost(t *testing.T) {
	t.Setenv("ANALYTICS_ENABLED", "true")
	t.Setenv("POSTHOG_KEY", "phc_test")
	t.Setenv("POSTHOG_HOST", "")

	got := resolveAnalyticsConfig()

	if got.Enabled {
		t.Fatal("expected analytics to stay disabled without a host")
	}
}

func TestResolveAnalyticsConfigEnabled(t *testing.T) {
	t.Setenv("ANALYTICS_ENABLED", "true")
	t.Setenv("POSTHOG_KEY", "phc_test")
	t.Setenv("POSTHOG_HOST", "https://eu.i.posthog.com")

	got := resolveAnalyticsConfig()

	if !got.Enabled {
		t.Fatal("expected analytics to be enabled")
	}
	if got.PostHogKey != "phc_test" {
		t.Fatalf("expected PostHog key %q, got %q", "phc_test", got.PostHogKey)
	}
	if got.PostHogHost != "https://eu.i.posthog.com" {
		t.Fatalf("expected PostHog host %q, got %q", "https://eu.i.posthog.com", got.PostHogHost)
	}
}

func TestWriteTemplatedAssetInjectsAnalyticsConfig(t *testing.T) {
	previousAnalyticsConfig := analyticsConfig
	t.Cleanup(func() {
		analyticsConfig = previousAnalyticsConfig
	})

	analyticsConfig = AnalyticsConfig{
		Enabled:     true,
		PostHogKey:  "phc_test",
		PostHogHost: "https://eu.i.posthog.com",
	}

	rr := httptest.NewRecorder()
	writeTemplatedAsset(
		rr,
		"text/html; charset=utf-8",
		[]byte(`enabled=__ANALYTICS_ENABLED__; key=__POSTHOG_KEY__; host=__POSTHOG_HOST__;`),
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
