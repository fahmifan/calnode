package server

import "testing"

func TestNegotiatedEncoding_prefersBrotliOnTie(t *testing.T) {
	if got, ok := negotiatedEncoding("gzip, br"); !ok || got != "br" {
		t.Fatalf("encoding = %q, ok=%v; want br", got, ok)
	}
}

func TestNegotiatedEncoding_honorsQuality(t *testing.T) {
	if got, ok := negotiatedEncoding("br;q=0.2, gzip;q=0.8"); !ok || got != "gzip" {
		t.Fatalf("encoding = %q, ok=%v; want gzip", got, ok)
	}
}

func TestNegotiatedEncoding_wildcardAndIdentityFallback(t *testing.T) {
	if got, ok := negotiatedEncoding("*;q=0.5"); !ok || got != "br" {
		t.Fatalf("encoding = %q, ok=%v; want br", got, ok)
	}
	if got, ok := negotiatedEncoding("br;q=0, gzip;q=0"); !ok || got != "" {
		t.Fatalf("encoding = %q, ok=%v; want identity", got, ok)
	}
}

func TestNegotiatedEncoding_rejectsWhenNoEncodingIsAccepted(t *testing.T) {
	if got, ok := negotiatedEncoding("br;q=0, gzip;q=0, identity;q=0"); ok || got != "" {
		t.Fatalf("encoding = %q, ok=%v; want no acceptable encoding", got, ok)
	}
}

func TestCompressibleAsset_acceptsSVG(t *testing.T) {
	if !compressibleAsset("image/svg+xml; charset=utf-8") {
		t.Fatal("SVG should be compressible")
	}
}
