// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSE for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2024 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2024 Intevation GmbH <https://intevation.de>

// Package config implements the configuration mechanisms.
package config

import (
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"time"

	"github.com/BurntSushi/toml"
)

// DefaultConfigFile is the name of the default config file.
const DefaultConfigFile = "oqcd.toml"

const (
	defaultLogFile   = "oqcd.log"
	defaultLogLevel  = slog.LevelInfo
	defaultLogSource = false
	defaultLogJSON   = false
)

const (
	defaultWebHost = "localhost"
	defaultWebPort = 8083
	defaultWebRoot = "web"
)

const (
	defaultDatabaseURL                     = "oqcd.sqlite"
	defaultDatabaseDriver                  = "sqlite3"
	defaultDatabaseMigrate                 = false
	defaultDatabaseTerminateAfterMigration = true
	defaultDatabaseMaxOpenConnections      = 0
	defaultDatabaseMaxIdleConnections      = 0
	defaultDatabaseConnMaxLifetime         = 0
	defaultDatabaseConnMaxIdletime         = 0
)

// Log are the config options for the logging.
type Log struct {
	File   string     `toml:"file"`
	Level  slog.Level `toml:"level"`
	Source bool       `toml:"source"`
	JSON   bool       `toml:"json"`
}

// Web are the config options for the web interface.
type Web struct {
	Host string `toml:"host"`
	Port int    `toml:"port"`
	Root string `toml:"root"`
}

// Database are the config options for the database.
type Database struct {
	DatabaseURL             string        `toml:"database"`
	Driver                  string        `toml:"driver"`
	Migrate                 bool          `toml:"migrate"`
	TerminateAfterMigration bool          `toml:"terminate_after_migration"`
	MaxOpenConnections      int           `toml:"max_open_conns"`
	MaxIdleConnections      int           `toml:"max_idle_conns"`
	ConnMaxLifetime         time.Duration `toml:"conn_max_lifetime"`
	ConnMaxIdletime         time.Duration `toml:"conn_max_idletime"`
}

// Config are all the configuration options.
type Config struct {
	Log      Log      `toml:"log"`
	Web      Web      `toml:"web"`
	Database Database `toml:"database"`
	Sessions Sessions `toml:"sessions"`
}

// Addr returns the combined address the web server should bind to.
func (w *Web) Addr() string {
	return net.JoinHostPort(w.Host, strconv.Itoa(w.Port))
}

// Load loads the configuration from a given file. An empty string
// resorts to the default configuration.
func Load(file string) (*Config, error) {
	cfg := &Config{
		Log: Log{
			File:   defaultLogFile,
			Level:  defaultLogLevel,
			Source: defaultLogSource,
			JSON:   defaultLogJSON,
		},
		Web: Web{
			Host: defaultWebHost,
			Port: defaultWebPort,
			Root: defaultWebRoot,
		},
		Database: Database{
			DatabaseURL:             defaultDatabaseURL,
			Driver:                  defaultDatabaseDriver,
			Migrate:                 defaultDatabaseMigrate,
			TerminateAfterMigration: defaultDatabaseTerminateAfterMigration,
			MaxOpenConnections:      defaultDatabaseMaxOpenConnections,
			MaxIdleConnections:      defaultDatabaseMaxIdleConnections,
			ConnMaxLifetime:         defaultDatabaseConnMaxLifetime,
			ConnMaxIdletime:         defaultDatabaseConnMaxIdletime,
		},
		Sessions: Sessions{
			Secret: nil,
			MaxAge: defaultSessionMaxAge,
		},
	}
	if file != "" {
		md, err := toml.DecodeFile(file, cfg)
		if err != nil {
			return nil, err
		}
		// Don't accept unknown entries in config file.
		if undecoded := md.Undecoded(); len(undecoded) != 0 {
			return nil, fmt.Errorf("config: could not parse %q", undecoded)
		}
	}
	if err := cfg.fillFromEnv(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// PresetDefaults initializes unset values.
func (cfg *Config) PresetDefaults() {
	cfg.Sessions.presetDefaults()
}

func (cfg *Config) fillFromEnv() error {
	var (
		storeString   = store(noparse)
		storeInt      = store(strconv.Atoi)
		storeBool     = store(strconv.ParseBool)
		storeLevel    = store(storeLevel)
		storeDuration = store(time.ParseDuration)
	)
	return storeFromEnv(
		envStore{"OQC_LOG_FILE", storeString(&cfg.Log.File)},
		envStore{"OQC_LOG_LEVEL", storeLevel(&cfg.Log.Level)},
		envStore{"OQC_LOG_JSON", storeBool(&cfg.Log.JSON)},
		envStore{"OQC_LOG_SOURCE", storeBool(&cfg.Log.Source)},
		envStore{"OQC_WEB_HOST", storeString(&cfg.Web.Host)},
		envStore{"OQC_WEB_PORT", storeInt(&cfg.Web.Port)},
		envStore{"OQC_WEB_ROOT", storeString(&cfg.Web.Root)},
		envStore{"OQC_DB_URL", storeString(&cfg.Database.DatabaseURL)},
		envStore{"OQC_DB_MIGRATE", storeBool(&cfg.Database.Migrate)},
		envStore{"OQC_DB_TERMINATE_AFTER_MIGRATION", storeBool(&cfg.Database.TerminateAfterMigration)},
		envStore{"OQC_DB_MAX_OPEN_CONNS", storeInt(&cfg.Database.MaxOpenConnections)},
		envStore{"OQC_DB_MAX_IDLE_CONNS", storeInt(&cfg.Database.MaxIdleConnections)},
		envStore{"OQC_DB_CONN_MAX_LIFETIME", storeDuration(&cfg.Database.ConnMaxLifetime)},
		envStore{"OQC_DB_CONN_MAX_IDLETIME", storeDuration(&cfg.Database.ConnMaxIdletime)},
		// TODO: Make session vars over-writable by env vars, too.
	)
}
