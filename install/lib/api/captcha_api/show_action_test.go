package captcha_api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/daqing/airway/lib/repo"
	"github.com/daqing/airway/lib/sql"
	"github.com/gin-gonic/gin"

	"github.com/daqing/airway-tencentcloud-plugin/install/ignore/testdb"
	"github.com/daqing/airway-tencentcloud-plugin/install/lib/captcha"
	"github.com/daqing/airway-tencentcloud-plugin/install/lib/models"
)

const testIP = "10.0.0.1"

type envelope struct {
	Code    int            `json:"code"`
	Data    map[string]any `json:"data"`
	Message string         `json:"message"`
}

func setupRouter(t *testing.T) *gin.Engine {
	t.Helper()

	db, err := testdb.Setup()
	if err != nil {
		t.Fatalf("setup database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	gin.SetMode(gin.TestMode)
	r := gin.New()
	Routes(r.Group("/api/v1"))

	return r
}

func getCaptcha(t *testing.T, r *gin.Engine, ip string) envelope {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/captcha", nil)
	req.RemoteAddr = ip + ":12345"

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
	}

	var env envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode %s: %v", w.Body.String(), err)
	}

	return env
}

func TestShowActionIssuesACaptcha(t *testing.T) {
	env := getCaptcha(t, setupRouter(t), testIP)

	if env.Code != 0 || env.Message != "" {
		t.Fatalf("code = %d, message = %q", env.Code, env.Message)
	}

	token, _ := env.Data["token"].(string)
	if len(token) != 16 {
		t.Fatalf("token = %q, want 16 hex characters", token)
	}

	image, _ := env.Data["image"].(string)
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(image, prefix) {
		t.Fatalf("image = %q, want a base64 PNG data URL", image)
	}

	png, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(image, prefix))
	if err != nil {
		t.Fatalf("decode image: %v", err)
	}
	if !bytes.HasPrefix(png, []byte("\x89PNG\r\n\x1a\n")) {
		t.Fatalf("image = % x, want a PNG header", png[:8])
	}

	row, err := repo.FindOneBy[models.Captcha](sql.H{"token": token})
	if err != nil || row == nil {
		t.Fatalf("captcha row = %#v, err = %v", row, err)
	}
	// The token the client sees is not the row's own key.
	if row.ID == 0 {
		t.Fatal("row.ID = 0, want the generated primary key")
	}
	if row.IP != testIP {
		t.Fatalf("row.IP = %q, want %q", row.IP, testIP)
	}
	if len(row.Answer) != 4 || strings.Trim(row.Answer, "0123456789") != "" {
		t.Fatalf("row.Answer = %q, want 4 digits", row.Answer)
	}
}

func TestShowActionRateLimits(t *testing.T) {
	r := setupRouter(t)

	for i := 0; i < captcha.MaxPerIPPerHour; i++ {
		if env := getCaptcha(t, r, testIP); env.Code != 0 {
			t.Fatalf("captcha %d: code = %d, message = %q", i, env.Code, env.Message)
		}
	}

	env := getCaptcha(t, r, testIP)
	if env.Code != codeRateLimited {
		t.Fatalf("code = %d, want %d; message = %q", env.Code, codeRateLimited, env.Message)
	}
	if env.Data != nil {
		t.Fatalf("data = %#v, want null", env.Data)
	}
	if env.Message == "" {
		t.Fatal("message is empty, want a reason")
	}
}
