package mysql

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strings"

	// mysql driver
	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
)

// DB wraps a SQL database with specific functionality
type DB struct {
	db            *sql.DB
	name          string
	dsn           string
	autoCreate    bool
	dropExisting  bool
	migrationsDir string
	migrations    []string
	dropOnClose   bool

	Users         *userStore
	Entries       *entryStore
	RefreshTokens *refreshTokenStore
}

// DBWithTx wraps a DB with a sql Tx.
type DBWithTx struct {
	*DB
	tx *sql.Tx
}

func (tx *DBWithTx) Rollback() error {
	return tx.tx.Rollback()
}

func (tx *DBWithTx) Commit() error {
	return tx.tx.Commit()
}

// Conn is used as a common interface for the stores so
// they don't need to worry about whether or not there's a
// transaction.
type Conn interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
}

type Tx interface {
	Rollback() error
	Commit() error
}

func (db *DB) WithTx() (*DBWithTx, error) {
	tx, err := db.db.Begin()
	if err != nil {
		return nil, err
	}

	return &DBWithTx{
		DB: &DB{
			db:            db.db,
			name:          db.name,
			dsn:           db.dsn,
			autoCreate:    db.autoCreate,
			dropExisting:  db.dropExisting,
			migrationsDir: db.migrationsDir,
			migrations:    db.migrations,
			dropOnClose:   db.dropOnClose,
			Users:         &userStore{tx},
			Entries:       &entryStore{tx},
			RefreshTokens: &refreshTokenStore{tx},
		},
		tx: tx,
	}, nil
}

// Option is an option to be applied to the DB.
type Option func(*DB)

// AutoCreateDB returns an option that will configure the DB to
// automatically create the database in the provided instance
// if it doesn't exist already.
func AutoCreateDB() Option {
	return func(db *DB) {
		db.autoCreate = true
	}
}

// DropExistingDB returns an option that will configure the DB to
// drop the database if it currently exists. This would be useful if
// the DB needs dropped before using the AutoCreateDB Option.
func DropExistingDB() Option {
	return func(db *DB) {
		db.dropExisting = true
	}
}

// WithMigrations returns an option that will configure the DB to
// perform automatic migrations. No subdirectories will be searched,
// and only files with a `.sql` extension will be run. If the directory
// string provided is empty, no migrations will be run.
func WithMigrations(migrationsDir string) Option {
	return func(db *DB) {
		db.migrationsDir = migrationsDir
	}
}

// DropDBOnClose returns an option that will configure the DB to
// drop the underlying database when the DB is closed. This is useful
// if the database is only needed temporarily e.g. for testing.
func DropDBOnClose() Option {
	return func(db *DB) {
		db.dropOnClose = true
	}
}

// NewDB returns a new DB with any necessary actions from the given options performed.
func NewDB(dsn string, options ...Option) (*DB, error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("parsing dsn: %w", err)
	}

	d := &DB{name: cfg.DBName, dsn: dsn}
	for _, o := range options {
		o(d)
	}

	if d.dropExisting {
		cfg.DBName = ""
		if err = dropExistingDatabaseIfExist(cfg.FormatDSN(), d.name); err != nil {
			return nil, fmt.Errorf("dropping existing database: %w", err)
		}
		cfg.DBName = d.name
	}

	if d.autoCreate {
		cfg.DBName = ""
		if err = createDatabaseIfNotExist(cfg.FormatDSN(), d.name); err != nil {
			return nil, fmt.Errorf("auto-creating database: %w", err)
		}
		cfg.DBName = d.name
	}

	d.db, err = sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if err = d.db.Ping(); err != nil {
		d.db.Close()
		return nil, err
	}

	if d.migrationsDir != "" {
		if err = d.runMigrations(); err != nil {
			d.db.Close()
			return nil, fmt.Errorf("running migrations: %w", err)
		}
	}

	d.Users = &userStore{d.db}
	d.Entries = &entryStore{d.db}
	d.RefreshTokens = &refreshTokenStore{d.db}

	return d, nil
}

// Close handles closing any underlying resources for the database. It also
// runs any of the specified options that are required at the end of the database's
// use, such as DropDBOnClose.
func (db *DB) Close() error {
	err := db.db.Close()
	if err != nil {
		return fmt.Errorf("closing database: %w", err)
	}

	if !db.dropOnClose {
		return nil
	}

	cfg, err := mysql.ParseDSN(db.dsn)
	if err != nil {
		return fmt.Errorf("parsing dsn: %w", err)
	}

	cfg.DBName = ""
	return dropExistingDatabaseIfExist(cfg.FormatDSN(), db.name)
}

func createDatabaseIfNotExist(dsn, dbName string) error {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("pinging database: %w", err)
	}

	_, err = db.Exec(`CREATE DATABASE IF NOT EXISTS ` + dbName + `;`)
	if err != nil {
		return fmt.Errorf("creating database: %w", err)
	}

	return nil
}

func dropExistingDatabaseIfExist(dsn, dbName string) error {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("pinging database: %w", err)
	}

	_, err = db.Exec(`DROP DATABASE IF EXISTS ` + dbName + `;`)
	if err != nil {
		return fmt.Errorf("dropping database: %w", err)
	}

	return nil
}

func (db *DB) runMigrations() error {
	_, err := db.db.Exec(`
CREATE TABLE IF NOT EXISTS __Migrations (
	ID INT NOT NULL AUTO_INCREMENT,
	` + "`" + `Name` + "`" + ` VARCHAR(255) NOT NULL,
	RunAt TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY(ID)
);`)
	if err != nil {
		return err
	}

	var fi []os.FileInfo
	fi, err = ioutil.ReadDir(db.migrationsDir)
	if err != nil {
		return fmt.Errorf("reading migrations directory: %w", err)
	}

	db.migrations = make([]string, 0)
	for _, f := range fi {
		if f.IsDir() || strings.ToLower(path.Ext(f.Name())) != ".sql" {
			continue
		}

		db.migrations = append(db.migrations, f.Name())
	}

	sort.Strings(db.migrations)

	for _, migration := range db.migrations {
		var exists mysqlBool
		row := db.db.QueryRow("SELECT COALESCE((SELECT b'1' FROM __Migrations WHERE `Name` = ?), b'0');", migration)
		if err = row.Scan(&exists); err != nil {
			return fmt.Errorf("querying for migration: %w", err)
		}

		if bool(exists) {
			continue
		}

		p := path.Join(db.migrationsDir, migration)
		s, err := ioutil.ReadFile(p)
		if err != nil {
			return fmt.Errorf("reading file %s: %w", p, err)
		}
		sql := strings.TrimSpace(string(s))

		delim := ";"
		for sql != "" {
			nextDelimIndex := strings.Index(sql, delim)
			nextDelimChangeIndex := strings.Index(sql, "delimiter ")

			if nextDelimIndex == -1 && nextDelimChangeIndex == -1 {
				return fmt.Errorf("unexpected end of migration: %s", migration)
			}

			if nextDelimChangeIndex == -1 || (nextDelimIndex != -1 && nextDelimIndex < nextDelimChangeIndex) {
				var stmt string
				// only include the delimiter if it's a semi-colon
				if delim == ";" {
					stmt = sql[:nextDelimIndex+1]
				} else {
					stmt = sql[:nextDelimIndex]
				}

				if _, err = db.db.Exec(stmt); err != nil {
					return fmt.Errorf("executing migration statement: %w", err)
				}

				if len(sql) <= nextDelimIndex {
					break
				}
				sql = strings.TrimSpace(sql[nextDelimIndex+1:])

				continue
			}

			delimLineEndIndex := strings.Index(sql, "\n")
			if delimLineEndIndex == -1 {
				// there's nothing after this delimiter change, so we're done with the script
				break
			}

			delim = strings.Replace(sql[:delimLineEndIndex+1], "delimiter ", "", 1)
			delim = strings.Replace(delim, "\n", "", -1)
			delim = strings.TrimSpace(delim)

			// advance the sql past the delimiter change statement since the client will
			// only handle this correctly without it, or break if it's the end of the script
			if len(sql) <= delimLineEndIndex {
				break
			}
			sql = strings.TrimSpace(sql[delimLineEndIndex+1:])
		}

		_, err = db.db.Exec("INSERT INTO __Migrations(`Name`) VALUES (?);", migration)
		if err != nil {
			return fmt.Errorf("inserting migration record '%s': %w", migration, err)
		}
	}

	return nil
}

type mysqlBool bool

func (b *mysqlBool) Scan(src interface{}) error {
	tmp, ok := src.([]uint8)
	if !ok {
		return fmt.Errorf("unexpected type for mysqlBool: %T", src)
	}
	switch string(tmp) {
	case "\x00":
		v := mysqlBool(false)
		*b = v
	case "\x01":
		v := mysqlBool(true)
		*b = v
	}
	return nil
}

func (b mysqlBool) Value() interface{} {
	if b {
		return []uint8("\x01")
	}
	return []uint8("\x00")
}

type mysqlUUID string

func (u *mysqlUUID) Scan(src interface{}) error {
	tmp, ok := src.([]byte)
	if !ok {
		return fmt.Errorf("unexpected type for mysqlUUID: %T", src)
	}

	*u = mysqlUUID(string(tmp))
	return nil
}

func (u mysqlUUID) Value() interface{} {
	return u
}

func (u mysqlUUID) UUID() uuid.UUID {
	return uuid.MustParse(hex.EncodeToString([]byte(u)))
}
