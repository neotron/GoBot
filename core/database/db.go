package database

import (
	"database/sql"
	"errors"
	"fmt"
	"log"

	"GoBot/core"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

type FieldName string
type TableName string

const (
	HelpField      FieldName = "help"
	LongHelpField  FieldName = "longhelp"
	PMEnabledField FieldName = "pmenabled"
	ValueField     FieldName = "value"
	GroupIdField   FieldName = "group_id"
	ParentField    FieldName = "parent"
	CommandField   FieldName = "command"
	RoleField      FieldName = "role"
	UserIdField    FieldName = "user_id"

	CommandAliasTable TableName = "commandalias"
	CommandGroupTable TableName = "commandgroup"
	UserRoleTable TableName = "userrole"
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
	Id             int64
	PMEnabled      bool
	GroupId        *int `db:"group_id"`
	Command, Value string
	Help, Longhelp *string
}

type CommandGroup struct {
	Id      int64
	Parent  *int
	Command string
	Help    *string
}

type UserRole struct {
	Id     int64
	Role   int
	UserId int `db:"user_id"`
}
type count struct {
	Count int64
}

var database *sqlx.DB

func InitalizeDatabase() {
	db, err := sqlx.Connect("sqlite3", core.Settings.Database())
	if err != nil {
		log.Fatal("Failed to create database", err)
	}

	// exec the schema or fail; multi-statement Exec behavior varies between
	// database drivers;  pq will exec them all, sqlite3 won't, ymmv
	db.MustExec(schema)
	database = db

	// Initialize carrier table
	InitializeCarrierTable()
}

func Close() {
	database.Close()
}

func FetchCommandAlias(cmd string) *CommandAlias {
	if database == nil {
		core.LogError("Database isn't open. Shouldn't happen.")
		return nil
	}
	command := CommandAlias{}
	err := database.Get(&command, "SELECT * FROM commandalias WHERE command=$1", cmd)
	switch err {
	default:
		core.LogErrorF("Failed to fetch count %s: %s", cmd, err)
		fallthrough
	case sql.ErrNoRows:
		return nil
	case nil:
		return &command
	}
}

func HasCommandAlias(cmd string) bool {
	if database == nil {
		core.LogError("Database isn't open. Shouldn't happen.")
		return false
	}
	count := count{}
	err := database.Get(&count, "SELECT count(*) count FROM commandalias WHERE command=$1", cmd)
	switch err {
	default:
		core.LogErrorF("Failed to fetch count %s: %s", cmd, err)
		fallthrough
	case sql.ErrNoRows:
		return false
	case nil:
		return count.Count > 0
	}
}

type executeFunc func(tx *sql.Tx) (sql.Result, error)

func executeAndCommit(action executeFunc) (res sql.Result, err error) {
	if database == nil {
		err = errors.New("database not open")
		return
	}
	tx, err := database.Begin()
	if err != nil {
		return
	}
	res, err = action(tx)

	if err != nil {
		tx.Rollback()
		return
	}
	err = tx.Commit()
	if err != nil {
		tx.Rollback()
	}
	return
}

func RemoveCommandAlias(cmd string) bool {
	res, err := executeAndCommit(func(tx *sql.Tx) (sql.Result, error) {
		return tx.Exec("DELETE FROM commandalias where command = $1", cmd)
	})
	switch err {
	default:
		core.LogError("Failed to remove command: ", err)
		fallthrough
	case sql.ErrNoRows:
		return false
	case nil:
		affected, err := res.RowsAffected()
		return err == nil && affected > 0
	}
}

func CreateCommandAlias(cmd, val string) bool {
	_, err := executeAndCommit(func(tx *sql.Tx) (sql.Result, error) {
		return tx.Exec("INSERT INTO commandalias (command, value, pmenabled) VALUES ($1, $2, FALSE)", cmd, val)
	})
	if err != nil {
		core.LogError("Failed to insert command: ", err)
		return false
	}
	return true
}

func updateTable(table TableName, whereKey FieldName, whereVal interface{}, field FieldName, val interface{}) bool {
	res, err := executeAndCommit(func(tx *sql.Tx) (sql.Result, error) {
		if val == nil {
			return tx.Exec(fmt.Sprintf("UPDATE %s SET %s = NULL WHERE %s = $1", table, field, whereKey), whereVal)
		} else {
			return tx.Exec(fmt.Sprintf("UPDATE %s SET %s = $1 WHERE %s = $2", table, field, whereKey), val, whereVal)
		}
	})
	if err != nil {
		core.LogError("Failed to update command: ", err)
		return false
	}
	numRows, err := res.RowsAffected()
	if err != nil {
		core.LogError("Failed to fetch affected rows: ", err)
		return false
	}
	return numRows > 0
}

func UpdateCommandAlias(whereKey FieldName, whereVal interface{}, field FieldName, val interface{}) bool {
	return updateTable(CommandAliasTable, whereKey, whereVal, field, val)
}

func UpdateCommandGroup(whereKey FieldName, whereVal interface{}, field FieldName, val interface{}) bool {
	return updateTable(CommandGroupTable, whereKey, whereVal, field, val)
}

func HasCommandGroup(cmd string) bool {
	if database == nil {
		core.LogError("Database isn't open. Shouldn't happen.")
		return false
	}
	count := count{}
	err := database.Get(&count, "SELECT count(*) count FROM commandgroup WHERE command=$1", cmd)
	switch err {
	default:
		core.LogErrorF("Failed to fetch count %s: %s", cmd, err)
		fallthrough
	case sql.ErrNoRows:
		return false
	case nil:
		return count.Count > 0
	}
}

func RemoveCommandGroup(cmd string) bool {
	res, err := executeAndCommit(func(tx *sql.Tx) (sql.Result, error) {
		return tx.Exec("DELETE FROM commandgroup where command = $1", cmd)
	})
	switch err {
	default:
		core.LogError("Failed to remove command group: ", err)
		fallthrough
	case sql.ErrNoRows:
		return false
	case nil:
		affected, err := res.RowsAffected()
		return err == nil && affected > 0
	}
}

func FetchCommandGroup(cmd string) *CommandGroup {
	if database == nil {
		core.LogError("Database isn't open. Shouldn't happen.")
		return nil
	}
	command := CommandGroup{}
	err := database.Get(&command, "SELECT * FROM commandgroup WHERE command=$1", cmd)
	switch err {
	default:
		core.LogErrorF("Failed to fetch command group %s: %s", cmd, err)
		fallthrough
	case sql.ErrNoRows:
		return nil
	case nil:
		return &command
	}
}
func FetchOrCreateCommandGroup(cmd string) *CommandGroup {
	command := FetchCommandGroup(cmd)
	if command == nil {
		// Try to create a new one
		command = &CommandGroup{Command: cmd}
		res, err := executeAndCommit(func(tx *sql.Tx) (sql.Result, error) {
			return tx.Exec("INSERT INTO commandgroup (command) VALUES ($1)", cmd)
		})
		if err != nil {
			core.LogErrorF("Failed to create new command group %s.", cmd)
			return nil
		}
		command.Id, err = res.LastInsertId()
		if err != nil {
			core.LogErrorF("Failed to get last insert id for command group, attempting fetch.")
			command = FetchCommandGroup(cmd)
		}
	}
	return command
}

func FetchCommandGroups() []CommandGroup {
	var groups []CommandGroup
	err := database.Select(&groups, "SELECT * FROM commandgroup ORDER BY command ASC")
	switch err {
	default:
		core.LogErrorF("Failed to fetch command groups: %s", err)
		fallthrough
	case sql.ErrNoRows:
		return nil
	case nil:
		return groups
	}
}

func (c *CommandGroup) FetchCommands() []CommandAlias {
	var commands []CommandAlias
	err := database.Select(&commands, "SELECT * FROM commandalias WHERE group_id=$1 ORDER BY command ASC", c.Id)
	switch err {
	default:
		core.LogErrorF("Failed to fetch commands group: %s", err)
		fallthrough
	case sql.ErrNoRows:
		return nil
	case nil:
		return commands
	}
}

func FetchStandaloneCommands() []CommandAlias {
	var commands []CommandAlias
	err := database.Select(&commands, "SELECT * FROM commandalias WHERE group_id IS NULL ORDER BY command ASC")
	switch err {
	default:
		core.LogErrorF("Failed to fetch standalone commandsXS: %s", err)
		fallthrough
	case sql.ErrNoRows:
		return nil
	case nil:
		return commands
	}
}
