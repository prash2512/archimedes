package datastore

import "github.com/prashanth/archimedes/internal/blocks"

type SQL struct{}

func (SQL) Kind() string { return "sql_datastore" }
func (SQL) Name() string { return "SQL Datastore" }

func init() { blocks.Types = append(blocks.Types, SQL{}) }
