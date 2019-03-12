package main

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/hako/durafmt"
	"github.com/julienschmidt/httprouter"
)

func HandleAdminConsole(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	user := RequestUser(r)

	if user == nil || !user.IsAdmin {
		return
	}

	content := map[string]interface{}{"User": user}

	tmpl := ps.ByName("template")
	switch tmpl {
	case "console":
		content["Vcode"] = Event.VerificationCode
		lastUpdate := "Never"
		if Event.ScheduleV2 != nil {
			lastUpdate = durafmt.ParseShort(time.Since(Event.ScheduleV2.Created)).String() + " ago"
		}
		content["SinceLastSchedule"] = lastUpdate
		switch {
		case Event.ScheduleState.IsRunning():
			content["IsInProgress"] = true
		case Event.ScheduleState.IsModified():
			content["IsStale"] = true
		default:
			content["IsCurrent"] = true
		}
		if Event.LockedSlots != nil {
			content["LockedSlots"] = Event.Timetable.FillDisplaySlots(Event.LockedSlots)
		}
		fallthrough
	case "test":
		content[tmpl] = true
		RenderTemplate(w, r, "admin/"+tmpl, content)
	}

}

func HandleAdminAction(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	user := RequestUser(r)

	if user == nil || !user.IsAdmin {
		return
	}

	action := ps.ByName("action")
	if !(action == "runschedule" ||
		action == "setvcode" ||
		action == "setstatus" ||
		action == "resetEventData" ||
		action == "setLocked") {
		return
	}

	switch action {
	case "runschedule":
		err := MakeSchedule(SearchAlgo(OptSearchAlgo), true)
		if err == nil {
			http.Redirect(w, r, "console?flash=Schedule+Started", http.StatusFound)
		} else {
			log.Printf("Error generating schedule: %v", err)
			http.Redirect(w, r, "console?flash=Error+starting+schedule", http.StatusFound)
		}
	case "resetEventData":
		Event.ResetEventData()
		http.Redirect(w, r, "console?flash=Event+data+reset", http.StatusFound)
		return
	case "setvcode":
		newvcode := r.FormValue("vcode")
		if newvcode == "" {
			RenderTemplate(w, r, "console?flash=Invalid+Vcode",
				map[string]interface{}{
					"User":    user,
					"console": true,
					"Vcode":   Event.VerificationCode,
				})
			return
		}

		log.Printf("New vcode: %s", newvcode)
		Event.VerificationCode = newvcode
		Event.Save()
		http.Redirect(w, r, "console?flash=Verification+code+updated", http.StatusFound)
		return
	case "setstatus":
		r.ParseForm()
		statuses := r.Form["status"]
		flash := ""
		newval := map[string]bool{"website": false, "schedule": false, "verification": false}
		for _, status := range statuses {
			switch status {
			case "websiteActive":
				newval["website"] = true
			case "scheduleActive":
				newval["schedule"] = true
			case "requireVerification":
				newval["verification"] = true
			default:
				log.Printf("Unexpected status value: %v", status)
				flash = "Invalid form result: Report this error to the admin"
				http.Redirect(w, r, "console?flash="+flash, http.StatusFound)
				return
			}
		}
		if newval["website"] != Event.Active {
			Event.Active = newval["website"]
			if Event.Active {
				flash += "Website+Activated"
			} else {
				flash += "Website+Deactivated"
			}
		}
		if newval["schedule"] != Event.ScheduleActive {
			Event.ScheduleActive = newval["schedule"]
			if flash != "" {
				flash += ", "
			}
			if Event.ScheduleActive {
				flash += "Schedule+Activated"
			} else {
				flash += "Schedule+Deactivated"
			}
		}
		if newval["verification"] != Event.RequireVerification {
			Event.RequireVerification = newval["verification"]
			if flash != "" {
				flash += ", "
			}
			if Event.RequireVerification {
				flash += "Verification+Required"
			} else {
				flash += "Verificaiton+Not+Required"
			}
		}

		log.Printf("New state: Active %v ScheduleActive %v RequireVerification $%v",
			Event.Active, Event.ScheduleActive, Event.RequireVerification)
		Event.Save()
		http.Redirect(w, r, "console?flash="+flash, http.StatusFound)
		return
	case "setLocked":
		r.ParseForm()
		locked, err := FormCheckToBool(r.Form["locked"])
		if err != nil {
			return
		}
		log.Printf("New locked slots: %v", locked)
		Event.LockedSlots.Set(locked)
		Event.Save()
		http.Redirect(w, r, "console?flash=Locked+slots+updated", http.StatusFound)
	}
}

func HandleTestAction(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	user := RequestUser(r)

	if user == nil || !user.IsAdmin {
		return
	}

	flash := "Success"

	action := ps.ByName("action")

	if !Event.TestMode {
		if action != "enabletest" {
			return
		}

		if r.FormValue("confirm") != "SafetyOff" {
			RenderTemplate(w, r, "admin/test", map[string]interface{}{
				"MustConfirm": true,
			})
			return
		}

		Event.TestMode = true
		Event.Save()

		flash = "Test+mode+enabled"
	} else {
		switch action {
		case "disabletest":
			Event.TestMode = false
			Event.Save()
			flash = "Test+mode+disabled"
		case "enabletest":
			flash = "Test+mode+already+disabled"
		case "resetUserData":
			Event.ResetUserData()
			flash = "Data+reset"
		case "genuser":
			countString := r.FormValue("count")
			count, err := strconv.Atoi(countString)
			if err != nil || !(count > 0) {
				flash = "Bad+input"
			} else {
				for i := 0; i < count; i++ {
					NewTestUser()
				}
				flash = countString + " users generated"
			}
		case "gendiscussion":
			countString := r.FormValue("count")
			count, err := strconv.Atoi(countString)
			if err != nil || !(count > 0) {
				flash = "Bad+input"
			} else {
				for i := 0; i < count; i++ {
					NewTestDiscussion(nil)
				}
				flash = countString + " discussions generated"
			}
		case "geninterest":
			TestGenerateInterest()
			flash = "Interest generated"
		default:
			return
		}
	}

	http.Redirect(w, r, "/admin/test?flash="+flash, http.StatusFound)
}
