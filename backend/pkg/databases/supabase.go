package databases

import (
    "context"
    "github.com/jackc/pgx/v5/pgxpool"
	"fmt"
)

type Event struct {
    Name      string
    Date      string
    Location  string
    MainEvent string
}

type SupabaseDB struct {
    pool *pgxpool.Pool
}

var db *SupabaseDB

func GetDB() *SupabaseDB {
    if db == nil {
        databaseURL := "your_supabase_connection_string"
        var err error
        db, err = NewSupabaseDB(databaseURL)
        if err != nil {
            return nil
        }
    }
    return db
}

func (db *SupabaseDB) Close(ctx context.Context) {
    if db.pool != nil {
        db.pool.Close()
    }
}

func BatchInsertEvents(events []*Event) error {
    if db == nil {
        return fmt.Errorf("database connection not initialized")
    }

    ctx := context.Background()
    query := `
        INSERT INTO events (name, date, location, main_event)
        VALUES ($1, $2, $3, $4)
    `

    for _, event := range events {
        _, err := db.pool.Exec(ctx, query,
            event.Name,
            event.Date,
            event.Location,
            event.MainEvent,
        )
        if err != nil {
            return fmt.Errorf("error inserting event %s: %w", event.Name, err)
        }
    }

    return nil
}

func NewSupabaseDB(databaseURL string) (*SupabaseDB, error) {
    config, err := pgxpool.ParseConfig(databaseURL)
    if err != nil {
        return nil, err
    }

    pool, err := pgxpool.NewWithConfig(context.Background(), config)
    if err != nil {
        return nil, err
    }

    return &SupabaseDB{pool: pool}, nil
}

func (db *SupabaseDB) InsertEvent(ctx context.Context, event *Event) error {
    query := `
        INSERT INTO events (name, date, location, main_event)
        VALUES ($1, $2, $3, $4)
    `
    _, err := db.pool.Exec(ctx, query, 
        event.Name,
        event.Date,
        event.Location,
        event.MainEvent,
    )
    return err
}