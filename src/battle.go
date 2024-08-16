package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

type Battle struct {
	ID     int
	Name   string
	Bots   []Bot
	Owners []User
	Public bool
	Archs  []Arch
	Bits   []Bit
}

//////////////////////////////////////////////////////////////////////////////
// GENERAL PURPOSE

func BattleGetAll() ([]Battle, error) {
	return globalState.GetAllBattles()
}

func BattleCreate(name string, public bool) (int, error) {
	return globalState.InsertBattle(Battle{Name: name, Public: public})
}

func BattleLinkBot(botid int, battleid int) error {
	return globalState.LinkBotBattle(botid, battleid)
}

func BattleGetByIdDeep(id int) (Battle, error) {
	return globalState.GetBattleByIdDeep(id)
}

func BattleUpdate(battle Battle) error {
	return globalState.UpdateBattle(battle)
}

func BattleLinkArchIDs(battleid int, archIDs []int) error {
	return globalState.LinkArchIDsToBattle(battleid, archIDs)
}

func BattleLinkBitIDs(battleid int, bitIDs []int) error {
	return globalState.LinkBitIDsToBattle(battleid, bitIDs)
}

//////////////////////////////////////////////////////////////////////////////
// DATABASE

func (s *State) InsertBattle(battle Battle) (int, error) {
	res, err := s.db.Exec("INSERT INTO battles VALUES(NULL,?,?,?);", time.Now(), battle.Name, battle.Public)
	if err != nil {
		return -1, err
	}

	var id int64
	if id, err = res.LastInsertId(); err != nil {
		return -1, err
	}
	return int(id), nil
}

func (s *State) UpdateBattle(battle Battle) error {
	_, err := s.db.Exec("UPDATE battles SET name=?, public=? WHERE id=?", battle.Name, battle.Public, battle.ID)
	if err != nil {
		return err
	}
	return nil
}

func (s *State) LinkBotBattle(botid int, battleid int) error {
	_, err := s.db.Exec("INSERT INTO bot_battle_rel VALUES (?, ?)", botid, battleid)
	if err != nil {
		log.Println("Error linking bot to battle: ", err)
		return err
	} else {
		return nil
	}
}

func (s *State) LinkArchIDsToBattle(battleid int, archIDs []int) error {
	// delete preexisting links
	_, err := s.db.Exec("DELETE FROM arch_battle_rel WHERE battle_id=?;", battleid)
	if err != nil {
		return err
	}

	// yes, we're building this by hand, but as we only insert int's I'm just confident that whoever
	// gets some sqli here just deserves it :D
	query := "INSERT INTO arch_battle_rel (arch_id, battle_id) VALUES"
	for idx, id := range archIDs {
		query += fmt.Sprintf("(%d, %d)", id, battleid)
		if idx != len(archIDs)-1 {
			query += ", "
		}
	}
	query += ";"
	log.Println(query)

	_, err = s.db.Exec(query)
	if err != nil {
		log.Println("LinkArchIDsToBattle err: ", err)
		return err
	} else {
		return nil
	}
}

func (s *State) LinkBitIDsToBattle(battleid int, bitIDs []int) error {
	// delete preexisting links
	_, err := s.db.Exec("DELETE FROM bit_battle_rel WHERE battle_id=?;", battleid)
	if err != nil {
		return err
	}

	// yes, we're building this by hand, but as we only insert int's I'm just confident that whoever
	// gets some sqli here just deserves it :D
	query := "INSERT INTO bit_battle_rel (bit_id, battle_id) VALUES"
	for idx, id := range bitIDs {
		query += fmt.Sprintf("(%d, %d)", id, battleid)
		if idx != len(bitIDs)-1 {
			query += ", "
		}
	}
	query += ";"
	log.Println(query)

	_, err = s.db.Exec(query)
	if err != nil {
		log.Println("LinkBitIDsToBattle err: ", err)
		return err
	} else {
		return nil
	}
}

func (s *State) GetAllBattles() ([]Battle, error) {
	rows, err := s.db.Query("SELECT id, name FROM battles;")
	defer rows.Close()
	if err != nil {
		return nil, err
	}

	var battles []Battle
	for rows.Next() {
		var battle Battle
		if err := rows.Scan(&battle.ID, &battle.Name); err != nil {
			log.Println(err)
			return battles, err
		}
		battles = append(battles, battle)
	}
	if err = rows.Err(); err != nil {
		log.Println(err)
		return battles, err
	}
	return battles, nil
}

func (s *State) GetBattleByIdDeep(id int) (Battle, error) {
	var battleid int
	var battlename string
	var battlepublic bool

	var botids string
	var botnames string

	var userids string
	var usernames string

	var archids string
	var archnames string

	var bitids string
	var bitnames string

	// battles have associated bots and users, we're fetching 'em all!

	// This fetches the battles and relates the associated bots, users, archs and bits

	err := s.db.QueryRow(`
	SELECT DISTINCT
		ba.id, ba.name, ba.public,
		COALESCE(group_concat(DISTINCT bb.bot_id), ""),
		COALESCE(group_concat(DISTINCT bo.name), ""),
		COALESCE(group_concat(DISTINCT ub.user_id), ""),
		COALESCE(group_concat(DISTINCT us.name), ""),
		COALESCE(group_concat(DISTINCT ab.arch_id), ""),
		COALESCE(group_concat(DISTINCT ar.name), ""),
		COALESCE(group_concat(DISTINCT bitbat.bit_id), ""),
		COALESCE(group_concat(DISTINCT bi.name), "")
	FROM battles ba

	LEFT JOIN bot_battle_rel bb ON bb.battle_id = ba.id
	LEFT JOIN bots bo ON bo.id = bb.bot_id

	LEFT JOIN user_battle_rel ub ON ub.battle_id = ba.id
	LEFT JOIN users us ON us.id = ub.user_id

	LEFT JOIN arch_battle_rel ab ON ab.battle_id = ba.id
	LEFT JOIN archs ar ON ar.id = ab.arch_id

	LEFT JOIN bit_battle_rel bitbat ON bitbat.battle_id = ba.id
	LEFT JOIN bits bi ON bi.id = bitbat.bit_id

	WHERE ba.id=?
	GROUP BY ba.id;
	`, id).Scan(&battleid, &battlename, &battlepublic, &botids, &botnames, &userids, &usernames, &archids, &archnames, &bitids, &bitnames)
	if err != nil {
		log.Println("Err making GetBattleByID query: ", err)
		return Battle{}, err
	}

	log.Println("battleid: ", battleid)
	log.Println("battlename: ", battlename)
	log.Println("battlepublic: ", battlepublic)
	log.Println("botids: ", botids)
	log.Println("botnames: ", botnames)
	log.Println("userids: ", userids)
	log.Println("usernames: ", usernames)
	log.Println("archids: ", archids)
	log.Println("archnames: ", archnames)
	log.Println("bitids: ", bitids)
	log.Println("bitnames: ", bitnames)

	// The below is a wonderful examle of how golang could profit from macros
	// I should just have done this all in common lisp tbh.

	// assemble the bots
	botIDList := strings.Split(botids, ",")
	botNameList := strings.Split(botnames, ",")

	// Using strings.Split on an empty string returns a list containing
	// nothing with a length of one
	// https://go.dev/play/p/N1D-OcwiVAs

	var bots []Bot
	if botIDList[0] != "" {
		for i, _ := range botIDList {
			id, err := strconv.Atoi(botIDList[i])
			if err != nil {
				log.Println("Err handling bots: ", err)
				return Battle{}, err
			}
			bots = append(bots, Bot{id, botNameList[i], "", []User{}, []Arch{}, []Bit{}})
		}
	} else {
		bots = []Bot{}
	}

	// assemble the users
	userIDList := strings.Split(userids, ",")
	userNameList := strings.Split(usernames, ",")

	var users []User
	if userIDList[0] != "" {
		for i, _ := range userIDList {
			id, err := strconv.Atoi(userIDList[i])
			if err != nil {
				log.Println("Err handling users: ", err)
				return Battle{}, err
			}
			users = append(users, User{id, userNameList[i], []byte{}})
		}
	} else {
		users = []User{}
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
				return Battle{}, err
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
				return Battle{}, err
			}
			bits = append(bits, Bit{id, bitNameList[i], true})
		}
	} else {
		bits = []Bit{}
	}

	// return it all!
	switch {
	case err != nil:
		log.Println("Overall err in the GetBattleByID func: ", err)
		return Battle{}, err
	default:
		return Battle{
			ID:     battleid,
			Name:   battlename,
			Bots:   bots,
			Owners: users,
			Public: battlepublic,
			Archs:  archs,
			Bits:   bits,
		}, nil
	}
}

//////////////////////////////////////////////////////////////////////////////
// HTTP

func battlesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		// define data
		data := map[string]interface{}{}
		data["pagelink1"] = Link{Name: "battle", Target: "/battle"}
		data["pagelink1options"] = []Link{
			{Name: "bot", Target: "/bot"},
			{Name: "user", Target: "/user"},
		}
		data["pagelinknext"] = []Link{
			{Name: "new", Target: "/new"},
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

		// get all battles
		battles, err := BattleGetAll()
		data["battles"] = battles

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
		t.ExecuteTemplate(w, "battles", data)
	default:
		http.Redirect(w, r, "/", http.StatusMethodNotAllowed)
	}
}

func battleNewHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		// define data
		data := map[string]interface{}{}

		// breadcrumb foo
		session, _ := globalState.sessions.Get(r, "session")
		username := session.Values["username"].(string)
		data["pagelink1"] = Link{Name: "battle", Target: "/battle"}
		data["pagelink1options"] = []Link{
			{Name: "user", Target: "/user"},
			{Name: "bot", Target: "/bot"},
		}
		data["pagelink2"] = Link{Name: "new", Target: "/new"}
		data["pagelink2options"] = []Link{
			{Name: "list", Target: ""},
		}

		// display errors passed via query parameters
		queryres := r.URL.Query().Get("err")
		if queryres != "" {
			data["res"] = queryres
		}

		// get data needed
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
		t.ExecuteTemplate(w, "battleNew", data)

	case "POST":
		// parse the post parameters
		r.ParseForm()
		name := r.Form.Get("name")

		var public bool
		query_public := r.Form.Get("public")
		if query_public == "on" {
			public = true
		}

		// gather the information from the arch and bit selection
		var archIDs []int
		var bitIDs []int

		for k, _ := range r.Form {
			if strings.HasPrefix(k, "arch-") {
				id, err := strconv.Atoi(strings.TrimPrefix(k, "arch-"))
				if err != nil {
					msg := "ERROR: Invalid arch id"
					http.Redirect(w, r, fmt.Sprintf("/battle/new?res=%s", msg), http.StatusSeeOther)
					return
				}
				archIDs = append(archIDs, id)
			}
			if strings.HasPrefix(k, "bit-") {
				id, err := strconv.Atoi(strings.TrimPrefix(k, "bit-"))
				if err != nil {
					msg := "ERROR: Invalid bit id"
					http.Redirect(w, r, fmt.Sprintf("/battle/new?res=%s", msg), http.StatusSeeOther)
					return
				}
				bitIDs = append(bitIDs, id)
			}
		}

		if name != "" {
			// create the battle itself
			log.Println("Creating battle")
			battleid, err := BattleCreate(name, public)
			if err != nil {
				log.Println("Error creating the battle using BattleCreate(): ", err)
				msg := "ERROR: Could not create due to internal reasons"
				http.Redirect(w, r, fmt.Sprintf("/battle/new?res=%s", msg), http.StatusSeeOther)
				return
			}

			// link archs to battle
			err = BattleLinkArchIDs(battleid, archIDs)
			if err != nil {
				log.Println("Error linking the arch ids to the battle: ", err)
				msg := "ERROR: Could not create due to internal reasons"
				http.Redirect(w, r, fmt.Sprintf("/battle/new?res=%s", msg), http.StatusSeeOther)
				return
			}

			// link bits to battle
			err = BattleLinkBitIDs(battleid, bitIDs)
			if err != nil {
				log.Println("Error linking the bit ids to the battle: ", err)
				msg := "ERROR: Could not create due to internal reasons"
				http.Redirect(w, r, fmt.Sprintf("/battle/new?res=%s", msg), http.StatusSeeOther)
				return
			}
		} else {
			msg := "ERROR: Please provide a name"
			http.Redirect(w, r, fmt.Sprintf("/battle/new?res=%s", msg), http.StatusSeeOther)
			return
		}

		http.Redirect(w, r, "/battle", http.StatusSeeOther)
		return
	default:
		http.Redirect(w, r, "/", http.StatusMethodNotAllowed)
	}
}

// TODO(emile): add user creating battle as default owner
// TODO(emile): allow adding other users as owners to battles
// TODO(emile): implement submitting bots
// TODO(emile): implement running the battle
// TODO(emile): add a "start battle now" button
// TODO(emile): add a "battle starts at this time" field into the battle
// TODO(emile): figure out how time is stored and restored with the db
// TODO(emile): do some magic to display the current fight backlog with all info

func battleSingleHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	battleid, err := strconv.Atoi(vars["id"])
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("500 - Invalid battle id"))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	switch r.Method {
	case "GET":
		// define data
		data := map[string]interface{}{}
		data["pagelink1"] = Link{"battle", "/battle"}
		data["pagelink1options"] = []Link{
			{Name: "user", Target: "/user"},
			{Name: "bot", Target: "/bot"},
		}

		// display errors passed via query parameters
		queryres := r.URL.Query().Get("res")
		if queryres != "" {
			data["res"] = queryres
		}

		session, _ := globalState.sessions.Get(r, "session")
		username := session.Values["username"].(string)

		viewer, err := UserGetUserFromUsername(username)
		if err != nil {
			data["err"] = "Could not get the id four your username... Please contact an admin"
		}
		data["user"] = viewer

		// get the battle including it's users, bots, archs, bits
		battle, err := BattleGetByIdDeep(int(battleid))
		data["battle"] = battle
		data["botAmount"] = len(battle.Bots)
		data["battleCount"] = (len(battle.Bots) * len(battle.Bots)) * 2

		// define the breadcrumbs
		data["pagelink2"] = Link{battle.Name, fmt.Sprintf("/%d", battle.ID)}

		allbattleNames, err := BattleGetAll()
		var opts []Link
		for _, battle := range allbattleNames {
			opts = append(opts, Link{Name: battle.Name, Target: fmt.Sprintf("/%d", battle.ID)})
		}
		data["pagelink2options"] = opts

		// get the bots of the user viewing the page, as they might want to submit them
		myBots, err := UserGetBotsUsingUsername(username)
		if err != nil {
			log.Println("err: ", err)
			http.Redirect(w, r, fmt.Sprintf("/battle/%d", battleid), http.StatusSeeOther)
			return
		}
		data["myBots"] = myBots

		// get all architectures and set the enable flag on the ones that are enabled in the battle
		archs, err := ArchGetAll()
		if err != nil {
			data["err"] = "Could not fetch the archs"
		} else {
			data["archs"] = archs
		}

		for i, a := range archs {
			for _, b := range battle.Archs {
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
			for _, b := range battle.Bits {
				if a.ID == b.ID {
					bits[i].Enabled = true
				}
			}
		}

		// check if we're allowed to edit
		editable := false
		for _, owner := range battle.Owners {
			if owner.ID == viewer.ID {
				editable = true
			}
		}
		if editable == true {
			data["editable"] = true
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
		t.ExecuteTemplate(w, "battleSingle", data)

	case "POST":
		log.Println("POST!")
		// checking if the user submitting the battle information is allowed to do so
		// session, _ := globalState.sessions.Get(r, "session")
		// username := session.Values["username"].(string)

		// get the user submitting
		// log.Println("Getting the user submitting the change request...")
		// requesting_user, err := UserGetUserFromUsername(username)
		// if err != nil {
		// 	log.Println("err: ", err)
		// 	http.Redirect(w, r, fmt.Sprintf("/battle/%d", battleid), http.StatusSeeOther)
		// 	return
		// }

		// get the users the battle belongs to
		// log.Println("Getting the user the battle belongs to...")
		// orig_battle, err := BattleGetByIdDeep(int(battleid))
		// if err != nil {
		// 	log.Println("err: ", err)
		// 	http.Redirect(w, r, fmt.Sprintf("/battle/%d", battleid), http.StatusSeeOther)
		// 	return
		// }

		// check if the user submitting the change request is within the users the battle belongs to
		// log.Println("Checking if edit is allowed...")
		// allowed_to_edit := false
		// for _, user := range orig_battle.Owners {
		// 	if user.ID == requesting_user.ID {
		// 		allowed_to_edit = true
		// 	}
		// }

		// if allowed_to_edit == false {
		// 	msg := "ERROR: You aren't allowed to edit this battle!"
		// 	http.Redirect(w, r, fmt.Sprintf("/battle/%d?res=%s", battleid, msg), http.StatusSeeOther)
		// 	return
		// }

		// at this point, we're sure the user is allowed to edit the battle

		r.ParseForm()

		log.Println("r.Form: ", r.Form)
		form_name := r.Form.Get("name")

		var public bool
		if r.Form.Get("public") == "on" {
			public = true
		}

		// gather the information from the arch and bit selection
		var archIDs []int
		var bitIDs []int

		for k, _ := range r.Form {
			if strings.HasPrefix(k, "arch-") {
				id, err := strconv.Atoi(strings.TrimPrefix(k, "arch-"))
				if err != nil {
					msg := "ERROR: Invalid arch id"
					http.Redirect(w, r, fmt.Sprintf("/battle/%d?res=%s#settings", battleid, msg), http.StatusSeeOther)
					return
				}
				archIDs = append(archIDs, id)
			}
			if strings.HasPrefix(k, "bit-") {
				id, err := strconv.Atoi(strings.TrimPrefix(k, "bit-"))
				if err != nil {
					msg := "ERROR: Invalid bit id"
					http.Redirect(w, r, fmt.Sprintf("/battle/%d?res=%s#settings", battleid, msg), http.StatusSeeOther)
					return
				}
				bitIDs = append(bitIDs, id)
			}
		}

		// link archs to battle
		err = BattleLinkArchIDs(battleid, archIDs)
		if err != nil {
			log.Println("Error linking the arch ids to the battle: ", err)
			msg := "ERROR: Could not create due to internal reasons"
			http.Redirect(w, r, fmt.Sprintf("/battle/%d?res=%s#settings", battleid, msg), http.StatusSeeOther)
			return
		}

		// link bits to battle
		err = BattleLinkBitIDs(battleid, bitIDs)
		if err != nil {
			log.Println("Error linking the bit ids to the battle: ", err)
			msg := "ERROR: Could not create due to internal reasons"
			http.Redirect(w, r, fmt.Sprintf("/battle/%d?res=%s#settings", battleid, msg), http.StatusSeeOther)
			return
		}

		new_battle := Battle{int(battleid), form_name, []Bot{}, []User{}, public, []Arch{}, []Bit{}}

		log.Println("Updating battle...")
		err = BattleUpdate(new_battle)
		if err != nil {
			log.Println("err: ", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - Error inserting battle into db"))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, fmt.Sprintf("/battle/%d?res=Success!#settings", battleid), http.StatusSeeOther)

	default:
		http.Redirect(w, r, "/", http.StatusMethodNotAllowed)
	}
}

func battleSubmitHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	battleid, err := strconv.Atoi(vars["id"])
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("500 - Invalid battle id"))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	switch r.Method {
	case "POST":
		r.ParseForm()

		log.Println("Adding bot to battle", battleid)
		log.Println(r.Form)

		// get all the form values that contain the bot that shall be submitted
		var botIDs []int
		for k, _ := range r.Form {
			if strings.HasPrefix(k, "bot-") {
				id, err := strconv.Atoi(strings.TrimPrefix(k, "bot-"))
				if err != nil {
					msg := "ERROR: Invalid bot supplied"
					http.Redirect(w, r, fmt.Sprintf("/battle/%d?res=%s", battleid, msg), http.StatusSeeOther)
					return
				}
				botIDs = append(botIDs, id)
			}
		}

		log.Println(botIDs)

		battle, err := BattleGetByIdDeep(battleid)
		if err != nil {
			msg := "ERROR: Couln't get the battle with the given id"
			http.Redirect(w, r, fmt.Sprintf("/battle/%d?res=%s", battleid, msg), http.StatusSeeOther)
			return
		}
		log.Println(battle)

		// for all bots, get their bits and arch and compare them to the one of the battle
		for _, id := range botIDs {
			bot, err := BotGetById(id)
			if err != nil {
				msg := fmt.Sprintf("ERROR: Couldn't get bot with id %d", id)
				http.Redirect(w, r, fmt.Sprintf("/battle/%d?res=%s", battleid, msg), http.StatusSeeOther)
				return
			}

			var archValid bool = false
			for _, battle_arch := range battle.Archs {
				for _, bot_arch := range bot.Archs {
					if battle_arch.ID == bot_arch.ID {
						archValid = true
					}
				}
			}

			var bitValid bool = false
			for _, battle_bit := range battle.Bits {
				for _, bot_bit := range bot.Bits {
					if battle_bit.ID == bot_bit.ID {
						bitValid = true
					}
				}
			}

			if archValid && bitValid {
				log.Printf("arch and bit valid, adding bot with id %d to battle with id %d\n", id, battleid)
				BattleLinkBot(id, battleid)
			} else {
				if archValid == false {
					msg := "Bot has an invalid architecture!"
					http.Redirect(w, r, fmt.Sprintf("/battle/%d?res=%s", battleid, msg), http.StatusSeeOther)
					return
				}
				if bitValid == false {
					msg := "Bot has an invalid 'bit-ness'!"
					http.Redirect(w, r, fmt.Sprintf("/battle/%d?res=%s", battleid, msg), http.StatusSeeOther)
					return
				}
			}

			log.Println(bot)
		}
		msg := "Success!"
		http.Redirect(w, r, fmt.Sprintf("/battle/%d?res=%s", battleid, msg), http.StatusSeeOther)
	default:
		http.Redirect(w, r, "/", http.StatusMethodNotAllowed)
	}
}
