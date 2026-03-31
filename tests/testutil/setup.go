package testutil

import (
	"log"
	"os"
	"testing"
)

const testSuiteLockKey int64 = 2026030501

// RunTestMain initializes DB and runs tests. Call from each test package's TestMain.
func RunTestMain(m *testing.M) {
	InitDB()
	if _, err := TestDB.Exec("SELECT pg_advisory_lock($1)", testSuiteLockKey); err != nil {
		panic("failed to acquire test advisory lock: " + err.Error())
	}
	log.Printf("acquired test advisory lock: %d", testSuiteLockKey)

	code := m.Run()

	if _, err := TestDB.Exec("SELECT pg_advisory_unlock($1)", testSuiteLockKey); err != nil {
		log.Printf("failed to release test advisory lock: %v", err)
	}
	os.Exit(code)
}
