package hal

import (
	"bytes"
	"log"
	"strings"
)

// provides a persistent configuration store

// Order of precendence for prefs:
// user -> channel -> broker -> plugin -> global -> default

// PREFS_TABLE contains the SQL to create the prefs table
// key field is called pkey because key is a reserved word
const PREFS_TABLE = `
CREATE TABLE IF NOT EXISTS prefs (
	 user    VARCHAR(32) DEFAULT "",
	 channel VARCHAR(32) DEFAULT "",
	 broker  VARCHAR(32) DEFAULT "",
	 plugin  VARCHAR(32) DEFAULT "",
	 pkey    VARCHAR(32) NOT NULL,
	 value   TEXT,
	 PRIMARY KEY(user, channel, broker, plugin, pkey)
)`

/*
   -- test data, will remove once there are automated tests
   INSERT INTO prefs (user,channel,broker,plugin,pkey,value) VALUES ("tobert", "", "", "", "foo", "user");
   INSERT INTO prefs (user,channel,broker,plugin,pkey,value) VALUES ("tobert", "CORE", "", "", "foo",
   "user-channel");
   INSERT INTO prefs (user,channel,broker,plugin,pkey,value) VALUES ("tobert", "CORE", "slack", "", "foo",
   "user-channel-broker");
   INSERT INTO prefs (user,channel,broker,plugin,pkey,value) VALUES ("tobert", "CORE", "slack", "uptime",
   "foo", "user-channel-broker-plugin");
   INSERT INTO prefs (user,channel,broker,plugin,pkey,value) VALUES ("tobert", "", "slack", "uptime", "foo",
   "user-broker-plugin");
   INSERT INTO prefs (user,channel,broker,plugin,pkey,value) VALUES ("tobert", "CORE", "", "uptime",
   "foo", "user-channel-plugin");
   INSERT INTO prefs (user,channel,broker,plugin,pkey,value) VALUES ("tobert", "", "", "uptime", "foo",
   "user-plugin");
*/

// !prefs list --scope plugin --plugin autoresponder
// !prefs get --scope channel --plugin autoresponder --channel CORE --key timezone
// !prefs set --scope user --plugin autoresponder --channel CORE

// Pref is a key/value pair associated with a combination of user, plugin,
// borker, or channel.
type Pref struct {
	User    string
	Plugin  string
	Broker  string
	Channel string
	Key     string
	Value   string
	Default string
	Success bool
	Error   error
}

type Prefs []*Pref

// GetPref will retreive the most-specific preference from pref
// storage using the parameters provided. This is a bit like pattern
// matching. If no match is found, the provided default is returned.
// TODO: explain this better
func GetPref(user, broker, channel, plugin, key, def string) Pref {
	pref := Pref{
		User:    user,
		Channel: channel,
		Broker:  broker,
		Plugin:  plugin,
		Key:     key,
		Default: def,
	}

	up := pref.Get()
	if up.Success {
		return up
	}

	// no match, return the default
	pref.Value = def
	return pref
}

// GetPrefs retrieves a set of preferences from the database. The
// settings are matched exactly on user,broker,channel,plugin.
// e.g. GetPrefs("", "", "", "uptime") would get only records that
// have user/broker/channel set to the empty string and channel
// set to "uptime". A record with user "pford" and plugin "uptime"
// would not be included.
func GetPrefs(user, broker, channel, plugin string) Prefs {
	pref := Pref{
		User:    user,
		Broker:  broker,
		Channel: channel,
		Plugin:  plugin,
	}
	return pref.get()
}

// FindPrefs gets all records that match any of the inputs that are
// not empty strings. (hint: user="x", broker="y"; WHERE user=? OR broker=?)
func FindPrefs(user, broker, channel, plugin, key string) Prefs {
	pref := Pref{
		User:    user,
		Broker:  broker,
		Channel: channel,
		Plugin:  plugin,
		Key:     key,
	}
	return pref.Find()
}

func GetUserPrefs(user string) Prefs {
	pref := Pref{}
	pref.User = user
	return pref.get()
}

func GetChannelPrefs(channel string) Prefs {
	pref := Pref{}
	pref.Channel = channel
	return pref.get()
}

func GetBrokerPrefs(broker string) Prefs {
	pref := Pref{}
	pref.Broker = broker
	return pref.get()
}

func GetPluginPrefs(plugin string) Prefs {
	pref := Pref{}
	pref.Plugin = plugin
	return pref.get()
}

// Get retrieves a value from the database. If the database returns
// an error, Success will be false and the Error field will be populated.
func (in *Pref) Get() Pref {
	prefs := in.get()

	if len(prefs) == 1 {
		return *prefs[0]
	} else if len(prefs) > 1 {
		panic("TOO MANY PREFS")
	} else if len(prefs) == 0 {
		out := *in
		out.Success = false
		return out
	}

	panic("BUG: should be impossible to reach this point")
}

func (in *Pref) get() Prefs {
	SqlInit(PREFS_TABLE)

	sql := `SELECT user,channel,broker,plugin,pkey,value
	        FROM prefs
	        WHERE user=?
			  AND channel=?
			  AND broker=?
			  AND plugin=?`
	params := []interface{}{&in.User, &in.Channel, &in.Broker, &in.Plugin}

	// only query by key if it's specified, otherwise get all keys for the selection
	if in.Key != "" {
		sql += " AND pkey=?"
		params = append(params, &in.Key)
	}

	db := SqlDB()

	rows, err := db.Query(sql, params...)
	if err != nil {
		log.Printf("Returning default due to SQL query failure: %s", err)
		return Prefs{}
	}

	defer rows.Close()

	out := make(Prefs, 0)

	for rows.Next() {
		p := *in

		err := rows.Scan(&p.User, &p.Channel, &p.Broker, &p.Plugin, &p.Key, &p.Value)

		if err != nil {
			log.Printf("Returning default due to row iteration failure: %s", err)
			p.Success = false
			p.Value = in.Default
			p.Error = err
		}

		out = append(out, &p)
	}

	return out
}

// Set writes the value and returns a new struct with the new value.
func (in *Pref) Set() Pref {
	db := SqlDB()
	SqlInit(PREFS_TABLE)

	sql := `INSERT INTO prefs
						(value,user,channel,broker,plugin,pkey)
			VALUES (?,?,?,?,?,?)
			ON DUPLICATE KEY
			UPDATE value=?, user=?, channel=?, broker=?, plugin=?, pkey=?`

	params := []interface{}{
		&in.Value, &in.User, &in.Channel, &in.Broker, &in.Plugin, &in.Key,
		&in.Value, &in.User, &in.Channel, &in.Broker, &in.Plugin, &in.Key,
	}

	_, err := db.Exec(sql, params...)
	if err != nil {
		out := *in
		out.Success = false
		out.Error = err
		return out
	}

	return in.Get()
}

// Find retrieves all preferences from the database that match any field in the
// handle's fields.
// Unlike Get(), empty string fields are not included in the (generated) query
// so it can potentially match a lot of rows.
// Returns an empty list and logs upon errors.
func (p Pref) Find() Prefs {
	SqlInit(PREFS_TABLE)

	fields := make([]string, 0)
	params := make([]interface{}, 0)

	if p.User != "" {
		fields = append(fields, "user=?")
		params = append(params, p.User)
	}

	if p.Channel != "" {
		fields = append(fields, "channel=?")
		params = append(params, p.Channel)
	}

	if p.Broker != "" {
		fields = append(fields, "broker=?")
		params = append(params, p.Broker)
	}

	if p.Plugin != "" {
		fields = append(fields, "plugin=?")
		params = append(params, p.Plugin)
	}

	if p.Key != "" {
		fields = append(fields, "pkey=?")
		params = append(params, p.Key)
	}

	q := bytes.NewBufferString("SELECT user,channel,broker,plugin,pkey,value\n")
	q.WriteString("FROM prefs\n")

	// TODO: maybe it's silly to make it easy for Find() to get all preferences
	// but let's cross that bridge when we come to it
	if len(fields) > 0 {
		q.WriteString("\nWHERE ")
		// might make sense to add a param to this func to make it easy to
		// switch this between AND/OR for unions/intersections
		q.WriteString(strings.Join(fields, "\n  OR "))
	}

	// TODO: add deterministic ordering at query time

	db := SqlDB()
	out := make(Prefs, 0)
	rows, err := db.Query(q.String(), params...)
	if err != nil {
		log.Println(q.String())
		log.Printf("Query failed: %s", err)
		return out
	}
	defer rows.Close()

	for rows.Next() {
		row := Pref{}
		err = rows.Scan(&row.User, &row.Channel, &row.Broker, &row.Plugin, &row.Key, &row.Value)
		// improbable in practice - follows previously mentioned conventions for errors
		if err != nil {
			log.Printf("Fetching a row failed: %s\n", err)
			row.Error = err
			row.Success = false
			row.Value = p.Default
		} else {
			row.Error = nil
			row.Success = true
		}

		out = append(out, &row)
	}

	return out
}

// User filters the preference list by user, returning a new Prefs
// e.g. uprefs = prefs.User("adent")
func (prefs Prefs) User(user string) Prefs {
	out := make(Prefs, 0)

	for _, pref := range prefs {
		if pref.User == user {
			out = append(out, pref)
		}
	}

	return out
}

// Channel filters the preference list by channel, returning a new Prefs
// e.g. instprefs = prefs.Channel("magrathea").Plugin("uptime").Broker("slack")
func (prefs Prefs) Channel(channel string) Prefs {
	out := make(Prefs, 0)

	for _, pref := range prefs {
		if pref.Channel == channel {
			out = append(out, pref)
		}
	}

	return out
}

// Broker filters the preference list by broker, returning a new Prefs
func (prefs Prefs) Broker(broker string) Prefs {
	out := make(Prefs, 0)

	for _, pref := range prefs {
		if pref.Broker == broker {
			out = append(out, pref)
		}
	}

	return out
}

// Plugin filters the preference list by plugin, returning a new Prefs
func (prefs Prefs) Plugin(plugin string) Prefs {
	out := make(Prefs, 0)

	for _, pref := range prefs {
		if pref.Plugin == plugin {
			out = append(out, pref)
		}
	}

	return out
}

// ready to hand off to e.g. hal.AsciiTable()
func (prefs Prefs) Table() [][]string {
	out := make([][]string, 1)
	out[0] = []string{"User", "Channel", "Broker", "Plugin", "Key", "Value"}

	for _, pref := range prefs {
		m := []string{
			pref.User,
			pref.Channel,
			pref.Broker,
			pref.Plugin,
			pref.Key,
			pref.Value,
		}

		out = append(out, m)
	}

	return out
}