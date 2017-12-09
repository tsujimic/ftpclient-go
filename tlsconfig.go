package ftpclient

import (
	"crypto/tls"
	"fmt"
	"os"
)

var clientCipherSuites = []uint16{
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
}

var acceptedCBCCiphers = []uint16{
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
	tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
	tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
	tls.TLS_RSA_WITH_AES_256_CBC_SHA,
	tls.TLS_RSA_WITH_AES_128_CBC_SHA,
}

var defaultServerAcceptedCiphers = append(clientCipherSuites, acceptedCBCCiphers...)

// NewTLSConfig ...
func NewTLSConfig() *tls.Config {
	return &tls.Config{
		// Avoid fallback to SSL protocols < TLS1.0
		MinVersion:               tls.VersionTLS10,
		PreferServerCipherSuites: true,
		CipherSuites:             defaultServerAcceptedCiphers,
		//CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
	}
}

// NewTLSConfigWithX509KeyPair ...
func NewTLSConfigWithX509KeyPair(certFile, keyFile string) (*tls.Config, error) {
	config := NewTLSConfig()
	//config.ClientAuth = tls.RequireAndVerifyClientCert
	tlsCertificate, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("could not load X509 key(cert: %q, key: %q): %v", certFile, keyFile, err)
		}

		return nil, fmt.Errorf("error load X509 key(cert: %q, key: %q): %v", certFile, keyFile, err)
	}

	config.Certificates = []tls.Certificate{tlsCertificate}
	//config.Certificates = make([]tls.Certificate, 1)
	//config.Certificates[0], err = tls.LoadX509KeyPair(certFile, keyFile)
	return config, nil
}
