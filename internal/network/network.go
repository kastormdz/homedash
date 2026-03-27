package network

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	// Dominios permitidos para el proxy de escudos/logos
	allowedDomains = []string{
		"upload.wikimedia.org",
		"cdn.register.f1.com",
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

	// Cliente HTTP global con timeouts configurados
	DefaultClient = &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
		CheckRedirect: validateRedirect,
	}
)

// validateRedirect bloquea redirects a IPs internas o dominios no allowlisted
func validateRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 3 {
		return fmt.Errorf("demasiados redirects")
	}
	// S-B. Re-validar dominio del redirect contra allowlist
	if !isInternalRequest(req.URL) {
		return nil
	}
	return fmt.Errorf("redirect a destino no permitido: %s", req.URL.Hostname())
}

// isInternalRequest verifica si una URL apunta a IPs internas (SSRF protection)
func isInternalRequest(u *url.URL) bool {
	hostname := u.Hostname()
	// S-A. Bloquear IPs directas internas
	if ip := net.ParseIP(hostname); ip != nil {
		return ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
	}
	// Intentar resolver DNS y verificar que no sea IP interna
	ips, err := net.LookupIP(hostname)
	if err != nil {
		return true // Si no se puede resolver, bloquear por seguridad
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() {
			return true
		}
	}
	return false
}

// IsDomainAllowed verifica si una URL pertenece a la lista blanca
func IsDomainAllowed(rawURL string) (bool, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false, err
	}

	// S-A. Primero verificar que no apunte a IP interna
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

// FetchSecure realiza una petición GET validando errores y status
func FetchSecure(targetURL string) (*http.Response, error) {
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

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
