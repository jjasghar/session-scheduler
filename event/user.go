package event

import (
	"log"

	"golang.org/x/crypto/bcrypt"

	"github.com/gwd/session-scheduler/id"
)

const (
	hashCost       = 10
	passwordLength = 6
	userIDLength   = 16
	InterestMax    = 100
)

type UserID string

func (uid *UserID) generate() {
	*uid = UserID(id.GenerateID("usr", userIDLength))
}

type User struct {
	UserID         UserID
	HashedPassword string
	Username       string
	IsAdmin        bool
	IsVerified     bool // Has entered the verification code
	//Interest       map[DiscussionID]int
	RealName    string
	Email       string
	Company     string
	Description string
}

func (u *User) MayEditUser(tgt *User) bool {
	return u.IsAdmin || u.UserID == tgt.UserID
}

func (u *User) MayEditDiscussion(d *Discussion) bool {
	return u.IsAdmin || u.UserID == d.Owner
}

func updateUser(user *User) error {
	res, err := event.Exec(`
        update event_users set
            hashedpassword = ?,
            isadmin = ?, isverified = ?,
            realname = ?, email = ?, company = ?, description = ?
          where userid = ?`,
		user.HashedPassword,
		user.IsAdmin, user.IsVerified,
		user.RealName, user.Email, user.Company, user.Description,
		user.UserID)
	if err != nil {
		return err
	}
	rcount, err := res.RowsAffected()
	if err != nil {
		log.Printf("ERROR Getting number of affected rows: %v; continuing", err)
	}
	switch {
	case rcount == 0:
		return ErrUserNotFound
	case rcount > 1:
		log.Printf("ERROR Expected to change 1 row, changed %d", rcount)
		return ErrInternal
	}
	return nil
}

func NewUser(password string, user *User) (UserID, error) {
	log.Printf("New user post: '%s'", user.Username)

	if user.Username == "" || AllWhitespace(user.Username) {
		log.Printf("New user failed: no username")
		return user.UserID, errNoUsername
	}

	if IsEmailAddress(user.Username) {
		log.Printf("New user failed: Username looks like an email address")
		return user.UserID, errUsernameIsEmail
	}

	switch {
	case user.HashedPassword == "" && password == "":
		if password == "" {
			log.Printf("New user failed: no password")
			return user.UserID, errNoPassword
		}
	case password != "":
		if len(password) < passwordLength {
			log.Printf("New user failed: password too short")
			return user.UserID, errPasswordTooShort
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), hashCost)
		if err != nil {
			log.Printf("Hashing password failed?! %v", err)
			return user.UserID, ErrInternal
		}
		user.HashedPassword = string(hashedPassword)
	}
	user.UserID.generate()

	_, err := event.Exec(`
        insert into event_users(
            userid,
            hashedpassword,
            username,
            isadmin, isverified,
            realname, email, company, description) values(?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		user.UserID,
		user.HashedPassword,
		user.Username,
		user.IsAdmin, user.IsVerified,
		user.RealName, user.Email, user.Company, user.Description)
	switch {
	case isErrorConstraintUnique(err):
		log.Printf("New user failed: user exists")
		return user.UserID, errUsernameExists
	case err != nil:
		log.Printf("New user failed: %v", err)
		return user.UserID, err
	}

	return user.UserID, err
}

func (u *User) CheckPassword(password string) bool {
	// Don't bother checking the password if it's empty
	if password == "" ||
		bcrypt.CompareHashAndPassword(
			[]byte(u.HashedPassword),
			[]byte(password),
		) != nil {
		return false
	}
	return true
}

// Use when you plan on setting a large sequence in a row and can save
// the state yourself
func (user *User) SetInterestNosave(disc *Discussion, interest int) error {
	log.Printf("Setinterest: %s '%s' %d", user.Username, disc.Title, interest)
	if interest > InterestMax || interest < 0 {
		log.Printf("SetInterest failed: Invalid interest")
		return errInvalidInterest
	}
	if interest > 0 {
		// FIXME: Interest
		//user.Interest[disc.ID] = interest
		disc.Interested[user.UserID] = true
	} else {
		// FIXME: Interest
		//delete(user.Interest, disc.ID)
		delete(disc.Interested, user.UserID)
		disc.maxScoreValid = false // Lazily update this when it's wanted
	}
	event.ScheduleState.Modify()
	return nil
}

func (user *User) SetInterest(disc *Discussion, interest int) error {
	err := user.SetInterestNosave(disc, interest)
	if err == nil {
		event.Save()
	}
	return err
}

func (user *User) SetPassword(newPassword string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), hashCost)
	if err != nil {
		return err
	}
	user.HashedPassword = string(hashedPassword)
	return nil
}

func (user *User) SetVerified(isVerified bool) error {
	_, err := event.Exec(`
    update event_users set isverified = ? where userid = ?`,
		isVerified, user.UserID)
	return err
}

func UserRemoveDiscussion(did DiscussionID) error {
	// FIXME: Interest
	return nil
}

func UserUpdate(userNext, modifier *User, currentPassword, newPassword string) error {
	if newPassword != "" {
		// No current password? Don't try update the password.
		// FIXME: Huh?
		if currentPassword == "" {
			return nil
		}

		if bcrypt.CompareHashAndPassword(
			[]byte(modifier.HashedPassword),
			[]byte(currentPassword),
		) != nil {
			return errPasswordIncorrect
		}

		if len(newPassword) < passwordLength {
			return errPasswordTooShort
		}

		err := userNext.SetPassword(newPassword)
		if err != nil {
			return err
		}
	}

	return updateUser(userNext)
}

func DeleteUser(userid UserID) error {
	// FIXME: Transaction
	dlist := event.Discussions.GetDidListUser(userid)

	for _, did := range dlist {
		DeleteDiscussion(did)
	}

	DiscussionRemoveUser(userid)

	res, err := event.Exec(`
        delete from event_users where userid=?`,
		userid)
	if err != nil {
		return err
	}

	rcount, err := res.RowsAffected()
	if err != nil {
		log.Printf("ERROR Getting number of affected rows: %v; continuing", err)
	}
	switch {
	case rcount == 0:
		return ErrUserNotFound
	case rcount > 1:
		log.Printf("ERROR Expected to change 1 row, changed %d", rcount)
		return ErrInternal
	}
	return nil
}
