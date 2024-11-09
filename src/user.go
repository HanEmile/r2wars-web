package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/crypto/argon2"
)

type User struct {
	ID           int
	Name         string
	PasswordHash []byte
}

//////////////////////////////////////////////////////////////////////////////
// GENERAL PURPOSE

func UserRegister(username string, passwordHash []byte) (int, error) {
	id, err := globalState.InsertUser(User{Name: username, PasswordHash: passwordHash})
	if err != nil {
		return 0, err
	} else {
		return id, nil
	}
}

// UserCheckPasswordHash returns a boolean that is true if the users password
// is correct and false if the users password is false and thus doesn't match
// the one stored in the database
func UserCheckPasswordHash(username string, passwordHash []byte) bool {
	return globalState.CheckUserHash(username, passwordHash)
}

// UserUpdatePasswordHash does exactly that
func UserUpdatePasswordHash(orig_username string, passwordHash []byte) error {
	return globalState.UpdateUserPasswordHash(orig_username, passwordHash)
}

func UserUpdateUsername(id int, new_username string) error {
	return globalState.UpdateUserUsername(id, new_username)
}

func UserLinkBot(username string, botid int) error {
	return globalState.LinkUserBot(username, botid)
}

func UserGetBotsUsingUsername(username string) ([]Bot, error) {
	return globalState.GetUserBotsUsername(username)
}

func UserGetBotsUsingUserID(userid int) ([]Bot, error) {
	return globalState.GetUserBotsId(userid)
}

func UserGetUserFromID(userid int) (User, error) {
	return globalState.GetUserFromId(userid)
}

func UserGetUserFromUsername(username string) (User, error) {
	return globalState.GetUserFromUsername(username)
}

func UserGetAll() ([]User, error) {
	return globalState.GetAllUsers()
}

func UserGetUsernameCount(username string) (int, error) {
	return globalState.GetUsernameCount(username)
}

//////////////////////////////////////////////////////////////////////////////
// DATABASE

func (s *State) InsertUser(user User) (int, error) {
	res, err := s.db.Exec("INSERT INTO users VALUES(NULL,?,?,?);", time.Now(), user.Name, user.PasswordHash)
	if err != nil {
		return 0, err
	}

	var id int64
	if id, err = res.LastInsertId(); err != nil {
		return 0, err
	}
	return int(id), nil
}

// returns true if the password matches
func (s *State) CheckUserHash(username string, passwordHash []byte) bool {
	var created time.Time
	err := s.db.QueryRow("SELECT created_at FROM users WHERE name=? AND passwordHash=?", username, passwordHash).Scan(&created)
	switch {
	case err != nil:
		return false
	default:
		return true
	}
}

func (s *State) UpdateUserPasswordHash(username string, passwordHash []byte) error {
	_, err := s.db.Exec("UPDATE users SET passwordHash=? WHERE name=?", passwordHash, username)
	if err != nil {
		return err
	} else {
		return nil
	}
}

func (s *State) UpdateUserUsername(id int, new_username string) error {
	_, err := s.db.Exec("UPDATE users SET name=? WHERE id=?", new_username, id)
	if err != nil {
		return err
	} else {
		return nil
	}
}

// Links the given bot to the given user in the user_bot_rel table
func (s *State) LinkUserBot(username string, botid int) error {
	_, err := s.db.Exec(`
		INSERT INTO user_bot_rel
		VALUES ((SELECT id FROM users WHERE name=?), ?)`, username, botid)
	if err != nil {
		return err
	} else {
		return nil
	}
}

// Links the given bot to the given user in the user_bot_rel table
func (s *State) GetUserFromId(id int) (User, error) {
	var user_id int
	var username string
	err := s.db.QueryRow("SELECT id, name FROM users WHERE id=?", id).Scan(&user_id, &username)
	if err != nil {
		return User{}, err
	} else {
		return User{user_id, username, nil}, nil
	}
}

func (s *State) GetUserFromUsername(username string) (User, error) {
	var id int
	var name string
	err := s.db.QueryRow("SELECT id, name FROM users WHERE name=?", username).Scan(&id, &name)
	if err != nil {
		return User{}, err
	} else {
		return User{id, name, nil}, nil
	}
}

// Returns the bots belonging to the given user
// TODO(emile): Also fetch the bits and the archs for displaying in the single battle page. In order to do so, join in both those tables
func (s *State) GetUserBotsUsername(username string) ([]Bot, error) {
	rows, err := s.db.Query(`
		SELECT id, name, source
		FROM bots b
		LEFT JOIN user_bot_rel ub ON ub.bot_id = b.id
		WHERE ub.user_id=(SELECT id FROM users WHERE name=?)`, username)
	defer rows.Close()
	if err != nil {
		return nil, err
	}

	var bots []Bot
	for rows.Next() {
		var bot Bot
		if err := rows.Scan(&bot.ID, &bot.Name, &bot.Source); err != nil {
			return bots, err
		}
		bots = append(bots, bot)
	}
	if err = rows.Err(); err != nil {
		return bots, err
	}
	return bots, nil
}

// Returns the bots belonging to the given user
func (s *State) GetUserBotsId(id int) ([]Bot, error) {
	rows, err := s.db.Query(`
		SELECT id, name, source
		FROM bots b
		LEFT JOIN user_bot_rel ub ON ub.bot_id = b.id
		WHERE ub.user_id=?`, id)
	defer rows.Close()
	if err != nil {
		return nil, err
	}

	var bots []Bot
	for rows.Next() {
		var bot Bot
		if err := rows.Scan(&bot.ID, &bot.Name, &bot.Source); err != nil {
			return bots, err
		}
		bots = append(bots, bot)
	}
	if err = rows.Err(); err != nil {
		return bots, err
	}
	return bots, nil
}

// Returns the bots belonging to the given user
func (s *State) GetAllUsers() ([]User, error) {
	rows, err := s.db.Query("SELECT id, name FROM users")
	defer rows.Close()
	if err != nil {
		return nil, err
	}

	var users []User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Name); err != nil {
			return users, err
		}
		users = append(users, user)
	}
	if err = rows.Err(); err != nil {
		return users, err
	}
	return users, nil
}

func (s *State) GetUsernameCount(username string) (int, error) {
	rows, err := s.db.Query(`
		SELECT COUNT(*)
		FROM users
		WHERE name=?`, username)
	defer rows.Close()
	if err != nil {
		return -1, err
	}

	var count int
	rows.Next()
	if err := rows.Scan(&count); err != nil {
		return -1, err
	}
	if err = rows.Err(); err != nil {
		return -1, err
	}
	return count, nil
}

//  return globalState.GetUsernameCount(username)

//////////////////////////////////////////////////////////////////////////////
// HTTP

func loginHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		// define data
		data := map[string]interface{}{}
		data["version"] = os.Getenv("VERSION")
		data["pagelink1"] = Link{"login", "/login"}
		data["pagelink1options"] = []Link{
			{Name: "register", Target: "/register"},
		}
		data["pagelinkauth"] = []Link{
			{Name: "register/", Target: "/register"},
		}

		// session foo
		session, _ := globalState.sessions.Get(r, "session")
		username := session.Values["username"]

		// get the user
		if username != nil {
			log.Printf("[d] Getting the user %s\n", username.(string))
			user, err := UserGetUserFromUsername(username.(string))
			if user.Name == "" {
			} else if err != nil {
				log.Println(err)
				msg := "Error: could not get the user for given username"
				http.Redirect(w, r, fmt.Sprintf("/login?res=%s", msg), http.StatusSeeOther)
				return
			} else {
				data["user"] = user
			}
		}

		// display errors passed via query parameters
		queryres := r.URL.Query().Get("res")
		if queryres != "" {
			data["res"] = queryres
		}

		// get the template
		t, err := template.ParseGlob(fmt.Sprintf("%s/*.html", templatesPath))
		if err != nil {
			log.Printf("Error reading the template Path: %s/*.html", templatesPath)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - Error reading template file"))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// exec!
		t.ExecuteTemplate(w, "login", data)

	case "POST":
		// parse the post parameters
		r.ParseForm()
		username := r.Form.Get("username")
		password := r.Form.Get("password")

		// if we've got a password, hash it and compare it with the stored one
		if password != "" {
			passwordHash := argon2.IDKey([]byte(password), []byte(os.Getenv("SALT")), 1, 64*1024, 4, 32)

			// check if it's valid
			valid := UserCheckPasswordHash(username, passwordHash)
			if valid {

				// if it's valid, we set a session for the user
				session, _ := globalState.sessions.Get(r, "session")
				session.Values["username"] = username
				err := session.Save(r, w)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				http.Redirect(w, r, "/", http.StatusSeeOther)
				return

			} else {
				// invalid password
				http.Redirect(w, r, "/login?err=Invalid+Password", http.StatusSeeOther)
				return
			}
		} else {
			// empty password
			http.Redirect(w, r, "/login?err=Empty+Password", http.StatusSeeOther)
			return
		}
	default:
		http.Redirect(w, r, "/", http.StatusMethodNotAllowed)
	}
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		// define data
		data := map[string]interface{}{}
		data["version"] = os.Getenv("VERSION")

		// get the session
		session, _ := globalState.sessions.Get(r, "session")
		username := session.Values["username"]
		data["pagelink1"] = Link{"register", "/register"}
		data["pagelink1options"] = []Link{
			{Name: "login", Target: "/login"},
		}
		data["pagelinkauth"] = []Link{
			{Name: "login/", Target: "/login"},
		}

		if username != nil {
			data["logged_in"] = true
		}

		// get the template
		t, err := template.ParseGlob(fmt.Sprintf("%s/*.html", templatesPath))
		if err != nil {
			log.Printf("Error reading the template Path: %s/*.html", templatesPath)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - Error reading template file"))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// exec!
		t.ExecuteTemplate(w, "register", data)

	case "POST":
		// parse the post parameters
		r.ParseForm()
		username := r.Form.Get("username")
		password1 := r.Form.Get("password1")
		password2 := r.Form.Get("password2")

		if len(username) >= 64 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - Oi', Backend here! Please enter less than 64 chars!"))
			return
		}

		if len(password1) >= 256 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - Oi', Backend here! Don't overdo with the length please!"))
			return
		}

		if password1 != password2 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - Oi', Backend here! The passwords you entered don't match!"))
			return
		}

		// Fetch all users and check that there isn't another user with the name here
		if _, err := UserGetUsernameCount(username); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - Oi', Backend here! That username has already been taken!"))
			return
		}

		// if we've got a password, hash it and store it and create a User
		if password1 != "" {
			passwordHash := argon2.IDKey([]byte(password1), []byte(os.Getenv("SALT")), 1, 64*1024, 4, 32)

			_, err := UserRegister(username, passwordHash)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("500 - We had problems inserting you into the DB"))
				return
			}

			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
	default:
		http.Redirect(w, r, "/", http.StatusMethodNotAllowed)
	}
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		session, _ := globalState.sessions.Get(r, "session")
		session.Options.MaxAge = -1
		err := session.Save(r, w)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	default:
		http.Redirect(w, r, "/", http.StatusMethodNotAllowed)
	}
}

func userHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Redirect(w, r, "/user", http.StatusSeeOther)
	}

	switch r.Method {
	case "GET":
		// define data
		data := map[string]interface{}{}
		data["version"] = os.Getenv("VERSION")
		data["pagelink1"] = Link{"user", "/user"}
		data["pagelink1options"] = []Link{
			{Name: "bot", Target: "/bot"},
			{Name: "battle", Target: "/battle"},
		}

		// session foo
		session, _ := globalState.sessions.Get(r, "session")
		username := session.Values["username"].(string)

		// the the user making the request
		user, err := UserGetUserFromUsername(username)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		} else {
			data["user"] = user
		}

		// get the target user using the provided id
		targetUser, err := UserGetUserFromID(id)
		if err != nil {
			data["err"] = "Could not find that user"
		} else {
			data["targetUser"] = targetUser
		}

		// define the breadcrumbs
		data["pagelink2"] = Link{targetUser.Name, fmt.Sprintf("/%s", targetUser.Name)}

		allUserNames, err := UserGetAll()
		var opts []Link
		for _, user := range allUserNames {
			opts = append(opts, Link{Name: user.Name, Target: fmt.Sprintf("/%d", user.ID)})
		}
		data["pagelink2options"] = opts

		// get the bots for the given user
		bots, err := UserGetBotsUsingUserID(id)
		if err != nil {
			http.Redirect(w, r, "/user", http.StatusSeeOther)
		} else {
			data["bots"] = bots
		}

		// get the template
		t, err := template.ParseGlob(fmt.Sprintf("%s/*.html", templatesPath))
		if err != nil {
			log.Printf("Error reading the template Path: %s/*.html", templatesPath)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - Error reading template file"))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// exec!
		t.ExecuteTemplate(w, "user", data)
	default:
		http.Redirect(w, r, "/", http.StatusMethodNotAllowed)
	}
}

func usersHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		// define data
		data := map[string]interface{}{}
		data["version"] = os.Getenv("VERSION")
		data["pagelink1"] = Link{Name: "user", Target: "/user"}
		data["pagelink1options"] = []Link{
			{Name: "bot", Target: "/bot"},
			{Name: "battle", Target: "/battle"},
		}

		// sessions
		session, _ := globalState.sessions.Get(r, "session")
		username := session.Values["username"].(string)

		// get the user
		user, err := UserGetUserFromUsername(username)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		} else {
			data["user"] = user
		}

		// get all users
		users, err := UserGetAll()
		data["users"] = users

		// get the template
		t, err := template.ParseGlob(fmt.Sprintf("%s/*.html", templatesPath))
		if err != nil {
			log.Printf("Error reading the template Path: %s/*.html", templatesPath)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - Error reading template file"))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// exec!
		t.ExecuteTemplate(w, "users", data)
	default:
		http.Redirect(w, r, "/", http.StatusMethodNotAllowed)
	}
}

func profileHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("500 - Error reading the profile id"))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	switch r.Method {
	case "GET":
		// define data
		data := map[string]interface{}{}
		data["version"] = os.Getenv("VERSION")
		data["pagelink1"] = Link{"user", "/user"}
		data["pagelink1options"] = []Link{
			{Name: "bot", Target: "/bot"},
			{Name: "battle", Target: "/battle"},
		}

		session, _ := globalState.sessions.Get(r, "session")
		username := session.Values["username"].(string)

		target_user, err := UserGetUserFromID(id)
		if err != nil {
			// w.WriteHeader(http.StatusUnauthorized)
			// w.Write([]byte("500 - Error reading template file"))
			// http.Error(w, err.Error(), http.StatusInternalServerError)
			data["err"] = "Error getting with the given id"
		}

		if username != target_user.Name {
			// w.WriteHeader(http.StatusInternalServerError)
			// w.Write([]byte("500 - Error reading template file"))
			// http.Error(w, err.Error(), http.StatusInternalServerError)
			data["err"] = "You aren't allowed to edit any user except yourself"
		}

		editing_user, err := UserGetUserFromUsername(username)
		if err != nil {
			data["err"] = "Coulnd't get a user for that id"
		}

		data["user"] = editing_user
		data["target_user"] = target_user

		data["pagelink2"] = Link{target_user.Name, fmt.Sprintf("/%d", id)}
		allUserNames, err := UserGetAll()
		var opts []Link
		for _, user := range allUserNames {
			opts = append(opts, Link{Name: user.Name, Target: fmt.Sprintf("/%d", user.ID)})
		}
		data["pagelink2options"] = opts

		data["pagelink3"] = Link{"profile", "/profile"}

		if username != "" {
			data["username"] = username
		}

		// get the template
		t, err := template.ParseGlob(fmt.Sprintf("%s/*.html", templatesPath))
		if err != nil {
			log.Printf("Error reading the template Path: %s/*.html", templatesPath)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - Error reading template file"))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// exec!
		t.ExecuteTemplate(w, "profile", data)

	case "POST":
		session, _ := globalState.sessions.Get(r, "session")
		orig_username := session.Values["username"].(string)

		// parse the post parameters
		r.ParseForm()
		new_username := r.Form.Get("username")
		password1 := r.Form.Get("password1")
		password2 := r.Form.Get("password2")

		if len(new_username) >= 64 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - Oi', Backend here! Please enter less than 64 chars!"))
			return
		}

		if len(password1) >= 256 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - Oi', Backend here! Don't overdo with the length please!"))
			return
		}

		if password1 != password2 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - Oi', Backend here! The passwords you entered don't match!"))
			return
		}

		// first update the password, as they might have also changed their
		// username
		if password1 != "" {
			passwordHash := argon2.IDKey([]byte(password1), []byte(os.Getenv("SALT")), 1, 64*1024, 4, 32)

			err := UserUpdatePasswordHash(orig_username, passwordHash)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("500 - We had problems inserting your new pw into the DB"))
				return
			}
		}

		if new_username != "" {
			err := UserUpdateUsername(id, new_username)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("500 - We had problems inserting your new uname into the DB"))
				return
			}

			// after changing the username, we also have to update the username
			//  stored in the session
			session.Values["username"] = new_username
			err = session.Save(r, w)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		http.Redirect(w, r, fmt.Sprintf("/user/%d/profile", id), http.StatusSeeOther)
		return
	default:
		http.Redirect(w, r, "/", http.StatusMethodNotAllowed)
	}
}
