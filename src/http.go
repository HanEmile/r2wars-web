package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
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
		data["version"] = os.Getenv("VERSION")
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
		t, err := template.ParseGlob(fmt.Sprintf("%s/*.html", templatesPath))
		if err != nil {
			log.Printf("Error reading the template Path: %s/*.html", templatesPath)
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
