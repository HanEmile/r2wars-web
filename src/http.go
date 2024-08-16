package main

import (
	"html/template"
	"net/http"
)

type Link struct {
	Name   string
	Target string
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		// define data
		data := map[string]interface{}{}
		data["pagelink1"] = ""
		data["pagelinknext"] = []Link{
			{Name: "user/", Target: "/user"},
			{Name: "bot/", Target: "/bot"},
			{Name: "battle/", Target: "/battle"},
		}
		data["pagelinkauth"] = []Link{
			{Name: "login/", Target: "/login"},
			{Name: "register/", Target: "/register"},
		}

		session, _ := globalState.sessions.Get(r, "session")
		username := session.Values["username"]

		if username != nil {
			data["logged_in"] = true

			user, err := UserGetUserFromUsername(username.(string))
			if err != nil {
				data["err"] = "Couln't get the user"
			}
			data["user"] = user
		}

		// get the template
		t, err := template.ParseGlob("./templates/*.html")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - Error reading template file"))
			return
		}

		// exec!
		t.ExecuteTemplate(w, "index", data)
	default:
		http.Redirect(w, r, "/", http.StatusMethodNotAllowed)
	}
}
