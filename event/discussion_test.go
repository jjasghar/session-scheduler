package event

import (
	"fmt"
	"sort"
	"testing"

	"github.com/icrowley/fake"
)

func testNewDiscussion(t *testing.T, owner UserID) (Discussion, bool) {
	disc := Discussion{
		Title:       fake.Title(),
		Description: fake.Paragraphs(),
	}

	if owner == "" {
		user, err := UserFindRandom()
		if err != nil {
			t.Logf("Getting a random user: %v", err)
			return disc, true
		}
		disc.Owner = user.UserID
	} else {
		disc.Owner = owner
	}

	//t.Logf("Creating test discussion %v", disc)

	failures := 0
	for err := NewDiscussion(&disc); err != nil; err = NewDiscussion(&disc) {
		failures++
		if failures > 10 {
			t.Logf("%d failures exceeded tolerance.  Most recent failure: %v", failures, err)
			return disc, true
		}

		switch err {
		case errTitleExists:
			// Somehow we got a collision?  Try a new one
			disc.Title = fake.Title()
			continue
		case errTooManyDiscussions:
			if owner == "" {
				// Try a new user if this one has too many.  If all users
				// have too many, we'll eventually hit the max failures
				// above.
				user, err2 := UserFindRandom()
				if err2 != nil {
					t.Logf("Getting a random user: %v", err2)
					return disc, true
				}
				disc.Owner = user.UserID
				continue
			}
		}

		t.Errorf("Error creating new discussion: %v", err)
		return disc, true
	}

	// Only 25% of discussions have constraints
	// if disc != nil && rand.Intn(4) == 0 {
	// 	// Make a continuous range where it's not schedulable
	// 	start := rand.Intn(len(disc.PossibleSlots))
	// 	end := rand.Intn(len(disc.PossibleSlots)-start) + 1
	// 	if start != 0 || end != len(disc.PossibleSlots) {
	// 		for i := start; i < end; i++ {
	// 			disc.PossibleSlots[i] = false
	// 		}
	// 	}
	// }

	return disc, false
}

func compareDiscussions(d1, d2 *Discussion, t *testing.T) bool {
	ret := true
	if d1.DiscussionID != d2.DiscussionID {
		t.Logf("mismatch DiscussionID: %v != %v", d1.DiscussionID, d2.DiscussionID)
		ret = false
	}
	if d1.Title != d2.Title {
		t.Logf("mismatch Title: %v != %v", d1.Title, d2.Title)
		ret = false
	}
	if d1.Description != d2.Description {
		t.Logf("mismatch Description: %v != %v", d1.Description, d2.Description)
		ret = false
	}
	if d1.ApprovedTitle != d2.ApprovedTitle {
		t.Logf("mismatch ApprovedTitle: %v != %v", d1.ApprovedTitle, d2.ApprovedTitle)
		ret = false
	}
	if d1.ApprovedDescription != d2.ApprovedDescription {
		t.Logf("mismatch ApprovedDescription: %v != %v", d1.ApprovedDescription, d2.ApprovedDescription)
		ret = false
	}
	if d1.Owner != d2.Owner {
		t.Logf("mismatch Owner: %v != %v", d1.Owner, d2.Owner)
		ret = false
	}
	if d1.IsPublic != d2.IsPublic {
		t.Logf("mismatch IsPublic: %v != %v", d1.IsPublic, d2.IsPublic)
		ret = false
	}
	return ret
}

func testUnitDiscussion(t *testing.T) (exit bool) {
	exit = true

	tc := dataInit(t)
	if tc == nil {
		return
	}

	// Make 6 users for testing
	testUserCount := 6
	users := make([]User, testUserCount)
	userMap := make(map[UserID]int)

	verified := 0
	unverified := 0
	for i := range users {
		subexit := false
		users[i], subexit = testNewUser(t)
		if subexit {
			return
		}
		userMap[users[i].UserID] = i
		if users[i].IsVerified {
			verified++
		} else {
			unverified++
		}
	}
	if verified == 0 || unverified == 0 {
		t.Errorf("Don't have a mix of verified / unverified (%d %d)", verified, unverified)
		return
	}

	// Try making an invalid discussion
	t.Logf("Trying to make invalid discussions")
	{
		err := NewDiscussion(&Discussion{Title: "", Description: "foo", Owner: users[0].UserID})
		if err == nil {
			t.Errorf("Created discussion with empty title")
			return
		}

		err = NewDiscussion(&Discussion{Title: "    ", Description: "foo", Owner: users[0].UserID})
		if err == nil {
			t.Errorf("Created discussion with whitespace title")
			return
		}

		err = NewDiscussion(&Discussion{Title: "foo", Description: "", Owner: users[0].UserID})
		if err == nil {
			t.Errorf("Created discussion with empty description")
			return
		}

		err = NewDiscussion(&Discussion{Title: "foo", Description: "    ", Owner: users[0].UserID})
		if err == nil {
			t.Errorf("Created discussion with whitespace description")
			return
		}

		disc := Discussion{Title: "foo", Description: "bar"}
		disc.Owner.generate()
		err = NewDiscussion(&disc)
		if err == nil {
			t.Errorf("Created discussion with invalid owner")
			return
		}
	}

	// Enough discussions to max out one user, plus a few for others
	testDiscussionCount := maxDiscussionsPerUser + testUserCount*2
	discussions := make([]Discussion, testDiscussionCount)

	t.Logf("Trying to max out one users's discussions")
	{
		for i := 0; i < maxDiscussionsPerUser; i++ {
			subexit := false
			discussions[i], subexit = testNewDiscussion(t, users[0].UserID)
			if subexit {
				return
			}
		}
		err := NewDiscussion(&Discussion{Title: "foo", Description: "bar", Owner: users[0].UserID})
		if err == nil {
			t.Errorf("Created too many discussions for one user")
			return
		}

	}

	t.Logf("Making some more discussions")
	public := 0
	private := 0
	for i := maxDiscussionsPerUser; i < len(discussions); i++ {
		subexit := false
		discussions[i], subexit = testNewDiscussion(t, "")
		if subexit {
			return
		}

		owneridx, prs := userMap[discussions[i].Owner]
		if !prs {
			t.Errorf("Unknown user %v!", discussions[i].Owner)
			return
		}
		owner := &users[owneridx] // Wish I had 'const'

		// owner.IsVerified <=> new discussion.IsPublic
		if owner.IsVerified != discussions[i].IsPublic {
			t.Errorf("Unexpected IsPublic: wanted %v got %v!",
				owner.IsVerified, discussions[i].IsPublic)
			return
		}

		if discussions[i].IsPublic {
			public++
			if discussions[i].ApprovedTitle != discussions[i].Title {
				t.Errorf("Discusison public, but approved title doesn't match!")
				return
			}
			if discussions[i].ApprovedDescription != discussions[i].Description {
				t.Errorf("Discusison public, but approved description doesn't match!")
				return
			}
		} else {
			private++
			if discussions[i].ApprovedTitle != "" {
				t.Errorf("Discussion private, but ApprovedTitle not empty!")
				return
			}
			if discussions[i].ApprovedDescription != "" {
				t.Errorf("Discussion private, but ApprovedDescription not empty!")
				return
			}
		}

		// Try creating a new discussionw ith the same title
		discCopy := discussions[i]
		err := NewDiscussion(&discCopy)
		if err == nil {
			t.Errorf("Created discussion with duplicate title")
			return
		}

		// Look for that discussion by did
		gotdisc, err := DiscussionFindById(discussions[i].DiscussionID)
		if err != nil {
			t.Errorf("Finding the discussion we just created by ID: %v", err)
			return
		}
		if gotdisc == nil {
			t.Errorf("Couldn't find just-created discussion by id %s!", discussions[i].DiscussionID)
			return
		}
		if !compareDiscussions(&discussions[i], gotdisc, t) {
			t.Errorf("Discussion data mismatch")
			return
		}
	}
	if public == 0 || private == 0 {
		t.Errorf("Didn't get both public and private discussions! (%d and %d)", public, private)
		return
	}

	t.Logf("Testing Corner cases")
	{
		// Try to find a non-existent ID.  Should return nil for both.
		var fakedid DiscussionID
		fakedid.generate()
		gotdisc, err := DiscussionFindById(fakedid)
		if err != nil {
			t.Errorf("Unexpected error finding non-existent discussion: %v", err)
			return
		}
		if gotdisc != nil {
			t.Errorf("Unexpectedly got non-existent discussion!")
			return
		}
	}

	t.Logf("Testing DiscussionUpdate and SetPublic")
	for i := range discussions {
		owneridx, prs := userMap[discussions[i].Owner]
		if !prs {
			t.Errorf("Unknown user %v!", discussions[i].Owner)
			return
		}
		owner := &users[owneridx] // Wish I had 'const'

		//
		// "Normal" title / discussion update
		//

		// NB at this point Approved* will all be "", so this hasn't checked an important state change yet
		copy := discussions[i]
		copy.Title = fake.Title()
		copy.Description = fake.Paragraphs()
		err := DiscussionUpdate(&copy)
		if err != nil {
			t.Errorf("Updating discussion: %v", err)
			return
		}

		gotdisc, err := DiscussionFindById(discussions[i].DiscussionID)
		if err != nil {
			t.Errorf("Unexpected error finding just-updated discussion: %v", err)
			return
		}

		if owner.IsVerified {
			copy.ApprovedTitle = copy.Title
			copy.ApprovedDescription = copy.Description
			copy.IsPublic = true
		} else {
			copy.IsPublic = false
		}

		if !compareDiscussions(gotdisc, &copy, t) {
			t.Errorf("Unexpected results after update")
			return
		}
		discussions[i] = *gotdisc

		//
		// Invert SetPublic
		//
		err = DiscussionSetPublic(discussions[i].DiscussionID, !discussions[i].IsPublic)
		if err != nil {
			t.Errorf("Fliping SetPublic: %v", err)
			return
		}

		gotdisc, err = DiscussionFindById(discussions[i].DiscussionID)
		if err != nil {
			t.Errorf("Unexpected error finding just-updated discussion: %v", err)
			return
		}

		copy = discussions[i]
		copy.IsPublic = !discussions[i].IsPublic
		if copy.IsPublic {
			copy.ApprovedTitle = copy.Title
			copy.ApprovedDescription = copy.Description
		} else {
			copy.ApprovedTitle = ""
			copy.ApprovedDescription = ""
		}

		if !compareDiscussions(gotdisc, &copy, t) {
			t.Errorf("Unexpected results after update")
			return
		}
		discussions[i] = *gotdisc

		//
		// Second "Normal" title / discussion update
		//

		// NB at this point, some unverified owners will have a non-empty "approved" title
		copy = discussions[i]
		copy.Title = fake.Title()
		copy.Description = fake.Paragraphs()
		err = DiscussionUpdate(&copy)
		if err != nil {
			t.Errorf("Updating discussion: %v", err)
			return
		}

		gotdisc, err = DiscussionFindById(discussions[i].DiscussionID)
		if err != nil {
			t.Errorf("Unexpected error finding just-updated discussion: %v", err)
			return
		}

		if owner.IsVerified {
			copy.ApprovedTitle = copy.Title
			copy.ApprovedDescription = copy.Description
			copy.IsPublic = true
		} else {
			copy.IsPublic = false
		}

		if !compareDiscussions(gotdisc, &copy, t) {
			t.Errorf("Unexpected results after update")
			return
		}
		discussions[i] = *gotdisc

		//
		// Change owner
		//
		copy = discussions[i]
		owner, err = UserFindRandom()
		if err != nil {
			t.Errorf("Finding a random user: %v", err)
			return
		}
		copy.Owner = owner.UserID
		err = DiscussionUpdate(&copy)
		if err != nil {
			t.Errorf("Updating discussion: %v", err)
			return
		}

		gotdisc, err = DiscussionFindById(discussions[i].DiscussionID)
		if err != nil {
			t.Errorf("Unexpected error finding just-updated discussion: %v", err)
			return
		}

		if owner.IsVerified {
			copy.ApprovedTitle = copy.Title
			copy.ApprovedDescription = copy.Description
			copy.IsPublic = true
		} else {
			copy.IsPublic = false
		}

		if !compareDiscussions(gotdisc, &copy, t) {
			t.Errorf("Unexpected results after update")
			return
		}
		discussions[i] = *gotdisc

		//
		// Bad input: No title, no description
		//
		for _, newTitle := range []string{"", "   "} {
			copy = discussions[i]
			copy.Title = newTitle
			err = DiscussionUpdate(&copy)
			if err == nil {
				t.Errorf("Updating discussion with empty title (%s) succeeded!", newTitle)
				return
			}
		}

		for _, newDesc := range []string{"", "   "} {
			copy = discussions[i]
			copy.Description = newDesc
			err = DiscussionUpdate(&copy)
			if err == nil {
				t.Errorf("Updating discussion with empty description (%s) succeeded!", newDesc)
				return
			}
		}

		gotdisc, err = DiscussionFindById(discussions[i].DiscussionID)
		if err != nil {
			t.Errorf("Unexpected error finding just-updated discussion: %v", err)
			return
		}

		// Nothing should have changed
		if !compareDiscussions(gotdisc, &discussions[i], t) {
			t.Errorf("Unexpected results after update")
			return
		}

		//
		// Bad input: Things that shouldn't be changed
		//
		copy = discussions[i]
		copy.ApprovedTitle = fake.Title()
		copy.ApprovedDescription = fake.Paragraphs()
		copy.IsPublic = true
		err = DiscussionUpdate(&copy)
		if err != nil {
			t.Errorf("Updating discussion: %v", err)
			return
		}

		gotdisc, err = DiscussionFindById(discussions[i].DiscussionID)
		if err != nil {
			t.Errorf("Unexpected error finding just-updated discussion: %v", err)
			return
		}

		// Nothing should have changed
		if !compareDiscussions(gotdisc, &discussions[i], t) {
			t.Errorf("Unexpected results after update")
			return
		}
	}

	// Sort the discussions by userid so they're in the same order as DiscussionIterate
	sort.Slice(discussions, func(i, j int) bool {
		return discussions[i].DiscussionID < discussions[j].DiscussionID
	})

	t.Logf("Testing DiscussionIterate")
	{
		i := 0
		err := DiscussionIterate(func(d *Discussion) error {
			if !compareDiscussions(&discussions[i], d, t) {
				return fmt.Errorf("DiscussionIterate mismatch")
			}
			i++
			return nil
		})
		if err != nil {
			t.Errorf("DiscussionIterate error: %v", err)
			return
		}
		if i != len(discussions) {
			t.Errorf("DiscussionIterate: expected %d function calls, got %d!", len(users), i)
		}
	}

	t.Logf("Testing DiscussionIterate error reporting")
	{
		i := 0
		err := DiscussionIterate(func(d *Discussion) error {
			if !compareDiscussions(&discussions[i], d, t) {
				return fmt.Errorf("DiscussionIterate mismatch")
			}
			i++

			if i > 3 {
				return ErrInternal
			}
			return nil
		})
		if err != ErrInternal {
			t.Errorf("DiscussionIterate error handling: wanted %v, got %v", ErrInternal, err)
			return
		}
	}

	t.Logf("Testing DiscussionIterateUser")
	{
		dcount := make(map[UserID]int)
		for didx := range discussions {
			dcount[discussions[didx].Owner]++
		}

		for uidx := range users {
			uid := users[uidx].UserID
			i := 0
			err := DiscussionIterateUser(uid, func(d *Discussion) error {
				if d.Owner != uid {
					return fmt.Errorf("Got user %v, expecting %v!", d.Owner, uid)
				}
				i++
				return nil
			})
			if err != nil {
				t.Errorf("DiscussionIterateUser(%v) error: %v", uid, err)
				return
			}
			if i != dcount[uid] {
				t.Errorf("DiscussionIterateUser(%v): expected %d function calls, got %d!", uid, len(users), i)
			}
		}
	}

	t.Logf("Testing DeleteDiscussion / DeleteUsers with discussions")
	// General approach:
	//
	// 1. Delete one discussion from each user.  Do all the normal "did it disappear" &c checks.
	//
	// 2. Delete the whole user.  Then run DiscussionIterateUser() and verify that nothing happens.
	//
	// Afterwards, iterate through our list of discussions and verify
	// that they're all gone.  (And also run DiscussionIterate again
	// for good measure.)
	for uidx := range users {
		uid := users[uidx].UserID

		// First, find one discussion to delete from this user
		var did DiscussionID
		{
			i := 0
			stopErr := fmt.Errorf("Done")
			err := DiscussionIterateUser(uid, func(d *Discussion) error {
				if d.Owner != uid {
					return fmt.Errorf("Got user %v, expecting %v!", d.Owner, uid)
				}
				if i == 0 {
					did = d.DiscussionID
					i++
					return nil
				}
				return stopErr
			})
			// Valid results:
			// err == nil, i == 0: User has no discussions
			// err == nil, i == 1: User has exactly one discussion
			// err == stopErr: User has more than one discussion, and we correctly stopped
			if !((err == stopErr) || (i < 2 && err == nil)) {
				t.Errorf("DiscussionIterateUser(%v): unexpected error %v", uid, err)
				return
			}
		}

		if did != "" {
			// If we have a discussion, delete it
			err := DeleteDiscussion(did)
			if err != nil {
				t.Errorf("Deleting discussion(%v): %v", did, err)
				return
			}

			// Try finding the discussion
			gotdisc, err := DiscussionFindById(did)
			if err != nil {
				t.Errorf("Finding deleted discussion: %v", err)
				return
			}
			if gotdisc != nil {
				t.Errorf("Got response from deleted discussion: %v", gotdisc)
				return
			}

			// Try deleting it again
			err = DeleteDiscussion(did)
			if err == nil {
				t.Errorf("DeleteDiscussion a second time succeeded!")
				return
			}
		}

		// Now, delete the user
		{
			err := DeleteUser(uid)
			if err != nil {
				t.Errorf("DeleteUser(%v): %v", uid, err)
				return
			}

			err = DiscussionIterateUser(uid, func(d *Discussion) error {
				return fmt.Errorf("Shouldn't be called!")
			})
			if err != nil {
				t.Errorf("DiscussionIterateUser for deleted user %v: %v", uid, err)
				return
			}
		}
	}

	for didx := range discussions {
		gotdisc, err := DiscussionFindById(discussions[didx].DiscussionID)
		if err != nil {
			t.Errorf("DiscussionFindById for (allegedly)-deleted discussion: %v", err)
			return
		}
		if gotdisc != nil {
			t.Errorf("DiscussionFindById succeeded for allegedly-deleted discussion!")
			return
		}
	}

	{
		err := DiscussionIterate(func(d *Discussion) error {
			return fmt.Errorf("Shouldn't be called!")
		})
		if err != nil {
			t.Errorf("DiscussionIterate when all discussions should have been deleted: %v", err)
			return
		}
	}

	tc.cleanup()

	return false
}

// DONE:
// NewDiscussion
// DiscussionFindById
// DiscussionUpdate
// DiscussionSetPublic
// DiscussionIterate
// DiscussionIterateUser

// IN PROGRESS
// DeleteDiscussion
//  - Deleting a discussion
//  - Deleting a user that has discussions
