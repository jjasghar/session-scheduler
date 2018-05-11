package main

import (
	"fmt"
	"sort"
)

// *ScheduleDisplay: Leftover from previous -- may be useful later
type UserScheduleDisplay struct {
	Username string
	Interest int
}

type DiscussionScheduleDisplay struct {
	ID DiscussionID
	Title string
	Description string
	Url string
	Attending []UserScheduleDisplay
	Missing []UserScheduleDisplay
}


type TimetableDiscussion struct {
	ID DiscussionID
	Title string
	Attendees int
	Score int

	// Users and their interest
	Attending []*User
}

type TimetableSlot struct {
	Time string
	IsBreak bool

	// Which room will each discussion be in?
	// (Separate because placement and scheduling are separate steps)
	Discussions []TimetableDiscussion

	// Link to the "real" slot
	slot *Slot
}

type TimetableDay struct {
	DayName string
	// Date?
	
	Slots []*TimetableSlot
}

// Placement: Specific days, times, rooms
type Timetable struct {
	Days []*TimetableDay
}


func (tt *Timetable) Init() {
	// For now, hardcode 3 days (W Th F), 3 time slots
	// 4 locations: 
	for _, day := range []string{"Wednesday", "Thursday", "Friday"} {
		td := &TimetableDay{ DayName: day }
		for _, time := range []string{"1:50", "2:40", "3:25", "4:00"} {
			ts := &TimetableSlot{ Time: time }
			if time == "3:25" {
				ts.IsBreak = true
			}
			// FIXME: Init Discussions
			td.Slots = append(td.Slots, ts)
		}
		tt.Days = append(tt.Days, td)
	}
}

func (tt *Timetable) GetSlots() int {
	count := 0
	for _, td := range tt.Days {
		for _, ts := range td.Slots {
			if !ts.IsBreak {
				count++
			}
		}
	}
	return count
}

// Take a "Schedule" (consisting only of slots arranged for minimal
// conflicts) and map it into a "Timetable" (consisting of actual
// times and locations)
func (tt *Timetable) Place(sched *Schedule) (err error) {
	ttSlots := tt.GetSlots()
	if len(sched.Slots) != ttSlots {
		err = fmt.Errorf("Internal error: Schedule slots %d, timetable slots %d!",
			len(sched.Slots), ttSlots)
		return
	}

	count := 0
	for _, td := range tt.Days {
		for _, ts := range td.Slots {
			if ts.IsBreak {
				continue
			}

			slot := sched.Slots[count]

			// For now, just list the discussions.  Place into locations later.
			ts.Discussions = []TimetableDiscussion{}
			for did := range slot.Discussions {
				disc, _ := Event.Discussions.Find(did)
				tdisc := TimetableDiscussion{
					ID: did,
					Title: disc.Title,
					Attendees: slot.DiscussionAttendeeCount(did),
				}
				tdisc.Score, _ = slot.DiscussionScore(did)

				ts.Discussions = append(ts.Discussions, tdisc)
			}

			// Sort by number of attendees
			sort.Slice(ts.Discussions, func(i, j int) bool {
				return ts.Discussions[i].Attendees > ts.Discussions[j].Attendees
			})

			count++
		}
	}
	
	return
}