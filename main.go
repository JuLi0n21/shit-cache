package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"fmt"
	"time"
	"log"
)

var allowedDomains []string

func init() {
	env := os.Getenv("ALLOWED_DOMAINS")
	if env != "" {
		parts := strings.Split(env, ",")
		for _, p := range parts {
			cleanDomain := strings.ToLower(strings.TrimSpace(p))
			if cleanDomain != "" {
				allowedDomains = append(allowedDomains, cleanDomain)
			}
		}
	}

	fmt.Println("allowed domains:", env)
}

func main() {
	http.HandleFunc("/", proxyHandler)
	http.ListenAndServe(":8080", nil)
}

func isDomainAllowed(host string) bool {
	if len(allowedDomains) == 0 {
		return true
	}

	host = strings.ToLower(host)
	for _, domain := range allowedDomains {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}

func proxyHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	encodedPath := strings.TrimPrefix(r.URL.Path, "/")
	if encodedPath == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	decodedBytes, err := base64.URLEncoding.DecodeString(encodedPath)
	if err != nil {
		decodedBytes, err = base64.RawURLEncoding.DecodeString(encodedPath)
		if err != nil {
			decodedBytes, err = base64.StdEncoding.DecodeString(encodedPath)
			if err != nil {
				decodedBytes, err = base64.RawStdEncoding.DecodeString(encodedPath)
				if err != nil {
					http.Error(w, "Invalid Base64", http.StatusBadRequest)
					return
				}
			}
		}
	}

	targetURLStr := string(decodedBytes)
	if !strings.HasPrefix(targetURLStr, "http://") && !strings.HasPrefix(targetURLStr, "https://") {
		targetURLStr = "https://" + targetURLStr
	}

	parsedTargetURL, err := url.Parse(targetURLStr)
	if err != nil {
		http.Error(w, "Invalid Target URL", http.StatusBadRequest)
		return
	}

	if !isDomainAllowed(parsedTargetURL.Hostname()) {
		log.Println("blocked:", parsedTargetURL.Hostname())
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	fullRequestURL := targetURLStr
	if r.URL.RawQuery != "" {
		if strings.Contains(fullRequestURL, "?") {
			fullRequestURL += "&" + r.URL.RawQuery
		} else {
			fullRequestURL += "?" + r.URL.RawQuery
		}
	}

	hasher := sha256.New()
	hasher.Write([]byte(fullRequestURL))
	hashHex := hex.EncodeToString(hasher.Sum(nil))

	matches, _ := filepath.Glob(hashHex + "*")
	if len(matches) > 0 {
		cachedFile := matches[0]
		file, err := os.Open(cachedFile)
		if err == nil {
			defer file.Close()
			ext := filepath.Ext(cachedFile)
			if contentType := mime.TypeByExtension(ext); contentType != "" {
				w.Header().Set("Content-Type", contentType)
			}
			io.Copy(w, file)
			log.Println("cache hit  | took:", time.Since(start))
			return
		}
	}

	proxyReq, err := http.NewRequest(r.Method, fullRequestURL, r.Body)
	if err != nil {
		http.Error(w, "Error creating request", http.StatusInternalServerError)
		return
	}

	for name, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(name, value)
		}
	}
	proxyReq.Host = proxyReq.URL.Host
	proxyReq.Header.Del("Accept-Encoding")

	client := &http.Client{}
	resp, err := client.Do(proxyReq)
	if err != nil {
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}
	w.WriteHeader(resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		io.Copy(w, resp.Body)
		return
	}

	originalExt := path.Ext(parsedTargetURL.Path)
	ext := originalExt

	if ext == "" {
		contentType := resp.Header.Get("Content-Type")
		if contentType != "" {
			mediaType, _, err := mime.ParseMediaType(contentType)
			if err == nil {
				exts, err := mime.ExtensionsByType(mediaType)
				if err == nil && len(exts) > 0 {
					ext = exts[0]
				}
			}
		}
	}

	safeFileName := hashHex + ext

	file, err := os.Create(safeFileName)
	if err != nil {
		io.Copy(w, resp.Body)
		return
	}
	defer file.Close()
	log.Println("cache miss | took:", time.Since(start))

	tee := io.TeeReader(resp.Body, file)
	io.Copy(w, tee)
}