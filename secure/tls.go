package secure

import (
	"crypto/tls"
	"crypto/x509"
)

type TLS struct {
	cs *tls.ConnectionState
}

func (t *TLS) Config() *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: true,
		VerifyConnection:   t.VerifyConnection,
	}
}

func (t *TLS) VerifyConnection(cs tls.ConnectionState) error {
	t.cs = &cs
	return nil
}

func (t *TLS) Info() (secured, trusted bool, info map[string]string) {
	if secured = t.cs != nil; secured {
		info = map[string]string{
			"Version":     tls.VersionName(t.cs.Version),
			"CipherSuite": tls.CipherSuiteName(t.cs.CipherSuite),
			"ServerName":  t.cs.ServerName,
		}
		if len(t.cs.PeerCertificates) > 0 {
			cert := t.cs.PeerCertificates[0]
			info["Subject"] = cert.Subject.String()
			info["Issuer"] = cert.Issuer.String()

			info["NotBefore"] = cert.NotBefore.String()
			info["NotAfter"] = cert.NotAfter.String()

			opts := x509.VerifyOptions{
				DNSName:       t.cs.ServerName,
				Intermediates: x509.NewCertPool(),
			}
			for _, c := range t.cs.PeerCertificates[1:] {
				opts.Intermediates.AddCert(c)
			}
			_, err := cert.Verify(opts)
			trusted = (err == nil)
			if err != nil {
				info["VerificationError"] = err.Error()
			}
		}
	}

	return secured, trusted, info
}
