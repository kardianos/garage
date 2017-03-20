package comm

import (
	"crypto/tls"
)

func Port() int {
	return port
}

func Host() string {
	return host
}

func CA() []byte {
	return []byte(ca)
}

func Cert() []byte {
	return []byte(cert)
}

func Key() []byte {
	return []byte(key)
}

func AuthKey() string {
	return authKey
}

type Request struct {
	Auth string
	Type string
}

type Response struct {
	OK      bool
	Message string
}

const (
	RequestPing   = "ping"
	RequestToggle = "toggle"
	RequestClose  = "close"
)

var (
	Ciphers = []uint16{
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,

		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	}
)
