package sessions

import (
	"net/http"
	"time"

	"github.com/gwd/session-scheduler/id"
)

const (
	// Keep users logged in for 3 days
	sessionLength     = 24 * 3 * time.Hour
	sessionCookieName = "XenSummitWebSession"
	sessionIDLength   = 20
)

type SessionID string

func (sid *SessionID) generate() {
	*sid = SessionID(id.GenerateID("sess", sessionIDLength))
}

type StringMarshaler interface {
	String() string
	FromString(string)
}

type Session struct {
	ID     SessionID
	UserID string
	Expiry time.Time
}

type GetCookier interface {
	Cookie(string) (*http.Cookie, error)
}

func (session *Session) Expired() bool {
	return session.Expiry.Before(time.Now())
}

func NewSession(w http.ResponseWriter, uid string) (*Session, error) {
	expiry := time.Now().Add(sessionLength)

	session := &Session{
		Expiry: expiry,
		UserID: uid,
	}

	session.ID.generate()

	cookie := http.Cookie{
		Name:    sessionCookieName,
		Value:   string(session.ID),
		Expires: session.Expiry,
	}

	http.SetCookie(w, &cookie)

	err := store.Save(session)

	return session, err
}

// Typically you would pass your *http.Request
func RequestSession(r GetCookier) *Session {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil
	}

	session, err := store.Find(cookie.Value)
	if err != nil {
		panic(err)
	}

	if session == nil {
		return nil
	}

	if session.Expired() {
		store.Delete(session)
		return nil
	}
	return session
}

func FindOrCreateSession(w http.ResponseWriter, r GetCookier, uid string) (*Session, error) {
	err := error(nil)

	session := RequestSession(r)
	if session == nil || session.UserID != uid {
		session, err = NewSession(w, uid)
	}

	return session, err
}

func DeleteSessionByRequest(r GetCookier) error {
	if session := RequestSession(r); session != nil {
		if err := store.Delete(session); err != nil {
			return err
		}
	}

	return nil
}
