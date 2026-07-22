package runbook

import "testing"

func TestLookupKnownServiceNormalizesName(t *testing.T) {
	got, err := Lookup(Args{Service: " Payment-API ", Symptom: "latency"})
	if err != nil {
		t.Fatalf("Lookup returned error: %v", err)
	}

	if got.Service != "payment-api" {
		t.Fatalf("Service = %q, want payment-api", got.Service)
	}
	if got.Owner != "payments-oncall" {
		t.Fatalf("Owner = %q, want payments-oncall", got.Owner)
	}
	if got.Risk != "high" {
		t.Fatalf("Risk = %q, want high for latency symptom", got.Risk)
	}
	if len(got.Diagnostics) == 0 {
		t.Fatal("Diagnostics is empty")
	}
}

func TestLookupUnknownService(t *testing.T) {
	_, err := Lookup(Args{Service: "missing-service"})
	if err == nil {
		t.Fatal("Lookup returned nil error for unknown service")
	}
}

func TestLookupGatewayRunbook(t *testing.T) {
	got, err := Lookup(Args{Service: "gateway"})
	if err != nil {
		t.Fatalf("Lookup returned error: %v", err)
	}

	if got.Owner != "platform-oncall" {
		t.Fatalf("Owner = %q, want platform-oncall", got.Owner)
	}
	if len(got.Escalation) == 0 {
		t.Fatal("Escalation is empty")
	}
}
