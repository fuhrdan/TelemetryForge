package stream

import "testing"

func TestKafkaSecurityRejectsIncompleteClientCertificate(t *testing.T) {
	_, err := kafkaSecurityOptions(KafkaSecurityConfig{
		TLS:      true,
		CertFile: "client.crt",
	})
	if err == nil {
		t.Fatal("expected certificate without key to fail")
	}
}

func TestKafkaSecurityRejectsUnknownSASLMechanism(t *testing.T) {
	_, err := kafkaSecurityOptions(KafkaSecurityConfig{
		SASLMechanism: "magic",
		Username:      "user",
		Password:      "secret",
	})
	if err == nil {
		t.Fatal("expected unsupported SASL mechanism to fail")
	}
}

func TestKafkaSecurityRequiresSASLCredentials(t *testing.T) {
	_, err := kafkaSecurityOptions(KafkaSecurityConfig{
		SASLMechanism: "scram-sha-256",
	})
	if err == nil {
		t.Fatal("expected missing SASL credentials to fail")
	}
}
