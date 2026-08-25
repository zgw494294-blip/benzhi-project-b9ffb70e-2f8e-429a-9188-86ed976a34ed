package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerServesRootWithoutRedirect(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()

	Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("状态码 = %d，期望 %d", recorder.Code, http.StatusOK)
	}
	if location := recorder.Header().Get("Location"); location != "" {
		t.Fatalf("根路径不应重定向，Location = %q", location)
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/html") {
		t.Fatalf("Content-Type = %q，期望 text/html", contentType)
	}
	if body := recorder.Body.String(); !strings.Contains(body, "<!doctype html>") {
		t.Fatalf("根路径未返回嵌入的工作台页面")
	}
}

func TestHandlerServesStaticAsset(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/styles.css", nil)
	recorder := httptest.NewRecorder()

	Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("状态码 = %d，期望 %d", recorder.Code, http.StatusOK)
	}
	if body := recorder.Body.String(); body == "" {
		t.Fatal("静态资源响应为空")
	}
}

func TestHandlerRejectsWriteMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	recorder := httptest.NewRecorder()

	Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("状态码 = %d，期望 %d", recorder.Code, http.StatusMethodNotAllowed)
	}
	if allow := recorder.Header().Get("Allow"); allow != "GET, HEAD" {
		t.Fatalf("Allow = %q，期望 GET, HEAD", allow)
	}
}
