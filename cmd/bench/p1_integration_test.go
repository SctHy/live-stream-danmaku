package main

import (
	"os"
	"testing"
)

func TestP1Postgres(t *testing.T) {
	dsn := os.Getenv("DANMAKU_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires PostgreSQL")
	}
	s := p1Suite{ctx: t.Context(), repeats: 2}
	if err := s.postgres(dsn); err != nil {
		t.Fatal(err)
	}
	for _, e := range s.report.Experiments {
		if !e.Passed {
			t.Fatal(e.Name, e.Load)
		}
	}
}
func TestP1Redis(t *testing.T) {
	addr := os.Getenv("DANMAKU_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("requires Redis")
	}
	s := p1Suite{ctx: t.Context(), repeats: 2}
	if err := s.redisSuite(addr); err != nil {
		t.Fatal(err)
	}
	for _, e := range s.report.Experiments {
		if !e.Passed {
			t.Fatal(e.Name, e.Load)
		}
	}
}
