// Package httpclient 提供统一 HTTP 出口。
//
// 默认基于标准库 net/http，预配置连接复用、分层超时、JSON 编码
// SetEscapeHTML(false)。渠道依赖 Doer 接口而非具体实现，调用方可注入
// resty/req 等第三方 client。
package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"time"
)

// Doer 最小化 HTTP 执行接口，net/http 的 *http.Client 天然满足。
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// defaultClient 预配置的 net/http 客户端。
var defaultClient = &http.Client{
	Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second, // Dial 超时
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   5 * time.Second, // TLS 超时
		ResponseHeaderTimeout: 30 * time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		ForceAttemptHTTP2:     true,
	},
	Timeout: 30 * time.Second, // 整体超时
}

// global 是全局 Doer，默认为 defaultClient；可经 SetGlobalDoer 替换。
var global Doer = defaultClient

// SetGlobalDoer 替换全局 HTTP 执行器（如换成 resty/req 的 client，需实现 Doer）。
// 非并发安全，应在程序初始化阶段调用一次。
func SetGlobalDoer(d Doer) {
	if d != nil {
		global = d
	}
}

// Default 返回库默认的 net/http 客户端。
func Default() *http.Client { return defaultClient }

// PostJSON 以 application/json 发送 POST 请求，返回响应体字节。
// JSON 编码使用 SetEscapeHTML(false)（消息推送场景不应转义 HTML 实体）。
func PostJSON(ctx context.Context, url string, body any) ([]byte, int, error) {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(body); err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, buf)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	return doRequest(ctx, req)
}

// PostJSONWith 返回 *http.Response，供需要读取响应头/状态的渠道使用。
// 调用方负责关闭 resp.Body。
func PostJSONWith(ctx context.Context, url string, body any) (*http.Response, error) {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(body); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	return global.Do(req)
}

// PostText 以 text/plain 或 application/x-www-form-urlencoded 发送 POST。
func PostText(ctx context.Context, url string, contentType string, body []byte) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	if contentType == "" {
		contentType = "application/json; charset=utf-8"
	}
	req.Header.Set("Content-Type", contentType)
	return doRequest(ctx, req)
}

// Get 发送 GET 请求。
func Get(ctx context.Context, url string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	return doRequest(ctx, req)
}

// Do 执行自定义请求，返回响应体字节与 HTTP 状态码。
// 供需要自定义 method/path/headers 的渠道（如自定义 webhook）使用。
func Do(req *http.Request) ([]byte, int, error) {
	resp, err := global.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	buf := &bytes.Buffer{}
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return nil, resp.StatusCode, err
	}
	return buf.Bytes(), resp.StatusCode, nil
}

func doRequest(ctx context.Context, req *http.Request) ([]byte, int, error) {
	resp, err := global.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	buf := &bytes.Buffer{}
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return nil, resp.StatusCode, err
	}
	return buf.Bytes(), resp.StatusCode, nil
}