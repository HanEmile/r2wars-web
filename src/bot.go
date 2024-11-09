package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/radareorg/r2pipe-go"
)

type Bot struct {
	ID     int
	Name   string
	Source string
	Users  []User

	Archs []Arch
	Bits  []Bit
}

//////////////////////////////////////////////////////////////////////////////
// GENERAL PURPOSE

func BotCreate(name string, source string) (int, error) {
	return globalState.InsertBot(Bot{Name: name, Source: source})
}

func BotUpdate(botid int, name string, source string) error {
	return globalState.UpdateBot(Bot{ID: botid, Name: name, Source: source})
}

func BotGetById(id int) (Bot, error) {
	return globalState.GetBotById(id)
}

func BotGetAll() ([]Bot, error) {
	return globalState.GetAllBot()
}

func BotLinkArchIDs(botid int, archIDs []int) error {
	return globalState.LinkArchIDsToBot(botid, archIDs)
}

func BotLinkBitIDs(botid int, bitIDs []int) error {
	return globalState.LinkBitIDsToBot(botid, bitIDs)
}

//////////////////////////////////////////////////////////////////////////////
// DATABASE

func (s *State) InsertBot(bot Bot) (int, error) {
	res, err := s.db.Exec("INSERT INTO bots VALUES(NULL,?,?,?);", time.Now(), bot.Name, bot.Source)
	if err != nil {
		return 0, err
	}

	var id int64
	if id, err = res.LastInsertId(); err != nil {
		return 0, err
	}
	return int(id), nil
}

func (s *State) UpdateBot(bot Bot) error {
	_, err := s.db.Exec("UPDATE bots SET name=?, source=? WHERE id=?", bot.Name, bot.Source, bot.ID)
	if err != nil {
		return err
	}
	return nil
}

func (s *State) GetBotById(id int) (Bot, error) {
	var botid int
	var botname string
	var botsource string

	var ownerids string
	var ownernames string

	var archids string
	var archnames string

	var bitids string
	var bitnames string

	err := s.db.QueryRow(`
	SELECT
		bo.id, bo.name, bo.source,
		COALESCE(group_concat(ub.user_id), ""),
		COALESCE(group_concat(us.name), ""),
		COALESCE(group_concat(ab.arch_id), ""),
		COALESCE(group_concat(ar.name), ""),
		COALESCE(group_concat(bb.bit_id), ""),
		COALESCE(group_concat(bi.name), "")
	FROM bots bo

	LEFT JOIN user_bot_rel ub ON ub.bot_id = bo.id
	LEFT JOIN users us ON us.id = ub.user_id

	LEFT JOIN arch_bot_rel ab ON ab.bot_id = bo.id
	LEFT JOIN archs ar ON ar.id = ab.arch_id

	LEFT JOIN bit_bot_rel bb ON bb.bot_id = bo.id
	LEFT JOIN bits bi ON bi.id = bb.bit_id

	WHERE bo.id=?
	GROUP BY bo.id;
	`, id).Scan(&botid, &botname, &botsource,
		&ownerids, &ownernames,
		&archids, &archnames,
		&bitids, &bitnames)
	if err != nil {
		log.Println(err)
		return Bot{}, err
	}

	ownerIDList := strings.Split(ownerids, ",")
	ownerNameList := strings.Split(ownernames, ",")

	var users []User
	for i, _ := range ownerIDList {
		id, err := strconv.Atoi(ownerIDList[i])
		if err != nil {
			log.Println("ERR1: ", err)
			return Bot{}, err
		}
		users = append(users, User{ID: id, Name: ownerNameList[i], PasswordHash: nil})
	}

	// assemble the archs
	archIDList := strings.Split(archids, ",")
	archNameList := strings.Split(archnames, ",")

	var archs []Arch
	if archIDList[0] != "" {
		for i, _ := range archIDList {
			id, err := strconv.Atoi(archIDList[i])
			if err != nil {
				log.Println("Err handling archs: ", err)
				return Bot{}, err
			}
			archs = append(archs, Arch{id, archNameList[i], true})
		}
	} else {
		archs = []Arch{}
	}

	// assemble the bits
	bitIDList := strings.Split(bitids, ",")
	bitNameList := strings.Split(bitnames, ",")

	var bits []Bit
	if bitIDList[0] != "" {
		for i, _ := range bitIDList {
			id, err := strconv.Atoi(bitIDList[i])
			if err != nil {
				log.Println("Err handling bits: ", err)
				return Bot{}, err
			}
			bits = append(bits, Bit{id, bitNameList[i], true})
		}
	} else {
		bits = []Bit{}
	}

	switch {
	case err != nil:
		log.Println("ERR4: ", err)
		return Bot{}, err
	default:
		//  log.Printf("returning bot with archs %+v and bits %+v", archs, bits)
		return Bot{botid, botname, botsource, users, archs, bits}, nil
	}
}

func (s *State) UpdateBotSource(name string, source string) error {
	_, err := s.db.Exec("UPDATE bots SET source=? WHERE name=?", source, name)
	if err != nil {
		return err
	} else {
		return nil
	}
}

func (s *State) UpdateBotName(orig_name string, new_name string) error {
	_, err := s.db.Exec("UPDATE bots SET name=? WHERE name=?", new_name, orig_name)
	if err != nil {
		return err
	} else {
		return nil
	}
}

// Returns the users belonging to the given bot
func (s *State) GetBotUsers(botid int) ([]User, error) {
	rows, err := s.db.Query("SELECT id, name FROM users u LEFT JOIN user_bot_rel ub ON ub.user_id = u.id WHERE ub.bot_id=?", botid)
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

// Returns the users belonging to the given bot
func (s *State) GetAllBot() ([]Bot, error) {
	rows, err := s.db.Query("SELECT id, name FROM bots;")
	defer rows.Close()
	if err != nil {
		return nil, err
	}

	var bots []Bot
	for rows.Next() {
		var bot Bot
		if err := rows.Scan(&bot.ID, &bot.Name); err != nil {
			return bots, err
		}
		bots = append(bots, bot)
	}
	if err = rows.Err(); err != nil {
		return bots, err
	}
	return bots, nil
}

// Returns the users belonging to the given bot
func (s *State) GetAllBotsWithUsers() ([]Bot, error) {
	rows, err := s.db.Query(`SELECT
		b.id, b.name, b.source, group_concat(ub.user_id), group_concat(u.name)
	FROM bots b
	LEFT JOIN user_bot_rel ub ON ub.bot_id = b.id
	LEFT JOIN users u ON ub.user_id = u.id
	GROUP BY b.id;`)
	defer rows.Close()
	if err != nil {
		return nil, err
	}

	var bots []Bot
	for rows.Next() {
		var bot Bot
		var userIDListString string
		var usernameListString string

		err := rows.Scan(&bot.ID, &bot.Name, &bot.Source, &userIDListString, &usernameListString)
		if err != nil {
			return nil, err
		}

		userIDList := strings.Split(userIDListString, ",")
		usernameList := strings.Split(usernameListString, ",")

		var users []User
		for i, _ := range userIDList {
			id, err := strconv.Atoi(userIDList[i])
			if err != nil {
				return nil, err
			}
			users = append(users, User{ID: id, Name: usernameList[i], PasswordHash: nil})
		}
		bot.Users = users

		bots = append(bots, bot)
	}
	if err = rows.Err(); err != nil {
		return bots, err
	}
	return bots, nil
}

func (s *State) LinkArchIDsToBot(botid int, archIDs []int) error {
	// delete preexisting links
	_, err := s.db.Exec("DELETE FROM arch_bot_rel WHERE bot_id=?;", botid)
	if err != nil {
		log.Println("Error deleting old arch bot link: ", err)
		return err
	}

	// yes, we're building this by hand, but as we only insert int's I'm just confident that whoever
	// gets some sqli here just deserves it :D
	query := "INSERT INTO arch_bot_rel (arch_id, bot_id) VALUES"
	for idx, id := range archIDs {
		query += fmt.Sprintf("(%d, %d)", id, botid)
		if idx != len(archIDs)-1 {
			query += ", "
		}
	}
	query += ";"
	log.Println(query)

	_, err = s.db.Exec(query)
	if err != nil {
		log.Println("LinkArchIDsToBot err: ", err)
		return err
	} else {
		return nil
	}
}

func (s *State) LinkBitIDsToBot(botid int, bitIDs []int) error {
	// delete preexisting links
	_, err := s.db.Exec("DELETE FROM bit_bot_rel WHERE bot_id=?;", botid)
	if err != nil {
		log.Println("Error deleting old bit bot link: ", err)
		return err
	}

	// yes, we're building this by hand, but as we only insert int's I'm just confident that whoever
	// gets some sqli here just deserves it :D
	query := "INSERT INTO bit_bot_rel (bit_id, bot_id) VALUES"
	for idx, id := range bitIDs {
		query += fmt.Sprintf("(%d, %d)", id, botid)
		if idx != len(bitIDs)-1 {
			query += ", "
		}
	}
	query += ";"
	log.Println(query)

	_, err = s.db.Exec(query)
	if err != nil {
		log.Println("LinkBitIDsToBot err: ", err)
		return err
	} else {
		return nil
	}
}

//////////////////////////////////////////////////////////////////////////////
// HTTP

func botsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		// define data
		data := map[string]interface{}{}
		data["version"] = os.Getenv("VERSION")

		session, _ := globalState.sessions.Get(r, "session")
		username := session.Values["username"]
		data["pagelink1"] = Link{"bot", "/bot"}
		data["pagelink1options"] = []Link{
			{Name: "user", Target: "/user"},
			{Name: "battle", Target: "/battle"},
		}
		data["pagelinknext"] = []Link{
			{Name: "new", Target: "/new"},
		}

		if username == nil {
			http.Redirect(w, r, "/login", http.StatusMethodNotAllowed)
		}

		user, err := UserGetUserFromUsername(username.(string))
		if err != nil {
			data["err"] = "Could not fetch the user"
		} else {
			data["user"] = user
		}

		bots, err := globalState.GetAllBotsWithUsers()
		data["bots"] = bots

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
		t.ExecuteTemplate(w, "bots", data)
	default:
		http.Redirect(w, r, "/", http.StatusMethodNotAllowed)
	}
}

func botSingleHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	botid, err := strconv.Atoi(vars["id"])
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("500 - Invalid bot id"))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	switch r.Method {
	case "GET":
		// define data
		data := map[string]interface{}{}
		data["version"] = os.Getenv("VERSION")
		data["pagelink1"] = Link{"bot", "/bot"}
		data["pagelink1options"] = []Link{
			{Name: "user", Target: "/user"},
			{Name: "battle", Target: "/battle"},
		}

		// display errors passed via query parameters
		log.Println("[d] Getting previous results")
		queryres := r.URL.Query().Get("res")
		if queryres != "" {
			data["res"] = queryres
		}

		// fetch the session and get the user that made the request
		session, _ := globalState.sessions.Get(r, "session")
		username := session.Values["username"].(string)

		viewer, err := UserGetUserFromUsername(username)
		if err != nil {
			data["err"] = "Could not get the id four your username... Please contact an admin"
		}

		// get the bot that was requested
		bot, err := BotGetById(int(botid))
		data["bot"] = bot
		data["user"] = viewer

		// open radare without input for building the bot
		r2p1, err := r2pipe.NewPipe("--")
		if err != nil {
			panic(err)
		}
		defer r2p1.Close()

		// TODO(emile): improve the archs and bit handling here. I'll use the first one for now,
		// but it would be nice to loop over all of them (would be a matrix with archs and bits
		// on the axes)
		src := strings.ReplaceAll(bot.Source, "\r\n", "; ")
		radareCommand := fmt.Sprintf("rasm2 -a %s -b %s \"%+v\"", bot.Archs[0].Name, bot.Bits[0].Name, src)
		bytecode, err := r2cmd(r2p1, radareCommand)
		if err != nil {
			data["err"] = "Error assembling the bot"
			http.Redirect(w, r, fmt.Sprintf("/bot/%d", botid), http.StatusSeeOther)
			return
		}
		data["bytecode_r2cmd"] = radareCommand
		data["bytecode"] = bytecode

		radareCommand = fmt.Sprintf("rasm2 -a %s -b %s -D %+v", bot.Archs[0].Name, bot.Bits[0].Name, bytecode)
		disasm, err := r2cmd(r2p1, radareCommand)
		if err != nil {
			data["err"] = "Error disassembling the bot"
			http.Redirect(w, r, fmt.Sprintf("/bot/%d", botid), http.StatusSeeOther)
			return
		}
		data["err"] = "Could not get the id four your username... Please contact an admin"

		data["disasm_r2cmd"] = radareCommand
		data["disasm"] = disasm

		// define the breadcrumbs
		data["pagelink2"] = Link{bot.Name, fmt.Sprintf("/%d", bot.ID)}

		allBotNames, err := BotGetAll()
		var opts []Link
		for _, bot := range allBotNames {

			// don't add the current bot to the list, we're already on that page!
			if bot.ID != botid {
				opts = append(opts, Link{Name: bot.Name, Target: fmt.Sprintf("/%d", bot.ID)})
			}
		}
		data["pagelink2options"] = opts

		editable := false
		for _, user := range bot.Users {
			if user.ID == viewer.ID {
				editable = true
			}
		}
		if editable == true {
			data["editable"] = true
		}

		// get all architectures and set the enable flag on the ones that are enabled in the battle
		archs, err := ArchGetAll()
		if err != nil {
			data["err"] = "Could not fetch the archs"
		} else {
			data["archs"] = archs
		}

		for i, a := range archs {
			for _, b := range bot.Archs {
				if a.ID == b.ID {
					archs[i].Enabled = true
				}
			}
		}

		// get all bits and set the enable flag on the ones that are enabled in the battle
		bits, err := BitGetAll()
		if err != nil {
			data["err"] = "Could not fetch the bits"
		} else {
			data["bits"] = bits
		}

		for i, a := range bits {
			for _, b := range bot.Bits {
				if a.ID == b.ID {
					bits[i].Enabled = true
				}
			}
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
		t.ExecuteTemplate(w, "botSingle", data)

	case "POST":
		// checking if the user submitting the bot information is allowed to do so
		session, _ := globalState.sessions.Get(r, "session")
		username := session.Values["username"].(string)

		// get the user submitting
		log.Println("Getting the user submitting the change request...")
		requesting_user, err := UserGetUserFromUsername(username)
		if err != nil {
			log.Println("err: ", err)
			http.Redirect(w, r, fmt.Sprintf("/bot/%d", botid), http.StatusSeeOther)
			return
		}

		// get the users the bot belongs
		log.Println("Getting the user the bot belongs to...")
		orig_bot, err := BotGetById(int(botid))
		if err != nil {
			log.Println("err: ", err)
			http.Redirect(w, r, fmt.Sprintf("/bot/%d", botid), http.StatusSeeOther)
			return
		}

		// check if the user submitting the change request is within the users the bot belongs to
		log.Println("Checking if edit is allowed...")
		allowed_to_edit := false
		for _, user := range orig_bot.Users {
			if user.ID == requesting_user.ID {
				allowed_to_edit = true
			}
		}

		if allowed_to_edit == false {
			http.Redirect(w, r, fmt.Sprintf("/bot/%d", botid), http.StatusSeeOther)
			return
		}

		// at this point, we're sure the user is allowed to edit the bot
		r.ParseForm()
		name := r.Form.Get("name")
		source := r.Form.Get("source")

		var archIDs []int
		var bitIDs []int

		for k, _ := range r.Form {
			if strings.HasPrefix(k, "arch-") {
				id, err := strconv.Atoi(strings.TrimPrefix(k, "arch-"))
				if err != nil {
					msg := "ERROR: Invalid arch id"
					http.Redirect(w, r, fmt.Sprintf("/bot/%d?res=%s", botid, msg), http.StatusSeeOther)
					return
				}
				archIDs = append(archIDs, id)
			}
			if strings.HasPrefix(k, "bit-") {
				id, err := strconv.Atoi(strings.TrimPrefix(k, "bit-"))
				if err != nil {
					msg := "ERROR: Invalid bit id"
					http.Redirect(w, r, fmt.Sprintf("/bot/%d?res=%s", botid, msg), http.StatusSeeOther)
					return
				}
				bitIDs = append(bitIDs, id)
			}
		}

		if len(archIDs) == 0 {
			msg := "ERROR: Please select an architecture"
			http.Redirect(w, r, fmt.Sprintf("/bot/%d?res=%s", botid, msg), http.StatusSeeOther)
			return
		}
		if len(archIDs) >= 2 {
			msg := "ERROR: Please select ONE architecture"
			http.Redirect(w, r, fmt.Sprintf("/bot/%d?res=%s", botid, msg), http.StatusSeeOther)
			return
		}

		if len(bitIDs) == 0 {
			msg := "ERROR: Please select one of the bits"
			http.Redirect(w, r, fmt.Sprintf("/bot/%d?res=%s", botid, msg), http.StatusSeeOther)
			return
		}
		if len(bitIDs) >= 2 {
			msg := "ERROR: Please select ONE of the bits"
			http.Redirect(w, r, fmt.Sprintf("/bot/%d?res=%s", botid, msg), http.StatusSeeOther)
			return
		}

		// link archs to battle
		err = BotLinkArchIDs(botid, archIDs)
		if err != nil {
			log.Println("Error linking the arch ids to the battle: ", err)
			msg := "ERROR: Could not create due to internal reasons"
			http.Redirect(w, r, fmt.Sprintf("/bot/%d?res=%s", botid, msg), http.StatusSeeOther)
			return
		}

		// link bits to battle
		err = BotLinkBitIDs(botid, bitIDs)
		if err != nil {
			log.Println("Error linking the bit ids to the battle: ", err)
			msg := "ERROR: Could not create due to internal reasons"
			http.Redirect(w, r, fmt.Sprintf("/bot/%d?res=%s", botid, msg), http.StatusSeeOther)
			return
		}

		if name != "" {
			if source != "" {
				log.Println("Updating bot...")
				err := BotUpdate(botid, name, source)
				if err != nil {
					log.Println("err: ", err)
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte("500 - Error inserting bot into db"))
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
		}

		http.Redirect(w, r, fmt.Sprintf("/bot/%d", botid), http.StatusSeeOther)

	default:
		http.Redirect(w, r, "/", http.StatusMethodNotAllowed)
	}
}

func botNewHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		// define data
		data := map[string]interface{}{}
		data["version"] = os.Getenv("VERSION")

		session, _ := globalState.sessions.Get(r, "session")
		username := session.Values["username"].(string)
		data["pagelink1"] = Link{Name: "bot", Target: "/bot"}
		data["pagelink1options"] = []Link{
			{Name: "user", Target: "/user"},
			{Name: "battle", Target: "/battle"},
		}
		data["pagelink2"] = Link{Name: "new", Target: "/new"}
		data["pagelink2options"] = []Link{
			{Name: "list", Target: ""},
		}

		// display errors passed via query parameters
		log.Println("[d] Getting previous results")
		queryres := r.URL.Query().Get("res")
		if queryres != "" {
			data["res"] = queryres
		}

		user, err := UserGetUserFromUsername(username)
		if err != nil {
			data["err"] = "Could not fetch the user"
		} else {
			data["user"] = user
		}

		archs, err := ArchGetAll()
		if err != nil {
			data["err"] = "Could not fetch the archs"
		} else {
			data["archs"] = archs
		}

		bits, err := BitGetAll()
		if err != nil {
			data["err"] = "Could not fetch the bits"
		} else {
			data["bits"] = bits
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
		t.ExecuteTemplate(w, "botNew", data)

	case "POST":
		session, _ := globalState.sessions.Get(r, "session")
		username := session.Values["username"].(string)

		// parse the post parameters
		r.ParseForm()
		log.Println("---")
		log.Println(r.Form)
		log.Println("---")

		name := r.Form.Get("name")
		source := r.Form.Get("source")

		if name == "" {
			msg := "ERROR: Please provide a name"
			http.Redirect(w, r, fmt.Sprintf("/bot/new?res=%s", msg), http.StatusSeeOther)
			return
		}

		if source == "" {
			msg := "ERROR: Please provide some source"
			http.Redirect(w, r, fmt.Sprintf("/bot/new?res=%s", msg), http.StatusSeeOther)
			return
		}

		var archIDs []int
		var bitIDs []int

		for k, _ := range r.Form {
			if strings.HasPrefix(k, "arch-") {
				id, err := strconv.Atoi(strings.TrimPrefix(k, "arch-"))
				if err != nil {
					msg := "ERROR: Invalid arch id"
					http.Redirect(w, r, fmt.Sprintf("/bot/new?res=%s", msg), http.StatusSeeOther)
					return
				}
				archIDs = append(archIDs, id)
			}
			if strings.HasPrefix(k, "bit-") {
				id, err := strconv.Atoi(strings.TrimPrefix(k, "bit-"))
				if err != nil {
					msg := "ERROR: Invalid bit id"
					http.Redirect(w, r, fmt.Sprintf("/bot/new?res=%s", msg), http.StatusSeeOther)
					return
				}
				bitIDs = append(bitIDs, id)
			}
		}

		if len(archIDs) == 0 {
			msg := "ERROR: Please select an architecture"
			http.Redirect(w, r, fmt.Sprintf("/bot/new?res=%s", msg), http.StatusSeeOther)
			return
		}
		if len(archIDs) >= 2 {
			msg := "ERROR: Please select ONE architecture"
			http.Redirect(w, r, fmt.Sprintf("/bot/new?res=%s", msg), http.StatusSeeOther)
			return
		}

		if len(bitIDs) == 0 {
			msg := "ERROR: Please select one of the bits"
			http.Redirect(w, r, fmt.Sprintf("/bot/new?res=%s", msg), http.StatusSeeOther)
			return
		}
		if len(bitIDs) >= 2 {
			msg := "ERROR: Please select ONE of the bits"
			http.Redirect(w, r, fmt.Sprintf("/bot/new?res=%s", msg), http.StatusSeeOther)
			return
		}

		botid, err := BotCreate(name, source)
		if err != nil {
			log.Println("Error creating the bot: ", err)
			msg := "ERROR: Could not create bot"
			http.Redirect(w, r, fmt.Sprintf("/bot/new?res=%s", msg), http.StatusSeeOther)
			return
		}

		err = UserLinkBot(username, botid)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - Error adding the bot to the user"))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if len(archIDs) == 0 {
			msg := "ERROR: Please select an architecture"
			http.Redirect(w, r, fmt.Sprintf("/bot/new?res=%s", msg), http.StatusSeeOther)
			return
		}

		// link archs to battle
		err = BotLinkArchIDs(botid, archIDs)
		if err != nil {
			log.Println("Error linking the arch ids to the bot: ", err)
			msg := "ERROR: Could not create due to internal reasons"
			http.Redirect(w, r, fmt.Sprintf("/bot/new?res=%s", msg), http.StatusSeeOther)
			return
		}

		if len(bitIDs) == 0 {
			msg := "ERROR: Please select an bits"
			http.Redirect(w, r, fmt.Sprintf("/bot/new?res=%s", msg), http.StatusSeeOther)
			return
		}

		// link bits to battle
		err = BotLinkBitIDs(botid, bitIDs)
		if err != nil {
			log.Println("Error linking the bit ids to the bot: ", err)
			msg := "ERROR: Could not create due to internal reasons"
			http.Redirect(w, r, fmt.Sprintf("/bot/new?res=%s", msg), http.StatusSeeOther)
			return
		}

		http.Redirect(w, r, "/bot", http.StatusSeeOther)
		return
	default:
		http.Redirect(w, r, "/", http.StatusMethodNotAllowed)
	}
}
