package main

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"
)

func TestAppAccountPermissionProbeMariaDBIntegration(t *testing.T) {
	if os.Getenv("AC_MARIADB_APP_ACCOUNT_PROBE_LIVE") != "YES" {
		t.Skip("set AC_MARIADB_APP_ACCOUNT_PROBE_LIVE=YES for the explicit mutating local integration probe")
	}
	dsn := strings.TrimSpace(os.Getenv("AC_MARIADB_APP_ACCOUNT_PROBE_DSN"))
	targets, err := localAppAccountProbeTargets(dsn)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	for _, target := range targets {
		db, err := sql.Open("mysql", target.dsn)
		if err != nil {
			t.Fatalf("open local application-account connection for host %s", target.host)
		}
		report, probeErr := runAppAccountPermissionProbe(ctx, db, "")
		report.TargetHost = target.host
		_ = db.Close()
		if probeErr != nil {
			t.Fatalf("runAppAccountPermissionProbe host=%s: %v report=%+v", target.host, probeErr, report)
		}
		if report.Status != "ok" || report.CleanupStatus != "ok" {
			t.Fatalf("unexpected live report for host %s: %+v", target.host, report)
		}
	}
}
