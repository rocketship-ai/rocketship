package cli

import (
    "testing"
)

func TestParseExplicitAddress(t *testing.T) {
    tests := []struct{
        name       string
        input      string
        wantTarget string
        wantTLS    bool
        wantSNI    string
        wantErr    bool
    }{
        {name: "https default port", input: "https://example.com", wantTarget: "example.com:443", wantTLS: true, wantSNI: "example.com"},
        {name: "https explicit port", input: "https://example.com:443", wantTarget: "example.com:443", wantTLS: true, wantSNI: "example.com"},
        {name: "grpcs custom port", input: "grpcs://foo.bar:8443", wantTarget: "foo.bar:8443", wantTLS: true, wantSNI: "foo.bar"},
        {name: "http default port", input: "http://foo", wantTarget: "foo:7700", wantTLS: false},
        {name: "grpc explicit", input: "grpc://foo:7700", wantTarget: "foo:7700", wantTLS: false},
        {name: "bare host:port", input: "foo:1234", wantTarget: "foo:1234", wantTLS: false},
        {name: "localhost", input: "localhost:7700", wantTarget: "localhost:7700", wantTLS: false},
        {name: "bare host default port", input: "foo", wantTarget: "foo:7700", wantTLS: false},
        {name: "unsupported scheme", input: "tcp://host:1", wantErr: true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            target, useTLS, sni, err := parseExplicitAddress(tt.input)
            if tt.wantErr {
                if err == nil {
                    t.Fatalf("expected error, got nil")
                }
                return
            }
            if err != nil {
                t.Fatalf("unexpected error: %v", err)
            }
            if target != tt.wantTarget {
                t.Errorf("target = %q, want %q", target, tt.wantTarget)
            }
            if useTLS != tt.wantTLS {
                t.Errorf("useTLS = %v, want %v", useTLS, tt.wantTLS)
            }
            if tt.wantSNI != "" && sni != tt.wantSNI {
                t.Errorf("sni = %q, want %q", sni, tt.wantSNI)
            }
        })
    }
}

