package database

import (
	"log"
	"sync"

	"GoBot/core"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

var schema = `
CREATE TABLE IF NOT EXISTS userrole ( id INTEGER PRIMARY KEY AUTOINCREMENT , role INTEGER, user_id VARCHAR );
CREATE INDEX IF NOT EXISTS userrole_id_index ON userrole (user_id);
CREATE INDEX IF NOT EXISTS userrole_role_index ON userrole (role);

CREATE TABLE IF NOT EXISTS commandalias ( id INTEGER PRIMARY KEY AUTOINCREMENT , pmenabled INTEGER, group_id INTEGER, command VARCHAR, help VARCHAR, longhelp VARCHAR, value VARCHAR );
CREATE INDEX IF NOT EXISTS commandalias_command_index ON commandalias (command);
CREATE INDEX IF NOT EXISTS commandalias_pmenabled_index ON commandalias (pmenabled);
CREATE INDEX IF NOT EXISTS commandalias_group_index ON commandalias (group_id);

CREATE TABLE IF NOT EXISTS commandgroup ( id INTEGER PRIMARY KEY AUTOINCREMENT , parent INTEGER, command VARCHAR, help VARCHAR );
CREATE INDEX IF NOT EXISTS commandgroup_command_index ON commandgroup (command);
CREATE INDEX IF NOT EXISTS commandgroup_parent_index ON commandgroup (parent);
`

type CommandAlias struct {
	Id             int
	PMEnabled      bool
	GroupId        *int `db:"group_id"`
	Command, Value string
	Help, Longhelp *string
}

type CommandGroup struct {
	Id      int
	Parent  *int
	Command string
	Help    *string
}

type UserRole struct {
	Id     int
	Role   int
	UserId int `db:"user_id"`
}

var database *sqlx.DB
var mu sync.RWMutex

func InitalizeDatabase() {
	db, err := sqlx.Connect("sqlite3", core.Settings.Database())
	if err != nil {
		log.Fatal("Failed to create database", err)
	}

	// exec the schema or fail; multi-statement Exec behavior varies between
	// database drivers;  pq will exec them all, sqlite3 won't, ymmv
	db.MustExec(schema)
	database = db
}

func Close() {
	database.Close()
}

func FetchCommandAlias(cmd string) *CommandAlias {
	mu.RLock()
	defer mu.RUnlock()
	if database == nil {
		core.LogError("Database isn't open. Shouldn't happen.")
		return nil
	}
	command := CommandAlias{}
	err := database.Get(&command, "SELECT * FROM commandalias WHERE command=$1", cmd)
	if err != nil {
		core.LogErrorF("Failed to fetch command %s: %s", cmd, err)
		return nil
	}
	core.LogDebugF("Loaded command: %#v", command)
	return &command
}

func FetchCommandGroup(cmd string) *CommandGroup {
	mu.RLock()
	defer mu.RUnlock()
	if database == nil {
		core.LogError("Database isn't open. Shouldn't happen.")
		return nil
	}
	command := CommandGroup{}
	err := database.Get(&command, "SELECT * FROM commandgroup WHERE command=$1", cmd)
	if err != nil {
		core.LogErrorF("Failed to fetch command group %s: %s", cmd, err)
		return nil
	}
	core.LogDebugF("Loaded command group: %#v", command)
	return &command
}

func FetchCommandGroups() []CommandGroup {
	mu.RLock()
	defer mu.RUnlock()

	var groups []CommandGroup
	err := database.Select(&groups, "SELECT * FROM commandgroup ORDER BY command ASC")
	if err != nil {
		core.LogErrorF("Failed to fetch command groups: %s", err)
		return nil
	}
	core.LogDebugF("Loaded command groups: %#v", groups)
	return groups
}

func (c *CommandGroup) FetchCommands() []CommandAlias {
	mu.RLock()
	defer mu.RUnlock()

	var commands []CommandAlias
	err := database.Select(&commands, "SELECT * FROM commandalias WHERE group_id=$1 ORDER BY command ASC", c.Id)
	if err != nil {
		core.LogErrorF("Failed to fetch commands for command group %s: %s", c.Command, err)
		return nil
	}
	core.LogDebugF("Loaded command group [%s] commands: %#v", c.Command, commands)
	return commands
}

func FetchStandaloneCommands() []CommandAlias {
	mu.RLock()
	defer mu.RUnlock()

	var commands []CommandAlias
	err := database.Select(&commands, "SELECT * FROM commandalias WHERE group_id IS NULL ORDER BY command ASC")
	if err != nil {
		core.LogErrorF("Failed to fetch standalone commands: %s", err)
		return nil
	}
	core.LogDebugF("Loaded commands: %#v", commands)
	return commands
}
