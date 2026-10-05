package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	mysql "github.com/go-sql-driver/mysql"
)

func TestMariaDBStatusTransitionWaitsForOutboxWriteLane(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	transition := reversibleStatusTransitionFixture()
	mock.ExpectBegin()
	expectActiveReversibleSource(mock, transition.Event.ChatSessionID, transition.SourceRevision)
	mock.ExpectQuery("FROM status_change_events").WillReturnRows(reversibleStatusEventRows(202, 101, transition.Event))
	mock.ExpectQuery("FROM status_current_values").WillReturnRows(sqlmock.NewRows(reversibleStatusCurrentColumns))
	mock.ExpectCommit()
	m.memoryDerivationWriteMu.Lock()
	locked := true
	defer func() {
		if locked {
			m.memoryDerivationWriteMu.Unlock()
		}
	}()
	done := make(chan error, 1)
	go func() { _, err := m.ApplyReversibleStatusTransition(context.Background(), transition); done <- err }()
	select {
	case err := <-done:
		t.Fatalf("status transition bypassed outbox write lane: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	m.memoryDerivationWriteMu.Unlock()
	locked = false
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("status transition did not resume")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// This connector delays only the scheduling boundary after the production
// outbox UPDATE. Every SQL statement still runs against real InnoDB unchanged.
type statusOutboxBarrierConnector struct {
	driver.Connector
	updated, release chan struct{}
	once             sync.Once
}

func (c *statusOutboxBarrierConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.Connector.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &statusOutboxBarrierConn{Conn: conn, barrier: c}, nil
}

type statusOutboxBarrierConn struct {
	driver.Conn
	barrier *statusOutboxBarrierConnector
}

func (c *statusOutboxBarrierConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	return c.Conn.(driver.ConnBeginTx).BeginTx(ctx, opts)
}
func (c *statusOutboxBarrierConn) QueryContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	return c.Conn.(driver.QueryerContext).QueryContext(ctx, q, args)
}
func (c *statusOutboxBarrierConn) ExecContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	result, err := c.Conn.(driver.ExecerContext).ExecContext(ctx, q, args)
	if err == nil && strings.Contains(q, "UPDATE memory_vector_outbox o") {
		c.barrier.once.Do(func() {
			close(c.barrier.updated)
			select {
			case <-c.barrier.release:
			case <-ctx.Done():
			}
		})
	}
	return result, err
}

func TestMariaDBStatusOutboxDeadlockIntegration(t *testing.T) {
	dsn := os.Getenv("AC_STORE_DEADLOCK_TEST_DSN")
	if dsn == "" {
		t.Skip("requires isolated local MariaDB on port 33318")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Net != "tcp" || cfg.Addr != "127.0.0.1:33318" || cfg.DBName != "" {
		t.Fatal("integration DSN must use 127.0.0.1:33318 with no existing database")
	}
	cfg.ParseTime, cfg.InterpolateParams, cfg.MultiStatements = true, true, true
	adminConnector, err := mysql.NewConnector(cfg)
	if err != nil {
		t.Fatal(err)
	}
	admin := sql.OpenDB(adminConnector)
	defer admin.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	name := fmt.Sprintf("sol_deadlock_%d", time.Now().UnixNano())
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+name+" CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.Exec("DROP DATABASE " + name); err != nil {
			t.Errorf("cleanup database: %v", err)
		}
	}()
	cfg.DBName = name
	connector, err := mysql.NewConnector(cfg)
	if err != nil {
		t.Fatal(err)
	}
	observer := sql.OpenDB(connector)
	defer observer.Close()
	schema, err := os.ReadFile("../../../migrations/001_schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := observer.ExecContext(ctx, string(schema)); err != nil {
		t.Fatal(err)
	}
	barrier := &statusOutboxBarrierConnector{Connector: connector, updated: make(chan struct{}), release: make(chan struct{})}
	db := sql.OpenDB(barrier)
	defer db.Close()
	m := &mariadbStore{db: db}
	transition := reversibleStatusTransitionFixture()
	transition.Event.StatusKey, transition.CurrentValue.StatusKey = "narrative_state", "narrative_state"
	transition.Event.EvidenceJSON = `{"source_revision":"revision-1","source_unit_id":"unit-1","current_projection":true}`
	transition.CurrentValue.EvidenceJSON = transition.Event.EvidenceJSON
	seed := `INSERT INTO memory_source_revisions (source_revision,chat_session_id,logical_turn_id,turn_index,raw_user_content,raw_assistant_content,combined_content_hash,hash_algorithm,host_observed_at_ms) VALUES ('revision-1','session-1','turn-3',3,'synthetic user','synthetic assistant',REPEAT('a',64),'sha256',1);
	INSERT INTO status_schema_registry (id,chat_session_id,status_key,label,owner_scope,value_kind) VALUES (7,'session-1','narrative_state','Synthetic state','fictional_entity','object');`
	if _, err := observer.ExecContext(ctx, seed); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for i := 0; i < 4; i++ {
		_, err := m.EnqueueMemoryVectorOperation(ctx, &MemoryVectorOutboxItem{OperationKey: fmt.Sprintf("%064d", i+1), Operation: "upsert", ChatSessionID: "session-1", SourceRevision: "revision-1", DocumentID: fmt.Sprintf("synthetic-%d", i), DocumentJSON: `{}`, Status: "needs_embedding", CreatedAt: now, UpdatedAt: now})
		if err != nil {
			t.Fatal(err)
		}
	}
	claims := make(chan error, 1)
	go func() {
		items, err := m.ClaimMemoryVectorOperations(ctx, "synthetic-worker", now, time.Minute)
		if err == nil && len(items) != 4 {
			err = fmt.Errorf("claimed %d operations, want all four", len(items))
		}
		claims <- err
	}()
	select {
	case <-barrier.updated:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	states := make(chan error, 1)
	go func() { _, err := m.ApplyReversibleStatusTransition(ctx, transition); states <- err }()
	// Broken code enters SQL and waits on the PRIMARY lock held by the outbox.
	// Fixed code waits outside SQL on the shared owner lane. Release in either
	// case so the regression checks actual commits rather than a timeout.
	waitObserved := false
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		var count int
		if err := observer.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.INNODB_LOCK_WAITS`).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count > 0 {
			waitObserved = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	close(barrier.release)
	claimErr, stateErr := <-claims, <-states
	t.Logf("source lock wait observed=%t claim_error=%v state_error=%v", waitObserved, claimErr, stateErr)
	if claimErr != nil || stateErr != nil {
		var kind, engine, status string
		if err := observer.QueryRowContext(ctx, "SHOW ENGINE INNODB STATUS").Scan(&kind, &engine, &status); err == nil {
			t.Log(status)
		}
		t.Fatalf("first concurrent outbox/narrative commit failed: claim=%v state=%v", claimErr, stateErr)
	}
	var current, events, leased int
	if err := observer.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM status_current_values),(SELECT COUNT(*) FROM status_change_events),(SELECT COUNT(*) FROM memory_vector_outbox WHERE status='leased' AND attempts=1)`).Scan(&current, &events, &leased); err != nil {
		t.Fatal(err)
	}
	if current != 1 || events != 1 || leased != 4 {
		t.Fatalf("non-atomic result: current=%d events=%d leased=%d", current, events, leased)
	}
	replayed, err := m.ApplyReversibleStatusTransition(ctx, transition)
	if err != nil || !replayed.Replayed {
		t.Fatalf("exact replay=%t err=%v", replayed.Replayed, err)
	}
	t.Log("first writes committed: current=1 event=1 leased=4; exact replay preserved")
}
