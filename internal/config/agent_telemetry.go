package config

import (
	"fmt"
	"math"
	"net"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

type AgentTelemetry struct {
	Prices AgentPriceTable      `yaml:"prices"`
	Export AgentTelemetryExport `yaml:"export"`
}

type AgentTelemetryExport struct {
	Endpoint  string `yaml:"endpoint"`
	BearerEnv string `yaml:"bearer_env"`
}

func (value AgentTelemetryExport) Validate() error {
	if value.Endpoint == "" && value.BearerEnv == "" {
		return nil
	}
	if err := validateExportEndpoint(value.Endpoint); err != nil {
		return fmt.Errorf("agent telemetry export: %w", err)
	}
	u, err := url.Parse(value.Endpoint)
	if err != nil || u.ForceQuery || strings.Contains(value.Endpoint, "#") || strings.TrimSpace(value.Endpoint) != value.Endpoint || strings.IndexFunc(value.Endpoint, unicode.IsControl) >= 0 {
		return fmt.Errorf("agent telemetry export requires a URL without query, fragment or control characters")
	}
	if value.BearerEnv != "" && !exportEnvName.MatchString(value.BearerEnv) {
		return fmt.Errorf("agent telemetry export bearer_env must name an explicit environment variable")
	}
	if u.Port() != "" {
		port, err := strconv.Atoi(u.Port())
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("agent telemetry export port must be between 1 and 65535")
		}
	}
	return nil
}

var exportEnvName = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,63}$`)

// validateExportEndpoint admits HTTPS, or HTTP only to a literal loopback address.
func validateExportEndpoint(endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil || len(endpoint) > 2048 || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return fmt.Errorf("endpoint must be an absolute HTTP URL without credentials, query or fragment")
	}
	ip := net.ParseIP(u.Hostname())
	if u.Scheme != "https" && !(u.Scheme == "http" && ip != nil && ip.IsLoopback()) {
		return fmt.Errorf("endpoint requires HTTPS, except literal loopback HTTP")
	}
	return nil
}

type AgentPriceTable struct {
	Version  string            `yaml:"version"`
	Currency string            `yaml:"currency"`
	Rates    []AgentModelPrice `yaml:"rates"`
}

// Pointer rates distinguish an explicit zero price from missing configuration.
type AgentModelPrice struct {
	Provider         string   `yaml:"provider"`
	Model            string   `yaml:"model"`
	InputPerMillion  *float64 `yaml:"input_per_million"`
	OutputPerMillion *float64 `yaml:"output_per_million"`
	CachedPerMillion *float64 `yaml:"cached_per_million"`
}

var priceVersion = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)
var priceCurrency = regexp.MustCompile(`^[A-Z]{3}$`)

func ResolveAgentTelemetry(value AgentTelemetry) (AgentTelemetry, error) {
	if err := value.Export.Validate(); err != nil {
		return value, err
	}
	table := &value.Prices
	if table.Version == "" && table.Currency == "" && len(table.Rates) == 0 {
		return value, nil
	}
	if !priceVersion.MatchString(table.Version) || !priceCurrency.MatchString(table.Currency) || len(table.Rates) == 0 || len(table.Rates) > 64 {
		return value, fmt.Errorf("agent_runtime.telemetry.prices requires a version, three-letter currency and 1–64 rates")
	}
	table.Rates = slices.Clone(table.Rates)
	seen := map[[2]string]bool{}
	for i, rate := range table.Rates {
		key := [2]string{rate.Provider, rate.Model}
		if seen[key] || !priceIdentity(rate.Provider) || !priceIdentity(rate.Model) {
			return value, fmt.Errorf("agent price identities must be nonempty, bounded and unique")
		}
		seen[key] = true
		copy, err := resolveModelPrice(rate)
		if err != nil {
			return value, err
		}
		table.Rates[i] = copy
	}
	return value, nil
}

func priceIdentity(value string) bool {
	return strings.TrimSpace(value) == value && value != "" && len(value) <= 256 && strings.IndexFunc(value, unicode.IsControl) < 0
}

func resolveModelPrice(rate AgentModelPrice) (AgentModelPrice, error) {
	for _, field := range []**float64{&rate.InputPerMillion, &rate.OutputPerMillion, &rate.CachedPerMillion} {
		if *field == nil || math.IsNaN(**field) || math.IsInf(**field, 0) || **field < 0 || **field > 1e6 {
			return rate, fmt.Errorf("agent prices require explicit finite input, output and cached rates from 0 to 1000000 per million tokens")
		}
		value := **field
		*field = &value
	}
	return rate, nil
}
