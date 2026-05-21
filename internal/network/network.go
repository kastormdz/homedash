package network

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

var (
	// Dominios permitidos para el proxy de escudos/logos
	allowedDomains = []string{
		"upload.wikimedia.org",
		"cdn.register.f1.com",
		"espncdn.com",
		"a.espncdn.com",
		"static.promiedos.com.ar",
		"www.promiedos.com.ar",
		"api.coingecko.com",
		"api.binance.com",
		"query1.finance.yahoo.com",
		"dolarapi.com",
		"api.jolpi.ca",
		"site.api.espn.com",
		"www.inpres.gob.ar",
		"earthquake.usgs.gov",
		"ssl.smn.gob.ar",
		"www.contingencias.mendoza.gov.ar",
		"open-meteo.com",
		"air-quality-api.open-meteo.com",
		"geocoding-api.open-meteo.com",
	}

	safeDialer = &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
		// Control se ejecuta después de la resolución pero antes de la conexión (Prevenir SSRF)
		Control: func(network, address string, c syscall.RawConn) error {
			host, _, _ := net.SplitHostPort(address)
			ip := net.ParseIP(host)
			if ip != nil {
				if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() {
					return fmt.Errorf("destino interno bloqueado: %s", host)
				}
			}
			return nil
		},
	}

	// Cliente HTTP global con timeouts configurados y protección DNS Rebinding (Hardening)
	DefaultClient = &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, networkStr, addr string) (net.Conn, error) {
				// Usamos safeDialer que ya tiene el Control para validar IPs
				return safeDialer.DialContext(ctx, networkStr, addr)
			},
			MaxIdleConns:        100,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
			ForceAttemptHTTP2:   true,
		},
		CheckRedirect: validateRedirect,
	}
)

func validateRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 3 {
		return fmt.Errorf("demasiados redirects")
	}
	if isInternalRequest(req.URL) {
		return fmt.Errorf("redirect a destino interno bloqueado: %s", req.URL.Hostname())
	}
	return nil
}

func isInternalRequest(u *url.URL) bool {
	hostname := u.Hostname()
	if ip := net.ParseIP(hostname); ip != nil {
		return ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
	}
	ips, err := net.LookupIP(hostname)
	if err != nil {
		return false
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() {
			return true
		}
	}
	return false
}

func IsDomainAllowed(rawURL string) (bool, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false, err
	}
	if isInternalRequest(u) {
		return false, nil
	}
	hostname := strings.ToLower(u.Hostname())
	for _, domain := range allowedDomains {
		if hostname == domain || strings.HasSuffix(hostname, "."+domain) {
			return true, nil
		}
	}
	return false, nil
}

func FetchSecure(targetURL string) (*http.Response, error) {
	return FetchSecureWithContext(context.Background(), targetURL)
}

func FetchSecureWithContext(ctx context.Context, targetURL string) (*http.Response, error) {
	u, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}

	if isInternalRequest(u) {
		return nil, fmt.Errorf("destino bloqueado por ser una dirección interna: %s", u.Hostname())
	}

	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")

	resp, err := DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("error HTTP: %d", resp.StatusCode)
	}

	return resp, nil
}
