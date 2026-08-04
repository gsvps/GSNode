package probe

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

var mailProviderDomains = []struct{ Name, Domain string }{
	{"Gmail", "gmail.com"},
	{"Outlook", "outlook.com"},
	{"Yahoo", "yahoo.com"},
	{"Apple", "me.com"},
	{"QQ", "qq.com"},
	{"MailRU", "mail.ru"},
	{"AOL", "aol.com"},
	{"GMX", "gmx.com"},
	{"MailCOM", "mail.com"},
	{"163", "163.com"},
	{"Sohu", "sohu.com"},
	{"Sina", "sina.com"},
}

func checkLocalPort25(ctx context.Context) (label string, outbound bool) {
	d := net.Dialer{Timeout: 4 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", "smtp.mailgun.org:25")
	if err != nil {
		return "阻断", false
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(4 * time.Second))
	line, _ := bufio.NewReader(conn).ReadString('\n')
	if strings.HasPrefix(line, "220") {
		return "可用", true
	}
	return "远端不可达", false
}

func checkMailProviders(ctx context.Context) (anyOK bool, rows []map[string]any) {
	type result struct {
		name, status string
		ok           bool
	}
	ch := make(chan result, len(mailProviderDomains))
	for _, p := range mailProviderDomains {
		p := p
		go func() {
			status := "不可用"
			ok := false
			mxs, err := net.DefaultResolver.LookupMX(ctx, p.Domain)
			if err == nil && len(mxs) > 0 {
				host := strings.TrimSuffix(mxs[0].Host, ".")
				d := net.Dialer{Timeout: 4 * time.Second}
				conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, "25"))
				if err == nil {
					defer conn.Close()
					_ = conn.SetReadDeadline(time.Now().Add(4 * time.Second))
					line, _ := bufio.NewReader(conn).ReadString('\n')
					if strings.HasPrefix(line, "220") {
						status = "可用"
						ok = true
					}
				}
			}
			ch <- result{p.Name, status, ok}
		}()
	}
	rows = make([]map[string]any, 0, len(mailProviderDomains))
	for range mailProviderDomains {
		r := <-ch
		if r.ok {
			anyOK = true
		}
		rows = append(rows, map[string]any{"provider": r.name, "target": pDomainTarget(r.name), "status": r.status})
	}
	return anyOK, rows
}

func pDomainTarget(name string) string {
	for _, p := range mailProviderDomains {
		if p.Name == name {
			return p.Domain + ":25"
		}
	}
	return ""
}

func loadDNSBLZoneList(ctx context.Context, client *http.Client) []string {
	text, _ := fetchTextFirstOK(ctx, client, dnsblListURLs())
	if text == "" {
		return dnsBLZones
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, 128)
	add := func(z string) {
		z = strings.TrimSpace(z)
		if z == "" || strings.HasPrefix(z, "#") {
			return
		}
		if _, ok := seen[z]; ok {
			return
		}
		seen[z] = struct{}{}
		out = append(out, z)
	}
	for _, z := range dnsBLZones {
		add(z)
	}
	for _, line := range strings.Split(text, "\n") {
		add(line)
	}
	if len(out) > 120 {
		out = out[:120]
	}
	return out
}

func checkDNSBLParallel(ctx context.Context, address string, zones []string) (valid, normal, listed int, rows []map[string]any) {
	ip := net.ParseIP(address).To4()
	if ip == nil {
		return
	}
	reversed := fmt.Sprintf("%d.%d.%d.%d", ip[3], ip[2], ip[1], ip[0])
	type item struct {
		zone, status string
	}
	ch := make(chan item, len(zones))
	sem := make(chan struct{}, 40)
	var wg sync.WaitGroup
	for _, zone := range zones {
		zone := zone
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			lookupCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			answers, err := net.DefaultResolver.LookupHost(lookupCtx, reversed+"."+zone)
			status := "已标记"
			if err != nil {
				var dnsErr *net.DNSError
				if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
					status = "正常"
				} else {
					status = "查询失败"
				}
			} else {
				for _, answer := range answers {
					if strings.HasPrefix(answer, "127.255.255.") {
						status = "查询受限"
						break
					}
				}
			}
			ch <- item{zone, status}
		}()
	}
	go func() {
		wg.Wait()
		close(ch)
	}()
	for r := range ch {
		rows = append(rows, map[string]any{"database": r.zone, "status": r.status})
		if r.status == "查询受限" || r.status == "查询失败" {
			continue
		}
		valid++
		if r.status == "已标记" {
			listed++
		} else {
			normal++
		}
	}
	return
}
