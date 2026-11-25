package httpapi

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPasswordVerifier(t *testing.T) {
	hash := hashPassword("正确密码123")
	if hash == hashPassword("正确密码123") || !checkPassword("正确密码123", hash) || checkPassword("错误密码123", hash) || checkPassword("正确密码123", "") {
		t.Fatal("salted password verification failed")
	}
}

func TestAccountInputAndOrigin(t *testing.T) {
	mux := routes(&fakeStore{})
	for _, tc := range []struct {
		body, origin string
		status       int
	}{
		{`{"username":"ab","password":"correct"}`, "", 400},
		{`{"username":"valid","password":"short"}`, "", 400},
		{`{"username":"valid","password":"correct","nickname":"` + strings.Repeat("字", 25) + `"}`, "", 400},
		{`{"username":"valid","password":"correct"}{}`, "", 400},
		{`{"username":"valid","password":"` + strings.Repeat("x", 8200) + `"}`, "", 400},
		{`{"username":"valid","password":"correct"}`, "https://different.example", 403},
		{`{"username":"valid","password":"correct"}`, "", 503},
	} {
		r := httptest.NewRequest("POST", "/api/auth/register", strings.NewReader(tc.body))
		r.Header.Set("Origin", tc.origin)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Errorf("registration status %d, want %d", w.Code, tc.status)
		}
	}
}
