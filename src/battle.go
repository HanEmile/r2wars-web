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

type Battle struct {
	ID        int
	Name      string
	Bots      []Bot
	Owners    []User
	Public    bool
	Archs     []Arch
	Bits      []Bit
	RawOutput string
	MaxRounds int
}

//////////////////////////////////////////////////////////////////////////////
// GENERAL PURPOSE

func BattleGetAll() ([]Battle, error) {
	return globalState.GetAllBattles()
}

func BattleCreate(name string, public bool, owner User) (int, error) {
	return globalState.InsertBattle(Battle{Name: name, Public: public}, owner)
}

func BattleLinkBot(botid int, battleid int) error {
	return globalState.LinkBotBattle(botid, battleid)
}

func BattleUnlinkAllBotsForUser(userid int, battleid int) error {
	return globalState.UnlinkAllBotsForUserFromBattle(userid, battleid)
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

func BattleSaveRawOutput(battleid int, rawOutput string) error {
	return globalState.UpdateBattleRawOutput(battleid, rawOutput)
}

func BattleDeleteID(battleid int) error {
	return globalState.DeleteBattleByID(battleid)
}

//////////////////////////////////////////////////////////////////////////////
// DATABASE

func (s *State) InsertBattle(battle Battle, owner User) (int, error) {
	// create the battle
	res, err := s.db.Exec("INSERT INTO battles VALUES(NULL,?,?,?);", time.Now(), battle.Name, battle.Public)
	if err != nil {
		log.Println(err)
		return -1, err
	}

	var id int64
	if id, err = res.LastInsertId(); err != nil {
		log.Println(err)
		return -1, err
	}

	// insert the owner into the battle_owner rel
	_, err = s.db.Exec("INSERT INTO owner_battle_rel VALUES (?, ?)", owner.ID, battle.ID)
	if err != nil {
		log.Println(err)
		return -1, err
	}

	return int(id), nil
}

func (s *State) UpdateBattle(battle Battle) error {
	_, err := s.db.Exec("UPDATE battles SET name=?, public=? WHERE id=?", battle.Name, battle.Public, battle.ID)
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func (s *State) LinkBotBattle(botid int, battleid int) error {
	_, err := s.db.Exec("INSERT INTO bot_battle_rel VALUES (?, ?)", botid, battleid)
	if err != nil {
		log.Println(err)
		return err
	} else {
		return nil
	}
}

func (s *State) UnlinkAllBotsForUserFromBattle(userid int, battleid int) error {
	// get a user with the given id
	// for all of their bots
	// delete the bots from the bot_battle relation

	// there are some joins to get through the following links:
	// bot_battle_rel.bot_id
	//   -> bot.id
	//   -> user_bot_rel.bot_id
	//   -> user_bot_rel.user_id
	//   -> user.id

	// delete preexisting links
	_, err := s.db.Exec(`
	DELETE FROM bot_battle_rel
	WHERE bot_id IN
		(SELECT b.id
		 FROM bot_battle_rel bb_rel
		 JOIN bots b ON b.id = bb_rel.bot_id
		 JOIN user_bot_rel ub_rel ON ub_rel.bot_id = b.id
		 JOIN users u ON u.id = ub_rel.user_id
		 WHERE u.id=?)`, userid)

	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}

func (s *State) LinkArchIDsToBattle(battleid int, archIDs []int) error {
	// delete preexisting links
	_, err := s.db.Exec("DELETE FROM arch_battle_rel WHERE battle_id=?;", battleid)
	if err != nil {
		log.Println(err)
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
		log.Println(err)
		return err
	} else {
		return nil
	}
}

func (s *State) LinkBitIDsToBattle(battleid int, bitIDs []int) error {
	// delete preexisting links
	_, err := s.db.Exec("DELETE FROM bit_battle_rel WHERE battle_id=?;", battleid)
	if err != nil {
		log.Println(err)
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
		log.Println(err)
		return err
	} else {
		return nil
	}
}

func (s *State) GetAllBattles() ([]Battle, error) {
	rows, err := s.db.Query("SELECT id, name FROM battles;")
	defer rows.Close()
	if err != nil {
		log.Println(err)
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
	var battlerawoutput string
	var battlemaxrounds int

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

	// TODO(emile): go deeper! we could fetch battle -> bot -> arch (so fetching the linked arch
	//              for the given bot)

	// COALESCE is used to set default values
	// TODO(emile): do fancy migrations instead of the COALESCE stuff for setting defaults if
	// no value is set beforehand

	err := s.db.QueryRow(`
	SELECT DISTINCT
		ba.id, ba.name, ba.public, 
		COALESCE(ba.raw_output, ""),
		COALESCE(ba.max_rounds, 100),
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
	`, id).Scan(&battleid, &battlename, &battlepublic, &battlerawoutput, &battlemaxrounds, &botids, &botnames, &userids, &usernames, &archids, &archnames, &bitids, &bitnames)
	if err != nil {
		log.Println(err)
		return Battle{}, err
	}

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
		for i := range botIDList {
			id, err := strconv.Atoi(botIDList[i])
			if err != nil {
				log.Println(err)
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
		for i := range userIDList {
			id, err := strconv.Atoi(userIDList[i])
			if err != nil {
				log.Println(err)
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
		for i := range archIDList {
			id, err := strconv.Atoi(archIDList[i])
			if err != nil {
				log.Println(err)
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
		for i := range bitIDList {
			id, err := strconv.Atoi(bitIDList[i])
			if err != nil {
				log.Println(err)
				return Battle{}, err
			}
			bits = append(bits, Bit{id, bitNameList[i], true})
		}
	} else {
		bits = []Bit{}
	}

	return Battle{
		ID:        battleid,
		Name:      battlename,
		Bots:      bots,
		Owners:    users,
		Public:    battlepublic,
		Archs:     archs,
		Bits:      bits,
		RawOutput: battlerawoutput,
		MaxRounds: battlemaxrounds,
	}, nil
}

func (s *State) UpdateBattleRawOutput(battleid int, rawOutput string) error {
	_, err := s.db.Exec("UPDATE battles SET raw_output=? WHERE id=?", rawOutput, battleid)
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

// This deletes a battle and all links to users, bots, architectures and bits
func (s *State) DeleteBattleByID(battleid int) error {
	_, err := s.db.Exec(`
	DELETE FROM battles WHERE battleid = ?;

	DELETE FROM user_battle_rel WHERE battleid = ?;
	DELETE FROM bot_battle_rel WHERE battleid = ?;
	DELETE FROM arch_battle_rel WHERE battleid = ?;
	DELETE FROM bit_battle_rel WHERE battleid = ?;
	`, battleid, battleid, battleid, battleid, battleid)

	if err != nil {
		log.Println(err)
		return err
	}

	_, err = s.db.Exec(`
	DELETE FROM battles
	WHERE battleid = ?
	`, battleid)

	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}

//////////////////////////////////////////////////////////////////////////////
// HTTP

func battlesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		// define data
		data := map[string]interface{}{}
		data["version"] = os.Getenv("VERSION")
		data["pagelink1"] = Link{Name: "battle", Target: "/battle"}
		data["pagelink1options"] = []Link{
			{Name: "bot", Target: "/bot"},
			{Name: "user", Target: "/user"},
		}
		data["pagelinknext"] = []Link{
			{Name: "new", Target: "/new"},
			{Name: "quick", Target: "/quick"},
		}

		// sessions
		session, _ := globalState.sessions.Get(r, "session")
		username := session.Values["username"]

		if username == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		} else {
			// get the user
			user, err := UserGetUserFromUsername(username.(string))
			if err != nil {
				log.Println(err)
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			data["user"] = user
		}

		// get all battles
		battles, err := BattleGetAll()
		data["battles"] = battles

		// get the template
		t, err := template.ParseGlob(fmt.Sprintf("%s/*.html", templatesPath))
		if err != nil {
			log.Println(err)
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
		data["version"] = os.Getenv("VERSION")

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
			{Name: "quick", Target: "/quick"},
		}

		// display errors passed via query parameters
		queryres := r.URL.Query().Get("err")
		if queryres != "" {
			data["res"] = queryres
		}

		// get data needed
		user, err := UserGetUserFromUsername(username)
		if err != nil {
			log.Println(err)
			data["err"] = "Could not fetch the user"
		} else {
			data["user"] = user
		}

		archs, err := ArchGetAll()
		if err != nil {
			log.Println(err)
			data["err"] = "Could not fetch the archs"
		} else {
			data["archs"] = archs
		}

		bits, err := BitGetAll()
		if err != nil {
			log.Println(err)
			data["err"] = "Could not fetch the bits"
		} else {
			data["bits"] = bits
		}

		// get the template
		t, err := template.ParseGlob(fmt.Sprintf("%s/*.html", templatesPath))
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - Error reading template file"))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// exec!
		t.ExecuteTemplate(w, "battleNew", data)

	case "POST":
		data := map[string]interface{}{}

		session, _ := globalState.sessions.Get(r, "session")
		username := session.Values["username"].(string)

		// get data needed
		user, err := UserGetUserFromUsername(username)
		if err != nil {
			log.Println(err)
			data["err"] = "Could not fetch the user"
		} else {
			data["user"] = user
		}

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
					log.Println(err)
					msg := "ERROR: Invalid arch id"
					http.Redirect(w, r, fmt.Sprintf("/battle/new?res=%s", msg), http.StatusSeeOther)
					return
				}
				archIDs = append(archIDs, id)
			}
			if strings.HasPrefix(k, "bit-") {
				id, err := strconv.Atoi(strings.TrimPrefix(k, "bit-"))
				if err != nil {
					log.Println(err)
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
			battleid, err := BattleCreate(name, public, user)
			if err != nil {
				log.Println(err)
				msg := "ERROR: Could not create due to internal reasons"
				http.Redirect(w, r, fmt.Sprintf("/battle/new?res=%s", msg), http.StatusSeeOther)
				return
			}

			// link archs to battle
			err = BattleLinkArchIDs(battleid, archIDs)
			if err != nil {
				log.Println(err)
				msg := "ERROR: Could not create due to internal reasons"
				http.Redirect(w, r, fmt.Sprintf("/battle/new?res=%s", msg), http.StatusSeeOther)
				return
			}

			// link bits to battle
			err = BattleLinkBitIDs(battleid, bitIDs)
			if err != nil {
				log.Println(err)
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

func battleQuickHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		// define data
		data := map[string]interface{}{}
		data["version"] = os.Getenv("VERSION")

		// breadcrumb foo
		session, _ := globalState.sessions.Get(r, "session")
		username := session.Values["username"].(string)
		data["pagelink1"] = Link{Name: "battle", Target: "/battle"}
		data["pagelink1options"] = []Link{
			{Name: "user", Target: "/user"},
			{Name: "bot", Target: "/bot"},
		}
		data["pagelink2"] = Link{Name: "quick", Target: "/quick"}
		data["pagelink2options"] = []Link{
			{Name: "new", Target: "/new"},
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
			log.Println(err)
			data["err"] = "Could not fetch the user"
		} else {
			data["user"] = user
		}

		// essentiall... ...the list of all bots from which the user can select two that shall
		// battle!
		bots, err := globalState.GetAllBotsWithUsers()
		data["bots"] = bots

		// get the template
		t, err := template.ParseGlob(fmt.Sprintf("%s/*.html", templatesPath))
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - Error reading template file"))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// exec!
		t.ExecuteTemplate(w, "battleQuick", data)

	case "POST":
		data := map[string]interface{}{}

		session, _ := globalState.sessions.Get(r, "session")
		username := session.Values["username"].(string)

		// get data needed
		user, err := UserGetUserFromUsername(username)
		if err != nil {
			log.Println(err)
			data["err"] = "Could not fetch the user"
		} else {
			data["user"] = user
		}

		// parse the post parameters
		r.ParseForm()

		var public bool
		query_public := r.Form.Get("public")
		if query_public == "on" {
			public = true
		}

		// gather the information from the arch and bit selection
		var botIDs []int

		for k, _ := range r.Form {
			if strings.HasPrefix(k, "bot-") {
				id, err := strconv.Atoi(strings.TrimPrefix(k, "bot-"))
				if err != nil {
					log.Println(err)
					msg := "ERROR: Invalid bot id"
					http.Redirect(w, r, fmt.Sprintf("/battle/quick?res=%s", msg), http.StatusSeeOther)
					return
				}
				botIDs = append(botIDs, id)
			}
		}

		// create the battle itself
		log.Println("Creating battle")
		battleid, err := BattleCreate("quick", public, user)
		if err != nil {
			log.Println(err)
			msg := "ERROR: Could not create due to internal reasons"
			http.Redirect(w, r, fmt.Sprintf("/battle/quick?res=%s", msg), http.StatusSeeOther)
			return
		}

		// allow all archs and all bits

		// link bots to battle

		http.Redirect(w, r, fmt.Sprintf("/battle/%d", battleid), http.StatusSeeOther)

		//  // link archs to battle
		//  err = BattleLinkArchIDs(battleid, archIDs)
		//  if err != nil {
		//  	log.Println(err)
		//  	msg := "ERROR: Could not create due to internal reasons"
		//  	http.Redirect(w, r, fmt.Sprintf("/battle/quick?res=%s", msg), http.StatusSeeOther)
		//  	return
		//  }

		//  // link bits to battle
		//  err = BattleLinkBitIDs(battleid, bitIDs)
		//  if err != nil {
		//  	log.Println(err)
		//  	msg := "ERROR: Could not create due to internal reasons"
		//  	http.Redirect(w, r, fmt.Sprintf("/battle/quick?res=%s", msg), http.StatusSeeOther)
		//  	return
		//  }

		//  http.Redirect(w, r, "/battle", http.StatusSeeOther)
		return
	default:
		http.Redirect(w, r, "/", http.StatusMethodNotAllowed)
	}
}

func battleSingleHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	battleid, err := strconv.Atoi(vars["id"])
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("500 - Invalid battle id"))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// A partially filled format string (the reason for the redirect is still to be filled later)
	redir_target := fmt.Sprintf("/battle/%d?res=%%s", battleid)

	switch r.Method {
	case "GET":
		// define data
		data := map[string]interface{}{}
		data["version"] = os.Getenv("VERSION")
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
		username := session.Values["username"]

		if username == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		viewer, err := UserGetUserFromUsername(username.(string))
		if err != nil {
			log_and_redir_with_msg(w, r, err, redir_target, "Could not get the id for your username")
			return
		}
		data["user"] = viewer

		// get the battle including it's users, bots, archs, bits
		battle, err := BattleGetByIdDeep(int(battleid))
		if err != nil {
			log_and_redir_with_msg(w, r, err, redir_target, "Could not get the battle given the id provided")
			return
		}
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
		myBots, err := UserGetBotsUsingUsername(username.(string))
		if err != nil {
			log_and_redir_with_msg(w, r, err, redir_target, "Could not get your bots")
			return
		}
		data["myBots"] = myBots

		// get all architectures and set the enable flag on the ones that are enabled in the battle
		archs, err := ArchGetAll()
		if err != nil {
			log_and_redir_with_msg(w, r, err, redir_target, "Could not get your bots")
			return
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
			log_and_redir_with_msg(w, r, err, redir_target, "Could not fetch the bits")
			return
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
		err = t.ExecuteTemplate(w, "battleSingle", data)
		if err != nil {
			log_and_redir_with_msg(w, r, err, redir_target, "err rendering template")
		}

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

		data := map[string]interface{}{}

		session, _ := globalState.sessions.Get(r, "session")
		username := session.Values["username"].(string)

		// get data needed
		user, err := UserGetUserFromUsername(username)
		if err != nil {
			log.Println(err)
			data["err"] = "Could not fetch the user"
		} else {
			data["user"] = user
		}

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
					log_and_redir_with_msg(w, r, err, redir_target, "Invalid Arch ID")
					return
				}
				archIDs = append(archIDs, id)
			}
			if strings.HasPrefix(k, "bit-") {
				id, err := strconv.Atoi(strings.TrimPrefix(k, "bit-"))
				if err != nil {
					log_and_redir_with_msg(w, r, err, redir_target, "Invalid Bit ID")
					return
				}
				bitIDs = append(bitIDs, id)
			}
		}

		// link archs to battle
		err = BattleLinkArchIDs(battleid, archIDs)
		if err != nil {
			log_and_redir_with_msg(w, r, err, redir_target+"#settings", "Could not link arch id to battle")
			return
		}

		// link bits to battle
		err = BattleLinkBitIDs(battleid, bitIDs)
		if err != nil {
			log_and_redir_with_msg(w, r, err, redir_target+"#settings", "Could not link bit id to battle")
			return
		}

		new_battle := Battle{int(battleid), form_name, []Bot{}, []User{user}, public, []Arch{}, []Bit{}, "", 100}

		log.Println("Updating battle...")
		err = BattleUpdate(new_battle)
		if err != nil {
			log_and_redir_with_msg(w, r, err, redir_target+"#settings", "Could not insert battle into db")
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

	redir_target := fmt.Sprintf("/battle/%d?res=%%s", battleid)

	switch r.Method {
	case "POST":
		r.ParseForm()

		log.Println("Someone submitted the following form:")
		log.Println(r.Form)

		session, _ := globalState.sessions.Get(r, "session")
		username := session.Values["username"]

		if username == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		user, err := UserGetUserFromUsername(username.(string))
		if err != nil {
			log_and_redir_with_msg(w, r, err, redir_target, "Could not get the id for your username")
			return
		}

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

		battle, err := BattleGetByIdDeep(battleid)
		if err != nil {
			msg := "ERROR: Couln't get the battle with the given id"
			http.Redirect(w, r, fmt.Sprintf("/battle/%d?res=%s", battleid, msg), http.StatusSeeOther)
			return
		}

		// clear all bots from that user for that battle before readding them here
		BattleUnlinkAllBotsForUser(user.ID, battleid)

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

// actually run the battle
func battleRunHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	battleid, err := strconv.Atoi(vars["id"])
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("500 - Invalid battle id"))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	redir_target := fmt.Sprintf("/battle/%d?res=%%s", battleid)

	switch r.Method {
	case "POST":
		r.ParseForm()

		log.Printf("running the battle with the id %d", battleid)
		log.Println("Someone submitted the following form:")
		log.Println(r.Form)

		// fetch the session and get the user
		session, _ := globalState.sessions.Get(r, "session")
		username := session.Values["username"]
		if username == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		user, err := UserGetUserFromUsername(username.(string))
		if err != nil {
			log_and_redir_with_msg(w, r, err, redir_target, "Could not get the id for your username")
			return
		}

		// open radare without input for building the bot
		// TODO(emile): configure a variable memsize for the arena
		r2p1, err := r2pipe.NewPipe("malloc://4096")
		if err != nil {
			panic(err)
		}
		defer r2p1.Close()

		// Fetch the battle information
		// This includes all bots linked to the battle
		log.Printf("user %+v wants to run the battle", user)
		fullDeepBattle, err := BattleGetByIdDeep(battleid)

		var botSources []string
		var rawOutput string

		// for each bot involved within the battle, we need to fetch it again, as the deep battle
		// fech doesn't fetch that deep (it fetches the batle and the corresponding bots, but only
		// their ids and names and not the archs and bits associated)
		for _, b := range fullDeepBattle.Bots {
			bot, err := BotGetById(b.ID)
			if err != nil {
				log.Println(err)
			}

			// define the command used to assemble the bot
			src := strings.ReplaceAll(bot.Source, "\r\n", "; ")
			radareCommand := fmt.Sprintf("rasm2 -a %s -b %s \"%+v\"", bot.Archs[0].Name, bot.Bits[0].Name, src)
			rawOutput += fmt.Sprintf("; %s\n", radareCommand)

			// assemble the bot
			bytecode, err := r2cmd(r2p1, radareCommand)
			if err != nil {
				http.Redirect(w, r, fmt.Sprintf("/battle/%d?res=%s", battleid, "err building bot"), http.StatusSeeOther)
				return
			}

			botSources = append(botSources, bytecode)
		}

		// TODO(emile): [L] implement some kind of queue

		// TODO(emile): [S] use the information given from the battle, such as the right arch and bits
		cmd := "e asm.arch=arm"
		output, _ := r2cmd(r2p1, cmd)
		rawOutput += fmt.Sprintf("[0x00000000]> %s\n%s", cmd, output)

		cmd = "e asm.bits=32"
		output, _ = r2cmd(r2p1, cmd)
		rawOutput += fmt.Sprintf("[0x00000000]> %s\n%s", cmd, output)

		cmd = "aei"
		output, _ = r2cmd(r2p1, cmd)
		rawOutput += fmt.Sprintf("[0x00000000]> %s\n%s", cmd, output)

		cmd = "aeim"
		output, _ = r2cmd(r2p1, cmd)
		rawOutput += fmt.Sprintf("[0x00000000]> %s\n%s", cmd, output)

		// TODO(emile): random offsets
		for i, s := range botSources {
			log.Printf("writing bot %d to 0x%d", i, 50*(i+1))
			cmd := fmt.Sprintf("wx %s @ 0x%d", s, 50*(i+1))
			_, _ = r2cmd(r2p1, cmd)
			rawOutput += fmt.Sprintf("[0x00000000]> %s\n", cmd)
		}

		// print the memory for some pleasing visuals
		cmd = fmt.Sprintf("pxc 100 @ 0x50")
		output, _ = r2cmd(r2p1, cmd) // print
		rawOutput += fmt.Sprintf("[0x00000000]> %s\n", cmd)
		fmt.Println(output)

		// init stack
		cmd = "aer PC = 0x50"
		_, _ = r2cmd(r2p1, cmd)
		rawOutput += fmt.Sprintf("[0x00000000]> %s\n", cmd)

		cmd = "aer SP = SP + 0x50"
		_, _ = r2cmd(r2p1, cmd)
		rawOutput += fmt.Sprintf("[0x00000000]> %s\n", cmd)

		output, _ = r2cmd(r2p1, "pxc 100 @ 0x50") // print
		fmt.Println(output)

		// define end conditions
		cmd = "e cmd.esil.todo=t theend=1"
		_, _ = r2cmd(r2p1, cmd)
		rawOutput += fmt.Sprintf("[0x00000000]> %s\n", cmd)
		cmd = "e cmd.esil.trap=t theend=1"
		_, _ = r2cmd(r2p1, cmd)
		rawOutput += fmt.Sprintf("[0x00000000]> %s\n", cmd)
		cmd = "e cmd.esil.intr=t theend=1"
		_, _ = r2cmd(r2p1, cmd)
		rawOutput += fmt.Sprintf("[0x00000000]> %s\n", cmd)
		cmd = "e cmd.esil.ioer=t theend=1"
		_, _ = r2cmd(r2p1, cmd)
		rawOutput += fmt.Sprintf("[0x00000000]> %s\n", cmd)

		// set the end condition to 0 initially
		cmd = "f theend=0"
		_, _ = r2cmd(r2p1, cmd)
		rawOutput += fmt.Sprintf("[0x00000000]> %s\n", cmd)

		// TODO(emile): find a sensible default for the max amount of rounds
		for i := 0; i < 1000; i++ {

			// this is architecture agnostic and just outputs the program counter
			rawOutput += fmt.Sprintf("[0x00000000]> ########################################################################\n")
			pc, _ := r2cmd(r2p1, "aer~$(arn PC)~[1]")
			arch, _ := r2cmd(r2p1, "e asm.arch")
			bits, _ := r2cmd(r2p1, "e asm.bits")
			rawOutput += fmt.Sprintf("[0x00000000]> # ROUND %d, PC=%s, arch=%s, bits=%s\n", i, pc, arch, bits)

			//  _, _ = r2cmd(r2p1, "aes") // step
			cmd = "aes"
			_, _ = r2cmd(r2p1, cmd)
			rawOutput += fmt.Sprintf("[0x00000000]> %s\n", cmd)

			// print the arena
			cmd := "pxc 100 @ 0x50"
			output, _ := r2cmd(r2p1, cmd) // print
			rawOutput += fmt.Sprintf("[0x00000000]> %s\n%s\n", cmd, output)
			fmt.Println(output)

			// TODO(emile): restore state

			// TODO(emile): check the end condition
			_, _ = r2cmd(r2p1, "?v 1+theend") // check end condition
		}

		BattleSaveRawOutput(battleid, rawOutput)

		msg := "Success!"
		http.Redirect(w, r, fmt.Sprintf("/battle/%d?res=%s#output", battleid, msg), http.StatusSeeOther)
	default:
		http.Redirect(w, r, "/", http.StatusMethodNotAllowed)
	}
}

// delete a battle
// TODO(emile): finish implementing the deletion of battles
func battleDeleteHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	battleid, err := strconv.Atoi(vars["id"])
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("500 - Invalid battle id"))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	redir_target := fmt.Sprintf("/battle/%d?res=%%s", battleid)

	switch r.Method {
	case "POST": // can't send a DELETE with pure HTML...

		// get the current user
		session, _ := globalState.sessions.Get(r, "session")
		username := session.Values["username"]
		if username == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		viewer, err := UserGetUserFromUsername(username.(string))
		if err != nil {
			log_and_redir_with_msg(w, r, err, redir_target, "Could not get the id for your username")
			return
		}

		// get the battle
		battle, err := BattleGetByIdDeep(int(battleid))
		if err != nil {
			log_and_redir_with_msg(w, r, err, redir_target, "Could not get the battle given the id provided")
			return
		}

		battle_owned_by_requesting_user := false
		for _, owner := range battle.Owners {

			// if the requesting users id is equal to an owners id, we're allowed to delete
			// the battle
			if viewer.ID == owner.ID {
				battle_owned_by_requesting_user = true
				break
			}
		}

		// check that the user that created the battle is the current user, if not, return
		if battle_owned_by_requesting_user == false {
			msg := "You aren't in the owners list of the battle, so you can't delete this battle"
			log_and_redir_with_msg(w, r, err, redir_target, msg)
			return
		}

		BattleDeleteID(battleid)

		msg := "Successfully deleted the battle"
		http.Redirect(w, r, fmt.Sprintf("/battle/%d?res=%s", battleid, msg), http.StatusSeeOther)
	default:
		log.Println("expected POST, got ", r.Method)
		http.Redirect(w, r, "/", http.StatusMethodNotAllowed)
	}
}
