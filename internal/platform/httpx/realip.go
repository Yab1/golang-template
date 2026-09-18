package httpx

import (
	"net"
	"net/http"
	"strconv"
	"strings"
)

func RealIP(trusted []string) func(http.Handler) http.Handler {
	nets := parseCIDRs(trusted)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(nets) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			ip := parseRemoteIP(r.RemoteAddr)
			if ip == nil || !ipInNets(ip, nets) {
				next.ServeHTTP(w, r)
				return
			}

			client := forwardedClientIP(r)
			if client == nil {
				next.ServeHTTP(w, r)
				return
			}

			_, port, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil || port == "" {
				port = "0"
			}
			r.RemoteAddr = net.JoinHostPort(client.String(), port)
			next.ServeHTTP(w, r)
		})
	}
}

func parseCIDRs(values []string) []*net.IPNet {
	out := make([]*net.IPNet, 0, len(values))
	for _, raw := range values {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if !strings.Contains(raw, "/") {
			if ip := net.ParseIP(raw); ip != nil {
				bits := 32
				if ip.To4() == nil {
					bits = 128
				}
				raw = raw + "/" + strconv.Itoa(bits)
			}
		}
		_, n, err := net.ParseCIDR(raw)
		if err != nil {
			continue
		}
		out = append(out, n)
	}
	return out
}

func parseRemoteIP(addr string) net.IP {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	return net.ParseIP(host)
}

func ipInNets(ip net.IP, nets []*net.IPNet) bool {
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func forwardedClientIP(r *http.Request) net.IP {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		first := strings.TrimSpace(strings.Split(fwd, ",")[0])
		if ip := net.ParseIP(first); ip != nil {
			return ip
		}
	}
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		if ip := net.ParseIP(strings.TrimSpace(realIP)); ip != nil {
			return ip
		}
	}
	return nil
}
