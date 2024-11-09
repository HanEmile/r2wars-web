package main

import (
	"fmt"
	"html/template"
	"log"
	"math/rand"
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
	ArenaSize int
}

//////////////////////////////////////////////////////////////////////////////
// GENERAL PURPOSE

func BattleGetAll() ([]Battle, error) {
	return globalState.GetAllBattles()
}

func BattleCreate(battle Battle, owner User) (int, error) {
	return globalState.InsertBattle(battle, owner)
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

func BattleLinkOwnerIDs(battleid int, ownerIDs []int) error {
	return globalState.LinkOwnerIDsToBattle(battleid, ownerIDs)
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
	res, err := s.db.Exec(`
		INSERT INTO battles
		VALUES(NULL,?,?,?,?,?,?)
		`, time.Now(),
		battle.Name,
		battle.Public,
		battle.RawOutput,
		battle.MaxRounds,
		battle.ArenaSize)

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
	log.Println("Updating battle:")
	log.Println(battle.ArenaSize)
	_, err := s.db.Exec(`
		UPDATE battles
		SET name=?, public=?, arena_size=?, max_rounds=?
		WHERE id=?`,
		battle.Name,
		battle.Public,
		battle.ArenaSize,
		battle.MaxRounds,
		battle.ID)
	if err != nil {
		log.Println(err)
		return err
	}
	log.Println("Done updating")
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

func (s *State) LinkOwnerBattle(battleid int, ownerid int) error {
	_, err := s.db.Exec("INSERT INTO owner_battle_rel VALUES (?, ?)", ownerid, battleid)
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
	_, err := s.db.Exec("DELETE FROM arch_battle_rel WHERE battle_id=?", battleid)
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
	_, err := s.db.Exec("DELETE FROM bit_battle_rel WHERE battle_id=?", battleid)
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

func (s *State) LinkOwnerIDsToBattle(battleid int, ownerIDs []int) error {
	// delete preexisting links
	_, err := s.db.Exec("DELETE FROM owner_battle_rel WHERE battle_id=?", battleid)
	if err != nil {
		log.Println(err)
		return err
	}

	// yes, we're building this by hand, but as we only insert int's I'm just confident that whoever
	// gets some sqli here just deserves it :D
	query := "INSERT INTO owner_battle_rel (user_id, battle_id) VALUES"
	for idx, id := range ownerIDs {
		query += fmt.Sprintf("(%d, %d)", id, battleid)
		if idx != len(ownerIDs)-1 {
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
	var battlearenasize int

	var botids string
	var botnames string

	var userids string
	var usernames string

	var archids string
	var archnames string

	var bitids string
	var bitnames string

	var ownerids string
	var ownernames string

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
		COALESCE(ba.arena_size, 4096),

		COALESCE(group_concat(DISTINCT bb.bot_id), ""),
		COALESCE(group_concat(DISTINCT bo.name), ""),

		COALESCE(group_concat(DISTINCT ub.user_id), ""),
		COALESCE(group_concat(DISTINCT us.name), ""),

		COALESCE(group_concat(DISTINCT ab.arch_id), ""),
		COALESCE(group_concat(DISTINCT ar.name), ""),

		COALESCE(group_concat(DISTINCT bitbat.bit_id), ""),
		COALESCE(group_concat(DISTINCT bi.name), ""),

		COALESCE(group_concat(DISTINCT ownerbat.user_id), ""),
		COALESCE(group_concat(DISTINCT owner.name), "")
	FROM battles ba

	LEFT JOIN bot_battle_rel bb ON bb.battle_id = ba.id
	LEFT JOIN bots bo ON bo.id = bb.bot_id

	LEFT JOIN user_battle_rel ub ON ub.battle_id = ba.id
	LEFT JOIN users us ON us.id = ub.user_id

	LEFT JOIN arch_battle_rel ab ON ab.battle_id = ba.id
	LEFT JOIN archs ar ON ar.id = ab.arch_id

	LEFT JOIN bit_battle_rel bitbat ON bitbat.battle_id = ba.id
	LEFT JOIN bits bi ON bi.id = bitbat.bit_id

	LEFT JOIN owner_battle_rel ownerbat ON ownerbat.battle_id = ba.id
	LEFT JOIN users owner ON owner.id = ownerbat.user_id

	WHERE ba.id=?
	GROUP BY ba.id;
	`, id).Scan(&battleid, &battlename, &battlepublic, &battlerawoutput, &battlemaxrounds, &battlearenasize, &botids, &botnames, &userids, &usernames, &archids, &archnames, &bitids, &bitnames, &ownerids, &ownernames)
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

	// assemble the owners
	ownerIDList := strings.Split(ownerids, ",")
	ownerNameList := strings.Split(ownernames, ",")

	var owners []User
	if ownerIDList[0] != "" {
		for i := range ownerIDList {
			id, err := strconv.Atoi(ownerIDList[i])
			if err != nil {
				log.Println(err)
				return Battle{}, err
			}
			owners = append(owners, User{id, ownerNameList[i], nil})
		}
	} else {
		owners = []User{}
	}

	return Battle{
		ID:        battleid,
		Name:      battlename,
		Bots:      bots,
		Owners:    owners,
		Public:    battlepublic,
		Archs:     archs,
		Bits:      bits,
		RawOutput: battlerawoutput,
		MaxRounds: battlemaxrounds,
		ArenaSize: battlearenasize,
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

		users, err := UserGetAll()
		if err != nil {
			log.Println(err)
			data["err"] = "Could not fetch all users"
		} else {
			data["users"] = users
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
		err = t.ExecuteTemplate(w, "battleNew", data)
		if err != nil {
			log.Println(err)
		}

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
		arenasize, err := strconv.Atoi(r.Form.Get("arena-size"))
		if err != nil {
			// TODO(emile): use the log_and_redir function in here (and the surrounding code)

			log.Println(err)
			msg := "ERROR: Invalid arch id"
			http.Redirect(w, r, fmt.Sprintf("/battle/new?res=%s", msg), http.StatusSeeOther)
			return
		}
		maxrounds, err := strconv.Atoi(r.Form.Get("max-rounds"))
		if err != nil {
			// TODO(emile): use the log_and_redir function in here (and the surrounding code)

			log.Println(err)
			msg := "ERROR: Invalid arch id"
			http.Redirect(w, r, fmt.Sprintf("/battle/new?res=%s", msg), http.StatusSeeOther)
			return
		}

		var public bool
		query_public := r.Form.Get("public")
		if query_public == "on" {
			public = true
		}

		// gather the information from the arch and bit selection
		var archIDs []int
		var bitIDs []int

		for k := range r.Form {
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
			newbattle := Battle{
				0,
				name,
				nil,
				nil,
				public,
				nil,
				nil,
				"",
				maxrounds,
				arenasize,
			}
			battleid, err := BattleCreate(newbattle, user)
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

			// link owner to battle
			err = BattleLinkOwnerIDs(battleid, []int{user.ID})
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

		arenasize, err := strconv.Atoi(r.Form.Get("arena-size"))
		if err != nil {
			// TODO(emile): use the log_and_redir function in here (and the surrounding code)

			log.Println(err)
			msg := "ERROR: Invalid arch id"
			http.Redirect(w, r, fmt.Sprintf("/battle/new?res=%s", msg), http.StatusSeeOther)
			return
		}
		maxrounds, err := strconv.Atoi(r.Form.Get("max-rounds"))
		if err != nil {
			// TODO(emile): use the log_and_redir function in here (and the surrounding code)

			log.Println(err)
			msg := "ERROR: Invalid arch id"
			http.Redirect(w, r, fmt.Sprintf("/battle/new?res=%s", msg), http.StatusSeeOther)
			return
		}

		// gather the information from the arch and bit selection
		var botIDs []int

		for k := range r.Form {
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
		newbattle := Battle{
			0,
			fmt.Sprintf("quick-%d", rand.Intn(10000)),
			nil,
			nil,
			public,
			nil,
			nil,
			"",
			maxrounds,
			arenasize,
		}
		battleid, err := BattleCreate(newbattle, user)
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

		users, err := UserGetAll()
		if err != nil {
			log_and_redir_with_msg(w, r, err, redir_target, "Could not get the users")
			return
		}
		data["users"] = users

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
		arenasize, err := strconv.Atoi(r.Form.Get("arena-size"))
		if err != nil {
			log_and_redir_with_msg(w, r, err, redir_target, "Invalid Arena size")
			return
		}

		var public bool
		if r.Form.Get("public") == "on" {
			public = true
		}

		// gather the information from the arch and bit selection
		var archIDs []int
		var bitIDs []int
		var ownerIDs []int

		log.Println(r.Form)

		for k := range r.Form {
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
			if strings.HasPrefix(k, "owner-") {
				id, err := strconv.Atoi(strings.TrimPrefix(k, "owner-"))
				if err != nil {
					log_and_redir_with_msg(w, r, err, redir_target, "Invalid Owner ID")
					return
				}
				ownerIDs = append(ownerIDs, id)
			}
		}

		// CHECK THAT THE USER REQUESTING CHANGES IS ALREADY PART OF THE OWNERS
		allowedToEdit := false
		for _, ownerID := range ownerIDs {
			if user.ID == ownerID {
				allowedToEdit = true
			}
		}
		if allowedToEdit == false {
			log_and_redir_with_msg(w, r, err, redir_target+"#settings", "You aren't an owner and aren't allowed to edit the settings")
			return
		}

		// DATABASE MANIPUTLATION BELOW

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

		// link bits to battle
		err = BattleLinkOwnerIDs(battleid, ownerIDs)
		if err != nil {
			log_and_redir_with_msg(w, r, err, redir_target+"#settings", "Could not link owner id to battle")
			return
		}

		new_battle := Battle{int(battleid), form_name, []Bot{}, []User{user}, public, []Arch{}, []Bit{}, "", 100, arenasize}

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
		for k := range r.Form {
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

		// Fetch the battle information
		// This includes all bots linked to the battle
		log.Printf("user %+v wants to run the battle", user)
		fullDeepBattle, err := BattleGetByIdDeep(battleid)

		// open radare without input for building the bot
		// TODO(emile): configure a variable memsize for the arena
		cmd := fmt.Sprintf("malloc://%d", fullDeepBattle.ArenaSize)
		r2p1, err := r2pipe.NewPipe(cmd)
		if err != nil {
			panic(err)
		}
		defer r2p1.Close()

		var botSources []string
		var rawOutput string

		cmd = fmt.Sprintf("pxc %d @ 0x0", fullDeepBattle.ArenaSize)
		output, _ := r2cmd(r2p1, cmd)
		rawOutput += fmt.Sprintf("[0x00000000]> %s\n%s\n", cmd, output)

		// TODO(emile): currently hardcoded to two bots, extract this anonymous struct into a named struct and make this work with > 2 bots
		runtimeBots := []struct {
			Name     string
			Regs     string
			BaseAddr int
			ArchName string
			BitsName string
		}{
			{Name: "", Regs: "", BaseAddr: 0, ArchName: "", BitsName: ""},
			{Name: "", Regs: "", BaseAddr: 0, ArchName: "", BitsName: ""},
		}

		rawOutput += "[0x00000000]> # Assembling the bots\n"

		// for each bot involved within the battle, we need to fetch it again, as the deep battle
		// fech doesn't fetch that deep (it fetches the batle and the corresponding bots, but only
		// their ids and names and not the archs and bits associated)
		for i, b := range fullDeepBattle.Bots {
			bot, err := BotGetById(b.ID)
			if err != nil {
				log.Println(err)
			}

			runtimeBots[i].Name = bot.Name

			// TODO(emile): a bot can have multiple archs/bits, figure out what to do then
			// I've just gone and used the first one, as a bot alwas has at least one...
			// ...it has right?
			runtimeBots[i].ArchName = bot.Archs[0].Name
			runtimeBots[i].BitsName = bot.Bits[0].Name

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

		rawOutput += "[0x00000000]> # initializing the vm and the stack\n"
		cmd = "aei"
		output, _ = r2cmd(r2p1, cmd)
		rawOutput += fmt.Sprintf("[0x00000000]> %s\n%s", cmd, output)

		cmd = "aeim"
		output, _ = r2cmd(r2p1, cmd)
		rawOutput += fmt.Sprintf("[0x00000000]> %s\n%s", cmd, output)

		// TODO(emile): random offsets
		// place bots
		for i, s := range botSources {

			// the address to write the bot to
			addr := 50 * (i + 1)

			// store it
			runtimeBots[i].BaseAddr = addr

			msg := fmt.Sprintf("# writing bot %d to 0x%d", i, addr)
			rawOutput += fmt.Sprintf("[0x00000000]> %s\n", msg)
			cmd := fmt.Sprintf("wx %s @ 0x%d", s, addr)
			_, _ = r2cmd(r2p1, cmd)
			rawOutput += fmt.Sprintf("[0x00000000]> %s\n", cmd)

			// define the instruction point and the stack pointer
			rawOutput += "[0x00000000]> # Setting the program counter and the stack pointer\n"
			cmd = fmt.Sprintf("aer PC=0x%d", addr)
			_, _ = r2cmd(r2p1, cmd)
			rawOutput += fmt.Sprintf("[0x00000000]> %s\n", cmd)

			cmd = fmt.Sprintf("aer SP=SP+0x%d", addr)
			_, _ = r2cmd(r2p1, cmd)
			rawOutput += fmt.Sprintf("[0x00000000]> %s\n", cmd)

			// dump the registers of the bot for being able to switch inbetween them
			// This is done in order to be able to play one step of each bot at a time,
			// but sort of in parallel
			rawOutput += "[0x00000000]> # Storing registers\n"
			cmd = "aerR"
			regs, _ := r2cmd(r2p1, cmd)
			rawOutput += fmt.Sprintf("[0x00000000]> %s\n", cmd)

			initialRegisers := strings.Replace(regs, "\n", ";", -1)
			runtimeBots[i].Regs = initialRegisers
		}

		for i := range botSources {
			// print the memory for some pleasing visuals
			cmd = fmt.Sprintf("pxc 100 @ 0x%d", runtimeBots[i].BaseAddr)
			output, _ = r2cmd(r2p1, cmd) // print
			rawOutput += fmt.Sprintf("[0x00000000]> %s\n%s\n", cmd, output)
		}

		output, _ = r2cmd(r2p1, "pxc 100 @ 0x50") // print
		fmt.Println(output)

		// define end conditions
		rawOutput += "[0x00000000]> # Defining the end conditions\n"
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
		rawOutput += "[0x00000000]> # Initializing the end condition variable\n"
		cmd = "f theend=0"
		_, _ = r2cmd(r2p1, cmd)
		rawOutput += fmt.Sprintf("[0x00000000]> %s\n", cmd)

		currentBotId := 0

		// TODO(emile): find a sensible default for the max amount of rounds
		for i := 0; i < fullDeepBattle.MaxRounds; i++ {

			currentBotId = i % 2

			rawOutput += fmt.Sprintf("[0x00000000]> ########################################################################\n")

			// this is architecture agnostic and just gets the program counter
			pc, _ := r2cmd(r2p1, "aer~$(arn PC)~[1]")

			arch, _ := r2cmd(r2p1, "e asm.arch")
			bits, _ := r2cmd(r2p1, "e asm.bits")
			rawOutput += fmt.Sprintf("[0x00000000]> # ROUND %d, BOT %d (%s), PC=%s, arch=%s, bits=%s\n", i, currentBotId, runtimeBots[currentBotId].Name, pc, arch, bits)

			rawOutput += "[0x00000000]> # setting the architecture accordingly\n"
			cmd = fmt.Sprintf("e asm.arch=%s", runtimeBots[currentBotId].ArchName)
			output, _ = r2cmd(r2p1, cmd)
			rawOutput += fmt.Sprintf("[0x00000000]> %s\n%s", cmd, output)

			cmd = fmt.Sprintf("e asm.bits=%s", runtimeBots[currentBotId].BitsName)
			output, _ = r2cmd(r2p1, cmd)
			rawOutput += fmt.Sprintf("[0x00000000]> %s\n%s", cmd, output)

			// load registers
			rawOutput += "[0x00000000]> # Loading the registers\n"
			r2cmd(r2p1, runtimeBots[currentBotId].Regs)
			//  rawOutput += fmt.Sprintf("%+v\n", runtimeBots[currentBotId].Regs)

			//  cmd = "dr"
			//  output, _ = r2cmd(r2p1, cmd)
			//  rawOutput += fmt.Sprintf("[0x00000000]> %s\n%s\n", cmd, output)

			//  _, _ = r2cmd(r2p1, "aes") // step
			rawOutput += "[0x00000000]> # Stepping\n"
			cmd = "aes"
			_, _ = r2cmd(r2p1, cmd)
			rawOutput += fmt.Sprintf("[0x00000000]> %s\n", cmd)

			// store the regisers
			rawOutput += "[0x00000000]> # Storing the registers\n"
			registers, _ := r2cmd(r2p1, "aerR")
			registersStripped := strings.Replace(registers, "\n", ";", -1)
			runtimeBots[currentBotId].Regs = registersStripped

			// print the arena
			rawOutput += "[0x00000000]> # Printing the arena\n"
			cmd := fmt.Sprintf("pxc 100 @ 0x%d", runtimeBots[currentBotId].BaseAddr)
			output, _ := r2cmd(r2p1, cmd) // print
			rawOutput += fmt.Sprintf("[0x00000000]> %s\n%s\n", cmd, output)
			//  fmt.Println(output)

			// predicate - the end?
			rawOutput += "[0x00000000]> # Checking if we've won\n"
			pend, _ := r2cmd(r2p1, "?v theend")
			status := strings.TrimSpace(pend)
			// fixme: on Windows, we sometimes get output *from other calls to r2*

			if status == "0x1" {
				log.Printf("[!] Bot %d has died", currentBotId)
			}
			if status != "0x0" {
				log.Printf("[!] Got invalid status '%s' for bot %d", status, currentBotId)
			}
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
