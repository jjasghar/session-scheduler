package main

import (
	"html/template"
	"log"
	"sort"

	"github.com/gwd/session-scheduler/event"
)

type UserProfile struct {
	RealName string
	Email    string
	Company  string
}

type UserDisplay struct {
	ID          event.UserID
	Username    string
	IsAdmin     bool
	IsVerified  bool // Has entered the verification code
	MayEdit     bool
	Profile     UserProfile
	Description template.HTML
	List        []*DiscussionDisplay
}

func UserGetDisplay(u *event.User, cur *event.User, long bool) (ud *UserDisplay) {
	ud = &UserDisplay{
		ID:         u.UserID,
		Username:   u.Username,
		IsVerified: u.IsVerified,
	}
	if cur != nil {
		ud.MayEdit = cur.MayEditUser(u)
		ud.IsAdmin = cur.IsAdmin
		// Only display profile information to people who are logged in
		ud.Profile.RealName = u.RealName
		ud.Profile.Email = u.Email
		ud.Profile.Company = u.Company
		ud.Description = ProcessText(u.Description)
		ud.List = DiscussionGetListUser(u, cur)
	}
	return
}

type DiscussionDisplay struct {
	ID             event.DiscussionID
	Title          string
	Description    template.HTML
	DescriptionRaw string
	Owner          *event.User
	Interested     []*event.User
	IsPublic       bool
	// IsUser: Used to determine whether to display 'interest'
	IsUser bool
	// MayEdit: Used to determine whether to show edit / delete buttons
	MayEdit bool
	// IsAdmin: Used to determine whether to show slot scheduling options
	IsAdmin       bool
	Interest      int
	Location      event.Location
	Time          string
	IsFinal       bool
	PossibleSlots []event.DisplaySlot
	AllUsers      []event.User
}

func DiscussionGetDisplay(d *event.Discussion, cur *event.User) *DiscussionDisplay {
	showMain := true

	// Only display a discussion if:
	// 1. It's pulbic, or...
	// 2. The current user is admin, or the discussion owner
	if !d.IsPublic &&
		(cur == nil || (!cur.IsAdmin && cur.UserID != d.Owner)) {
		if d.ApprovedTitle == "" {
			return nil
		} else {
			showMain = false
		}
	}

	dd := &DiscussionDisplay{
		ID:       d.DiscussionID,
		IsPublic: d.IsPublic,
	}

	if showMain {
		dd.Title = d.Title
		dd.DescriptionRaw = d.Description
	} else {
		dd.Title = d.ApprovedTitle
		dd.DescriptionRaw = d.ApprovedDescription
	}

	dd.Description = ProcessText(dd.DescriptionRaw)

	dd.Location = d.Location()

	dd.IsFinal, dd.Time = d.Slot()

	dd.Owner, _ = event.UserFind(d.Owner)
	if cur != nil {
		if cur.Username != event.AdminUsername {
			dd.IsUser = true
			// FIXME: Interest
			//dd.Interest = cur.Interest[d.ID]
		}
		dd.MayEdit = cur.MayEditDiscussion(d)
		if cur.IsAdmin {
			dd.IsAdmin = true
			//dd.PossibleSlots = event.TimetableFillDisplaySlots(d.PossibleSlots)
			var err error
			dd.AllUsers, err = event.UserGetAll()
			if err != nil {
				// Report error but continue
				log.Printf("INTERNAL ERROR: Getting all users: %v", err)
			}
		}
	}
	// for uid := range d.Interested {
	// 	a, _ := event.UserFind(uid)
	// 	if a != nil {
	// 		dd.Interested = append(dd.Interested, a)
	// 	}
	// }
	return dd
}

func DiscussionGetListUser(u *event.User, cur *event.User) (list []*DiscussionDisplay) {
	event.DiscussionIterate(func(d *event.Discussion) error {
		if d.Owner == u.UserID {
			dd := DiscussionGetDisplay(d, cur)
			if dd != nil {
				list = append(list, dd)
			}
		}
		return nil
	})

	sort.Slice(list, func(i, j int) bool {
		return list[i].ID < list[j].ID
	})

	return
}

func DiscussionGetList(cur *event.User) (list []*DiscussionDisplay) {
	event.DiscussionIterate(func(d *event.Discussion) error {
		dd := DiscussionGetDisplay(d, cur)
		if dd != nil {
			list = append(list, dd)
		}
		return nil
	})

	sort.Slice(list, func(i, j int) bool {
		return list[i].ID < list[j].ID
	})

	return
}

func UserGetUsersDisplay(cur *event.User) (users []*UserDisplay) {
	event.UserIterate(func(u *event.User) error {
		if u.Username != event.AdminUsername {
			users = append(users, UserGetDisplay(u, cur, false))
		}
		return nil
	})

	sort.Slice(users, func(i, j int) bool {
		return users[i].ID < users[j].ID
	})
	return
}