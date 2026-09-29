package captcha

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/daqing/airway/lib/repo"
	"github.com/daqing/airway/lib/sql"

	"github.com/daqing/airway-tencentcloud-plugin/install/ignore/testdb"
	"github.com/daqing/airway-tencentcloud-plugin/install/lib/models"
)

func setupDB(t *testing.T) {
	t.Helper()

	db, err := testdb.Setup()
	if err != nil {
		t.Fatalf("setup database: %v", err)
	}

	t.Cleanup(func() { db.Close() })
}

// issue returns a new captcha along with the answer it stored, which is what
// the client would read off the image.
func issue(t *testing.T, ip string) (id, answer string, png []byte) {
	t.Helper()

	id, png, err := Issue(context.Background(), ip)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	row, err := repo.FindOneBy[models.Captcha](sql.H{"id": id})
	if err != nil || row == nil {
		t.Fatalf("captcha row = %#v, err = %v", row, err)
	}

	return id, row.Answer, png
}

// wrongAnswer changes the leading digit so it cannot equal answer.
func wrongAnswer(answer string) string {
	first := "0"
	if answer[:1] == "0" {
		first = "1"
	}

	return first + answer[1:]
}

func TestIssueStoresAnswerAndImage(t *testing.T) {
	setupDB(t)

	id, answer, png := issue(t, "10.0.0.1")

	// utils.RandomHex(16) renders 8 random bytes, i.e. 16 hex characters.
	if len(id) != 16 || strings.Trim(id, "0123456789abcdef") != "" {
		t.Fatalf("id = %q, want 16 hex characters", id)
	}
	if len(answer) != AnswerLength || strings.Trim(answer, "0123456789") != "" {
		t.Fatalf("answer = %q, want %d digits", answer, AnswerLength)
	}
	if !bytes.HasPrefix(png, []byte("\x89PNG\r\n\x1a\n")) {
		t.Fatalf("image = % x, want a PNG header", png[:8])
	}
}

func TestIssueCapsPerIP(t *testing.T) {
	setupDB(t)

	for i := 0; i < MaxPerIPPerHour; i++ {
		issue(t, "10.0.0.1")
	}

	if _, _, err := Issue(context.Background(), "10.0.0.1"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("Issue past the cap: err = %v, want ErrRateLimited", err)
	}

	// The cap is checked before anything is written or rendered, so a refused
	// request leaves no row behind.
	stored, err := repo.CountWhere[models.Captcha](sql.H{"ip": "10.0.0.1"})
	if err != nil {
		t.Fatalf("count captchas: %v", err)
	}
	if stored != int64(MaxPerIPPerHour) {
		t.Fatalf("stored = %d, want %d", stored, MaxPerIPPerHour)
	}

	if _, _, err := Issue(context.Background(), "10.0.0.2"); err != nil {
		t.Fatalf("Issue from another ip: %v", err)
	}
}

func TestIssueForgetsCaptchasOlderThanAnHour(t *testing.T) {
	setupDB(t)

	for i := 0; i < MaxPerIPPerHour; i++ {
		issue(t, "10.0.0.1")
	}

	if err := repo.UpdateWhere[models.Captcha](
		sql.H{"created_at": time.Now().Add(-time.Hour - time.Minute)},
		sql.Eq("ip", "10.0.0.1"),
	); err != nil {
		t.Fatalf("age captchas: %v", err)
	}

	if _, _, err := Issue(context.Background(), "10.0.0.1"); err != nil {
		t.Fatalf("Issue after the window slid past: %v", err)
	}
}

func TestVerifyAcceptsOnce(t *testing.T) {
	setupDB(t)

	id, answer, _ := issue(t, "10.0.0.1")

	if !Verify(context.Background(), "10.0.0.1", id, answer) {
		t.Fatal("Verify rejected a correct answer")
	}
	if Verify(context.Background(), "10.0.0.1", id, answer) {
		t.Fatal("Verify accepted an already consumed captcha")
	}
}

func TestVerifyRejects(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(t *testing.T, id, answer string) (ip, verifyID, verifyAnswer string)
	}{
		{
			name: "wrong answer",
			prepare: func(_ *testing.T, id, answer string) (string, string, string) {
				return "10.0.0.1", id, wrongAnswer(answer)
			},
		},
		{
			name: "issued to another ip",
			prepare: func(_ *testing.T, id, answer string) (string, string, string) {
				return "10.0.0.2", id, answer
			},
		},
		{
			name: "unknown id",
			prepare: func(_ *testing.T, _, answer string) (string, string, string) {
				return "10.0.0.1", "00000000000000000000000000000000", answer
			},
		},
		{
			name: "expired",
			prepare: func(t *testing.T, id, answer string) (string, string, string) {
				t.Helper()

				if err := repo.UpdateWhere[models.Captcha](
					sql.H{"expires_at": time.Now().Add(-time.Minute)},
					sql.Eq("id", id),
				); err != nil {
					t.Fatalf("expire captcha: %v", err)
				}

				return "10.0.0.1", id, answer
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupDB(t)

			id, answer, _ := issue(t, "10.0.0.1")
			ip, verifyID, verifyAnswer := test.prepare(t, id, answer)

			if Verify(context.Background(), ip, verifyID, verifyAnswer) {
				t.Fatalf("Verify(%q, %q, %q) = true, want false", ip, verifyID, verifyAnswer)
			}
		})
	}
}
