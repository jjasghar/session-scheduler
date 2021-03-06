package event

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/gwd/session-scheduler/id"
)

const (
	discussionIDLength = 20
)

type DiscussionID string

func (did *DiscussionID) generate() {
	*did = DiscussionID(id.GenerateID("disc", discussionIDLength))
}

// display.go:DiscussionGetDisplay()
//
// !IsPublic && ApprovedTitle == ""
//  -> Real title only available to admin or owner, hidden to everyone else
// !IsPublic && ApprovedTitle != ""
//  -> Real title shown to admin or owner, approved title to everyone else
// IsPublic
//  -> Real title to everyone
//
// SetPublic(true)
// - IsPublic = true, ApprovedX = X
// SetPublic(false)
// - IsPublic = false, ApprovedX = ""
//
// DiscussionUpdate(), owner.IsVerified
// - IsPublic = true, ApprovedX = newX
// DiscussionUpdate(), !owner.IsVerified
// -
//
// discussion.html:item-full
// IsPublic: Shows alert, "This discussion has changes awaiting moderation"
// "SetPublic" checkbox
//

type Discussion struct {
	DiscussionID DiscussionID
	Owner        UserID

	Title               string
	Description         string
	ApprovedTitle       string
	ApprovedDescription string

	// PossibleSlots []bool

	// Is this discussion publicly visible?
	// If true, 'Title' and 'Description' should be shown to everyone.
	// If false:
	//   admin and owner should see 'Title' and 'Description'
	//   Everyone else should either see 'Approved*', or nothing at all (if nothing has been approved)
	IsPublic bool
}

// Annotated for display to an individual user

type DisplaySlot struct {
	Label   string
	Index   int
	Checked bool
}

func (d *Discussion) GetURL() string {
	return "/uid/discussion/" + string(d.DiscussionID) + "/view"
}

// FIXME
const maxDiscussionsPerUser = 5

func checkDiscussionParams(disc *Discussion) error {
	if disc.Title == "" || AllWhitespace(disc.Title) {
		log.Printf("%s New/Update discussion failed: no title",
			disc.Owner)
		return errNoTitle
	}

	if disc.Description == "" || AllWhitespace(disc.Description) {
		log.Printf("%s New/Update discussion failed: no description",
			disc.Owner)
		return errNoDesc
	}
	return nil
}

// Restrictions:
// - Can't already have too many discussions
// - Title can't be empty
// - Description can't be empty
// - Title unique (enforced by SQL)
func NewDiscussion(disc *Discussion) error {
	owner := disc.Owner

	log.Printf("%s New discussion post: '%s'",
		owner, disc.Title)

	if err := checkDiscussionParams(disc); err != nil {
		return err
	}

	for {
		tx, err := event.Beginx()
		if shouldRetry(err) {
			tx.Rollback()
			continue
		} else if err != nil {
			return fmt.Errorf("Starting transaction: %v", err)
		}
		defer tx.Rollback()

		count := 0
		err = tx.Get(&count,
			`select count(*) from event_discussions where owner=?`,
			disc.Owner)
		if shouldRetry(err) {
			tx.Rollback()
			continue
		} else if err != nil {
			return fmt.Errorf("Getting discussion count for user %v: %v",
				disc.Owner, err)
		}

		if count >= maxDiscussionsPerUser {
			return errTooManyDiscussions
		}

		disc.DiscussionID.generate()

		// New discussions are non-public by default unless owner is verified
		err = tx.Get(&disc.IsPublic,
			`select isverified from event_users where userid = ?`,
			owner)
		if shouldRetry(err) {
			tx.Rollback()
			continue
		} else if err != nil {
			return err
		}

		if disc.IsPublic {
			disc.ApprovedTitle = disc.Title
			disc.ApprovedDescription = disc.Description
		} else {
			disc.ApprovedTitle = ""
			disc.ApprovedDescription = ""
		}
		_, err = tx.Exec(
			`insert into event_discussions values (?, ?, ?, ?, ?, ?, ?)`,
			disc.DiscussionID, disc.Owner, disc.Title, disc.Description,
			disc.ApprovedTitle, disc.ApprovedDescription,
			disc.IsPublic)
		if shouldRetry(err) {
			tx.Rollback()
			continue
		} else if err != nil {
			return err
		}

		// Owners are assumed to want to attend their own session
		err = setInterestTx(tx, disc.Owner, disc.DiscussionID, InterestMax)
		if shouldRetry(err) {
			tx.Rollback()
			continue
		} else if err != nil {
			return err
		}

		err = tx.Commit()
		if shouldRetry(err) {
			tx.Rollback()
			continue
		} else if err != nil {
			return err
		}

		return nil
	}
}

// GetMaxScore returns the maximum possible score a discussion could
// have if everyone attended; that is, the sum of all the interests
// expressed.
func (d *Discussion) GetMaxScore() (int, error) {
	var maxscore int
	// Theoretically the owner should always have non-zero interest,
	// so sum(interest) should never be NULL; but better to be robust.
	for {
		err := event.Get(&maxscore, `
            select IFNULL(sum(interest), 0)
                from event_interest
                where discussionid = ?`,
			d.DiscussionID)
		switch {
		case shouldRetry(err):
			continue
		case err != nil:
			log.Printf("INTERNAL ERROR: Getting max score for discussion %v: %v",
				d.DiscussionID, err)
			return 0, err
		default:
			return maxscore, err
		}
	}
}

func (d *Discussion) Location() Location {
	// FIXME: Location
	return Location{}
}

func (d *Discussion) Slot() (IsFinal bool, Time string) {
	// FIXME: Slot
	return false, ""
}

// Updates discussion's Title, Description, and Owner.
//
// If the owner (or the new owner, if that's being changed) is verifed, then
// IsPublic will be set to 'true', and ApprovedTitle and ApprovedDescription will be set from
// Title and Description as well.
//
// If the owner (or new owner) is not verified, then IsPublic will be
// set to false, and only Title and Description will be modified.
func DiscussionUpdate(disc *Discussion) error {
	log.Printf("Update discussion post: '%s'", disc.Title)

	if err := checkDiscussionParams(disc); err != nil {
		return err
	}

	for {
		tx, err := event.Beginx()
		if shouldRetry(err) {
			tx.Rollback()
			continue
		} else if err != nil {
			return fmt.Errorf("Starting transaction: %v", err)
		}
		defer tx.Rollback()

		var curOwner UserID
		var ownerIsVerified bool
		row := tx.QueryRow(
			`select owner, isverified
                 from event_discussions
                   join event_users on owner = userid
                 where discussionid = ?`, disc.DiscussionID)
		err = row.Scan(&curOwner, &ownerIsVerified)
		if shouldRetry(err) {
			tx.Rollback()
			continue
		} else if err != nil {
			return fmt.Errorf("Getting info for owner of discussion %v: %v",
				disc.DiscussionID, err)
		}

		// NB: We don't check for number owned discussions. Only the
		// admin can re-assign a discussion; they can be allowed the
		// latitude.

		if disc.Owner != curOwner {
			err := setInterestTx(tx, disc.Owner, disc.DiscussionID, InterestMax)
			if shouldRetry(err) {
				tx.Rollback()
				continue
			} else if err != nil {
				return fmt.Errorf("Setting interest for new owner %v: %v",
					disc.Owner, err)
			}

			// If we're changing owner, we need to see whether the new owner is verified
			err = tx.Get(&ownerIsVerified,
				`select isverified from event_users where userid = ?`,
				disc.Owner)
			if shouldRetry(err) {
				tx.Rollback()
				continue
			} else if err != nil {
				return fmt.Errorf("Getting IsVerified for new owner %v: %v",
					disc.Owner, err)
			}
		}

		// Editing a discussion takes it non-public unless the owner is verified.
		disc.IsPublic = ownerIsVerified

		q :=
			`update event_discussions set
                 owner = ?,
                 title = ?,
                 description = ?,
                 ispublic = ?`
		args := []interface{}{disc.Owner, disc.Title, disc.Description, disc.IsPublic}

		if disc.IsPublic {
			disc.ApprovedTitle = disc.Title
			disc.ApprovedDescription = disc.Description
			q += `,
                 approvedtitle = ?,
                 approveddescription = ?`
			args = append(args, disc.ApprovedTitle)
			args = append(args, disc.ApprovedDescription)
		}

		q += `where discussionid = ?`
		args = append(args, disc.DiscussionID)

		_, err = tx.Exec(q, args...)
		if shouldRetry(err) {
			tx.Rollback()
			continue
		} else if err != nil {
			return err
		}

		err = tx.Commit()
		if shouldRetry(err) {
			tx.Rollback()
			continue
		} else if err != nil {
			return err
		}

		return nil
	}
}

// Sets the given discussion ID to public or private.
//
// If public is true, it copies the title and description into the
// "approved" title and description, so that it will be visible even
// after being modified.
//
// If public is false, it hides the discussion entirely, by both
// setting 'IsPublic' to false, but also clearing the approved title
// and description.
func DiscussionSetPublic(discussionid DiscussionID, public bool) error {
	var query, errlogfmt string
	if public {
		query = `
        update event_discussions
            set (approvedtitle, approveddescription, ispublic)
            = (select title, description, true
                   from event_discussions
                   where discussionid = :did)
            where discussionid = :did`
		errlogfmt = "Setting event discussion %v public: %v"
	} else {
		query = `
        update event_discussions
            set ispublic = false,
                approvedtitle = "",
                approveddescription = ""
            where discussionid = ?`
		errlogfmt = "Setting event discussion %v non-public: %v"
	}

	for {
		res, err := event.Exec(query, discussionid)
		switch {
		case shouldRetry(err):
			continue
		case err != nil:
			log.Printf(errlogfmt, discussionid, err)
			return ErrInternal
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
}

func DeleteDiscussion(did DiscussionID) error {
	log.Printf("Deleting discussion %s", did)

	for {
		tx, err := event.Beginx()
		if shouldRetry(err) {
			tx.Rollback()
			continue
		} else if err != nil {
			return fmt.Errorf("Starting transaction: %v", err)
		}
		defer tx.Rollback()

		// Delete foreign key references first
		_, err = tx.Exec(`
           delete from event_interest
               where discussionid = ?`, did)
		if shouldRetry(err) {
			tx.Rollback()
			continue
		} else if err != nil {
			return fmt.Errorf("Deleting discussion from event_interest: %v", err)
		}

		res, err := tx.Exec(`
        delete from event_discussions
            where discussionid = ?`, did)
		if shouldRetry(err) {
			tx.Rollback()
			continue
		} else if err != nil {
			return fmt.Errorf("Deleting discussion from event_discussions: %v", err)
		}

		rcount, err := res.RowsAffected()
		if shouldRetry(err) {
			tx.Rollback()
			continue
		} else if err != nil {
			log.Printf("ERROR Getting number of affected rows: %v; continuing", err)
		}
		switch {
		case rcount == 0:
			return ErrDiscussionNotFound
		case rcount > 1:
			log.Printf("ERROR Expected to change 1 row, changed %d", rcount)
			return ErrInternal
		}

		err = tx.Commit()
		if shouldRetry(err) {
			tx.Rollback()
			continue
		} else if err != nil {
			return err
		}
		return nil
	}
	return nil
}

func MakePossibleSlots(len int) []bool {
	pslots := make([]bool, len)
	for i := range pslots {
		pslots[i] = true
	}
	return pslots
}

func DiscussionFindById(discussionid DiscussionID) (*Discussion, error) {
	disc := &Discussion{}
	for {
		err := event.Get(disc,
			`select * from event_discussions where discussionid = ?`,
			discussionid)
		switch {
		case shouldRetry(err):
			continue
		case err == sql.ErrNoRows:
			return nil, nil
		default:
			return disc, err
		}
	}
}

func discussionIterateQuery(query string, args []interface{}, f func(*Discussion) error) error {
	for {
		rows, err := event.Queryx(query, args...)
		switch {
		case shouldRetry(err):
			continue
		case err != nil:
			return err
		}
		defer rows.Close()
		processed := 0
		for rows.Next() {
			var disc Discussion
			if err := rows.StructScan(&disc); err != nil {
				return err
			}
			err = f(&disc)
			if err != nil {
				return err
			}
			processed++
		}

		// For some reason we often get the transaction conflict error
		// from rows.Err() rather than from the original Queryx.
		// Retrying is fine as long as we haven't actually processed
		// any rows yet.  If we have, throw an error.  (There's an
		// argument to makign this a panic() instead.)
		err = rows.Err()
		if shouldRetry(err) {
			if processed == 0 {
				rows.Close()
				continue
			}
			err = fmt.Errorf("INTERNAL ERROR: Got transaction retry error after processing %d callbacks",
				processed)
		}

		return err
	}
}

func DiscussionIterate(f func(*Discussion) error) error {
	return discussionIterateQuery(`select * from event_discussions order by discussionid`, nil, f)
}

// FIXME: This will simply do nothing if the userid doesn't exist.  It
// would be nice for the caller to distinguish between "User does not
// exist" and "User has no discussions".
func DiscussionIterateUser(userid UserID, f func(*Discussion) error) (err error) {
	return discussionIterateQuery(`select * from event_discussions where owner=? order by discussionid`, []interface{}{userid}, f)
}
