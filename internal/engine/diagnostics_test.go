package engine

import (
	"context"
	"encoding/binary"
	"net"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/Nciae-Zyh/stundeck/internal/store"
)

func TestParseSTUNBindingResponseIPv4(t *testing.T) {
	transactionID := []byte("abcdefghijkl")
	response := make([]byte, 32)
	binary.BigEndian.PutUint16(response[0:2], 0x0101)
	binary.BigEndian.PutUint16(response[2:4], 12)
	binary.BigEndian.PutUint32(response[4:8], 0x2112A442)
	copy(response[8:20], transactionID)
	binary.BigEndian.PutUint16(response[20:22], 0x0020)
	binary.BigEndian.PutUint16(response[22:24], 8)
	response[25] = 0x01

	family, err := parseSTUNBindingResponse(response, transactionID)
	if err != nil {
		t.Fatal(err)
	}
	if family != "IPv4" {
		t.Fatalf("family = %q", family)
	}
}

func TestParseSTUNBindingResponseRejectsTransactionMismatch(t *testing.T) {
	response := make([]byte, 20)
	binary.BigEndian.PutUint16(response[0:2], 0x0101)
	binary.BigEndian.PutUint32(response[4:8], 0x2112A442)
	copy(response[8:20], []byte("abcdefghijkl"))

	if _, err := parseSTUNBindingResponse(response, []byte("mnopqrstuvwx")); err == nil {
		t.Fatal("transaction mismatch was accepted")
	}
}

func TestParseSTUNResponseIncludesMappedAndOtherAddresses(t *testing.T) {
	transactionID := []byte("abcdefghijkl")
	response := make([]byte, 44)
	binary.BigEndian.PutUint16(response[0:2], 0x0101)
	binary.BigEndian.PutUint16(response[2:4], 24)
	binary.BigEndian.PutUint32(response[4:8], 0x2112A442)
	copy(response[8:20], transactionID)

	binary.BigEndian.PutUint16(response[20:22], 0x0020)
	binary.BigEndian.PutUint16(response[22:24], 8)
	response[25] = 0x01
	binary.BigEndian.PutUint16(response[26:28], uint16(45678)^uint16(0x2112))
	mapped := net.ParseIP("203.0.113.9").To4()
	magic := []byte{0x21, 0x12, 0xa4, 0x42}
	for index := range mapped {
		response[28+index] = mapped[index] ^ magic[index]
	}

	binary.BigEndian.PutUint16(response[32:34], 0x802c)
	binary.BigEndian.PutUint16(response[34:36], 8)
	response[37] = 0x01
	binary.BigEndian.PutUint16(response[38:40], 3479)
	copy(response[40:44], net.ParseIP("198.51.100.7").To4())

	result, err := parseSTUNResponse(response, transactionID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Family != "IPv4" || result.Mapped.String() != "203.0.113.9:45678" {
		t.Fatalf("mapped = %#v", result.Mapped)
	}
	if result.Other.String() != "198.51.100.7:3479" {
		t.Fatalf("other = %#v", result.Other)
	}
}

func TestClassifyNATMapping(t *testing.T) {
	address := func(ip string, port int) *net.UDPAddr {
		return &net.UDPAddr{IP: net.ParseIP(ip), Port: port}
	}
	local := address("192.0.2.10", 30000)
	tests := []struct {
		name                 string
		first, second, third *net.UDPAddr
		want                 string
	}{
		{name: "open internet", first: local, second: local, third: local, want: "open_internet"},
		{name: "endpoint independent", first: address("203.0.113.2", 40000), second: address("203.0.113.2", 40000), third: address("203.0.113.2", 40000), want: "endpoint_independent"},
		{name: "address dependent", first: address("203.0.113.2", 40000), second: address("203.0.113.2", 40001), third: address("203.0.113.2", 40001), want: "address_dependent"},
		{name: "address and port dependent", first: address("203.0.113.2", 40000), second: address("203.0.113.2", 40001), third: address("203.0.113.2", 40002), want: "address_port_dependent"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := classifyNATMapping(local, test.first, test.second, test.third); got != test.want {
				t.Fatalf("classification = %q, want %q", got, test.want)
			}
		})
	}
}

func TestDiagnosticOutcome(t *testing.T) {
	if got := diagnosticOutcome([]DiagnosticCheck{{Status: "pass"}, {Status: "info"}}); got != "pass" {
		t.Fatalf("outcome = %q", got)
	}
	if got := diagnosticOutcome([]DiagnosticCheck{{Status: "warn"}}); got != "warning" {
		t.Fatalf("outcome = %q", got)
	}
	if got := diagnosticOutcome([]DiagnosticCheck{{Status: "warn"}, {Status: "fail"}}); got != "fail" {
		t.Fatalf("outcome = %q", got)
	}
}

func TestCheckTargetProtocolDetectsHTTPSOnHTTPService(t *testing.T) {
	tlsServer := httptest.NewTLSServer(nil)
	defer tlsServer.Close()
	host, portText, err := net.SplitHostPort(strings.TrimPrefix(tlsServer.URL, "https://"))
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}

	status, message := checkTargetProtocol(context.Background(), store.Service{
		TargetHost:  host,
		TargetPort:  port,
		Protocol:    "tcp",
		PublishMode: "redirect",
		Scheme:      "http",
	})
	if status != "fail" || !strings.Contains(message, "实际接受 HTTPS") {
		t.Fatalf("status = %q, message = %q", status, message)
	}
}

func TestCheckProxyEnvironmentReportsConfiguredProxy(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://proxy.invalid:7890")
	status, message := checkProxyEnvironment(context.Background())
	if status != "fail" || !strings.Contains(message, "HTTP_PROXY") {
		t.Fatalf("status = %q, message = %q", status, message)
	}
}
