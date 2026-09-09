package stream

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"
)

// KafkaSecurityConfig configures encrypted/authenticated broker connections.
//
// Empty values preserve the local plaintext Kafka development profile.
type KafkaSecurityConfig struct {
	TLS           bool
	CAFile        string
	CertFile      string
	KeyFile       string
	SASLMechanism string
	Username      string
	Password      string
}

// KafkaSecurityFromEnv reads the standard TelemetryForge Kafka security
// variables. It is shared by gateway, worker, and telemetryctl so all broker
// clients use the same TLS/SASL contract.
func KafkaSecurityFromEnv() KafkaSecurityConfig {
	return KafkaSecurityConfig{
		TLS:           envBool("TELEMETRYFORGE_KAFKA_TLS"),
		CAFile:        strings.TrimSpace(os.Getenv("TELEMETRYFORGE_KAFKA_CA_FILE")),
		CertFile:      strings.TrimSpace(os.Getenv("TELEMETRYFORGE_KAFKA_CERT_FILE")),
		KeyFile:       strings.TrimSpace(os.Getenv("TELEMETRYFORGE_KAFKA_KEY_FILE")),
		SASLMechanism: strings.TrimSpace(os.Getenv("TELEMETRYFORGE_KAFKA_SASL_MECHANISM")),
		Username:      strings.TrimSpace(os.Getenv("TELEMETRYFORGE_KAFKA_SASL_USERNAME")),
		Password:      os.Getenv("TELEMETRYFORGE_KAFKA_SASL_PASSWORD"),
	}
}

func envBool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func kafkaSecurityOptions(config KafkaSecurityConfig) ([]kgo.Opt, error) {
	options := make([]kgo.Opt, 0, 2)

	if config.TLS {
		tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}

		if caFile := strings.TrimSpace(config.CAFile); caFile != "" {
			pem, err := os.ReadFile(caFile)
			if err != nil {
				return nil, fmt.Errorf("read Kafka CA file: %w", err)
			}
			roots, err := x509.SystemCertPool()
			if err != nil || roots == nil {
				roots = x509.NewCertPool()
			}
			if !roots.AppendCertsFromPEM(pem) {
				return nil, errors.New("Kafka CA file contains no parseable certificates")
			}
			tlsConfig.RootCAs = roots
		}

		certFile := strings.TrimSpace(config.CertFile)
		keyFile := strings.TrimSpace(config.KeyFile)
		if (certFile == "") != (keyFile == "") {
			return nil, errors.New("Kafka client certificate and key must be configured together")
		}
		if certFile != "" {
			certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
			if err != nil {
				return nil, fmt.Errorf("load Kafka client certificate: %w", err)
			}
			tlsConfig.Certificates = []tls.Certificate{certificate}
		}

		options = append(options, kgo.DialTLSConfig(tlsConfig))
	}

	mechanism := strings.ToLower(strings.TrimSpace(config.SASLMechanism))
	if mechanism == "" || mechanism == "disabled" {
		return options, nil
	}
	username := strings.TrimSpace(config.Username)
	if username == "" || config.Password == "" {
		return nil, errors.New("Kafka SASL username and password are required")
	}

	switch mechanism {
	case "plain":
		options = append(options, kgo.SASL(plain.Auth{
			User: username,
			Pass: config.Password,
		}.AsMechanism()))
	case "scram-sha-256":
		options = append(options, kgo.SASL(scram.Auth{
			User: username,
			Pass: config.Password,
		}.AsSha256Mechanism()))
	case "scram-sha-512":
		options = append(options, kgo.SASL(scram.Auth{
			User: username,
			Pass: config.Password,
		}.AsSha512Mechanism()))
	default:
		return nil, fmt.Errorf("unsupported Kafka SASL mechanism %q", mechanism)
	}

	return options, nil
}
