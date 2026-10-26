// This executable drives the real pinned ntfy binary on loopback. It requires
// --network none and a writable /tmp; it never contacts a real relay or device.
package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func secret() string {
	b := make([]byte, 24)
	_, err := rand.Read(b)
	must(err)
	return hex.EncodeToString(b)
}
func main() {
	dir, err := os.MkdirTemp("", "ntfy-contract-")
	must(err)
	defer os.RemoveAll(dir)
	config := dir + "/server.yml"
	type relayRequest struct{ Path, Poll, Body, Auth string }
	captured := make(chan relayRequest, 16)
	relay := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		captured <- relayRequest{r.URL.Path, r.Header.Get("X-Poll-ID"), string(b), r.Header.Get("Authorization")}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"synthetic"}`)
	}))
	defer relay.Close()
	base := "http://127.0.0.1:8080"
	must(os.WriteFile(config, []byte(fmt.Sprintf("base-url: %s\nlisten-http: 127.0.0.1:8080\nauth-file: %s/auth.db\nauth-default-access: deny-all\ncache-file: %s/cache.db\nupstream-base-url: %s\nlog-level: error\n", base, dir, dir, relay.URL)), 0600))
	bootstrap := exec.Command("ntfy", "serve", "--config", config)
	must(bootstrap.Start())
	for deadline := time.Now().Add(10 * time.Second); ; {
		if _, err := os.Stat(dir + "/auth.db"); err == nil {
			time.Sleep(200 * time.Millisecond)
			break
		}
		if time.Now().After(deadline) {
			bootstrap.Process.Kill()
			bootstrap.Wait()
			panic("auth database bootstrap timeout")
		}
		time.Sleep(50 * time.Millisecond)
	}
	bootstrap.Process.Kill()
	bootstrap.Wait()
	run := func(env []string, args ...string) {
		c := exec.Command("ntfy", args...)
		c.Env = append(os.Environ(), append([]string{"NTFY_CONFIG_FILE=" + config}, env...)...)
		if output, err := c.CombinedOutput(); err != nil {
			message := string(output)
			for _, entry := range env {
				parts := strings.SplitN(entry, "=", 2)
				if len(parts) == 2 {
					message = strings.ReplaceAll(message, parts[1], "[redacted]")
				}
			}
			panic("ntfy setup failed: " + args[0] + ": " + message)
		}
	}
	passwords := map[string]string{"publisher": secret(), "reader-a": secret(), "reader-b": secret()}
	a, b := "opaque_"+secret(), "opaque_"+secret()
	for name, pw := range passwords {
		run([]string{"NTFY_PASSWORD=" + pw}, "user", "add", name)
	}
	for _, topic := range []string{a, b} {
		run(nil, "access", "publisher", topic, "wo")
	}
	run(nil, "access", "reader-a", a, "ro")
	run(nil, "access", "reader-b", b, "ro")
	tokenCommand := exec.Command("ntfy", "token", "add", "publisher")
	tokenCommand.Env = append(os.Environ(), "NTFY_CONFIG_FILE="+config)
	tokenOutput, err := tokenCommand.CombinedOutput()
	must(err)
	token := regexp.MustCompile(`tk_[A-Za-z0-9]+`).Find(tokenOutput)
	if len(token) == 0 {
		panic("synthetic token missing")
	}
	server := exec.Command("ntfy", "serve", "--config", config)
	must(server.Start())
	defer func() { server.Process.Kill(); server.Wait() }()
	client := &http.Client{Timeout: 3 * time.Second}
	for deadline := time.Now().Add(10 * time.Second); ; {
		r, e := client.Get(base + "/v1/health")
		if e == nil {
			r.Body.Close()
			if r.StatusCode == 200 {
				break
			}
		}
		if time.Now().After(deadline) {
			panic("server readiness timeout")
		}
		time.Sleep(50 * time.Millisecond)
	}
	count := 0
	check := func(name string, ok bool) {
		if !ok {
			panic("FAIL " + name)
		}
		count++
		fmt.Println("PASS", name)
	}
	request := func(method, path, user, body string) (int, []byte) {
		r, e := http.NewRequest(method, base+path, strings.NewReader(body))
		must(e)
		if user == "publisher" {
			r.Header.Set("Authorization", "Bearer "+string(token))
		} else if user != "" {
			r.SetBasicAuth(user, passwords[user])
		}
		if method == "POST" {
			r.Header.Set("Title", "meowGram")
		}
		resp, e := client.Do(r)
		must(e)
		defer resp.Body.Close()
		data, e := io.ReadAll(resp.Body)
		must(e)
		return resp.StatusCode, data
	}
	for _, t := range []struct {
		name, method, path, user string
		want                     int
	}{
		{"anonymous read denied", "GET", "/" + a + "/json?poll=1", "", 403},
		{"anonymous publish denied", "POST", "/" + a, "", 403},
		{"publisher read denied", "GET", "/" + a + "/json?poll=1", "publisher", 403},
		{"reader other topic denied", "GET", "/" + b + "/json?poll=1", "reader-a", 403},
		{"reader own publish denied", "POST", "/" + a, "reader-a", 403},
		{"reader other publish denied", "POST", "/" + b, "reader-a", 403},
	} {
		status, _ := request(t.method, t.path, t.user, "New message")
		check(t.name, status == t.want)
	}
	status, data := request("POST", "/"+a, "publisher", "New message")
	check("publisher write allowed", status == 200)
	var sent map[string]any
	must(json.Unmarshal(data, &sent))
	status, data = request("GET", "/"+a+"/json?poll=1", "reader-a", "")
	check("own topic read positive control", status == 200 && bytes.Contains(data, []byte(`"message":"New message"`)) && bytes.Contains(data, []byte(`"title":"meowGram"`)))
	status, _ = request("GET", "/"+b+"/json?poll=1", "reader-b", "")
	check("second reader own topic allowed", status == 200)
	select {
	case got := <-captured:
		hash := sha256.Sum256([]byte(base + "/" + a))
		check("relay opaque topic URL hash", got.Path == "/"+hex.EncodeToString(hash[:]))
		check("relay poll ID matches persisted message", got.Poll == sent["id"])
		check("relay content-free request body", got.Body == "")
		check("publisher credentials not forwarded", got.Auth == "")
	case <-time.After(5 * time.Second):
		panic("relay request timeout")
	}
	fmt.Printf("PASS ntfy contract: %d checks; TZ=%s; real binary v2.34.0; no external network\n", count, os.Getenv("TZ"))
}
