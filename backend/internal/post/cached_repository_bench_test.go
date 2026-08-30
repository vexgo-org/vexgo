package post

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/cache"
	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// These benchmarks compare the repository wirings the cache layer introduced
// on the public read paths: reading straight from the database (the pre-cache
// behavior) versus the read-through decorator on the memory backend, an
// in-process miniredis server, and a real Valkey server.
//
// VEXGO_BENCH_CACHE selects the wiring ("db", "memory", "miniredis" or
// "valkey"). The variants run under identical benchmark names so benchstat
// can compare them across runs:
//
//	VEXGO_BENCH_CACHE=db        go test -run '^$' -bench BenchmarkPost -benchmem -count=10 ./backend/internal/post/ > db.txt
//	VEXGO_BENCH_CACHE=memory    ... > memory.txt
//	VEXGO_BENCH_CACHE=miniredis ... > miniredis.txt
//	VEXGO_BENCH_CACHE=valkey    ... > valkey.txt
//	benchstat db.txt memory.txt miniredis.txt valkey.txt
//
// The valkey variant needs a reachable server; VEXGO_BENCH_VALKEY_URL
// overrides its URL (default valkey://127.0.0.1:16379/0). The miniredis
// variant talks RESP over loopback TCP to an in-process server: it exercises
// the full client and wire stack but not a production server's compute, so
// treat its absolute numbers as optimistic. The db variant uses in-process
// sqlite, which is also optimistic compared to a networked production
// database.

// benchBackendEnv selects the repository wiring under measurement.
const benchBackendEnv = "VEXGO_BENCH_CACHE"

// benchValkeyURLEnv overrides the real Valkey server URL for the "valkey"
// wiring.
const benchValkeyURLEnv = "VEXGO_BENCH_VALKEY_URL"

// benchDefaultValkeyURL is the real Valkey server the "valkey" wiring
// connects to when benchValkeyURLEnv is unset.
const benchDefaultValkeyURL = "valkey://127.0.0.1:16379"

const (
	benchPostCount     = 100
	benchPostContentKB = 2
)

// benchSeed opens an in-memory sqlite database and seeds one author, ten
// tags and benchPostCount published posts with realistic content sizes. It
// returns the database and the seeded slugs.
func benchSeed(b *testing.B) (*gorm.DB, []string) {
	b.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		b.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&model.Post{}, &model.User{}, &model.Tag{}); err != nil {
		b.Fatalf("migrate: %v", err)
	}

	author := model.User{Username: "author", Email: "author@example.com", Role: model.RoleAuthor}
	if err := db.Create(&author).Error; err != nil {
		b.Fatalf("seed author: %v", err)
	}
	tags := make([]model.Tag, 10)
	for i := range tags {
		tags[i] = model.Tag{Name: fmt.Sprintf("tag-%02d", i)}
	}
	if err := db.Create(&tags).Error; err != nil {
		b.Fatalf("seed tags: %v", err)
	}

	content := strings.Repeat("Lorem ipsum dolor sit amet, consectetur adipiscing elit. ", benchPostContentKB)
	slugs := make([]string, benchPostCount)
	posts := make([]model.Post, benchPostCount)
	for i := range posts {
		slugs[i] = fmt.Sprintf("bench-post-%03d", i)
		posts[i] = model.Post{
			Slug:     slugs[i],
			Title:    fmt.Sprintf("Benchmark post %03d", i),
			Content:  content,
			Excerpt:  "lorem ipsum",
			Category: "general",
			Status:   model.PostStatusPublished,
			AuthorID: author.ID,
			Tags:     tags[i%8 : i%8+3],
		}
	}
	if err := db.CreateInBatches(&posts, 50).Error; err != nil {
		b.Fatalf("seed posts: %v", err)
	}
	return db, slugs
}

// benchRepository wires the variant selected by benchBackendEnv; see the
// file comment for how to run the comparison.
func benchRepository(b *testing.B, db *gorm.DB) Repository {
	b.Helper()
	switch backend := os.Getenv(benchBackendEnv); backend {
	case "", "db":
		return NewRepository(db)
	case "memory":
		return NewCachedRepository(NewRepository(db), cache.NewMemory())
	case "miniredis":
		mr := miniredis.RunT(b)
		backend, err := cache.NewValkey(context.Background(), "redis://"+mr.Addr())
		if err != nil {
			b.Fatalf("connect miniredis: %v", err)
		}
		b.Cleanup(backend.Close)
		return NewCachedRepository(NewRepository(db), backend)
	case "valkey":
		url := os.Getenv(benchValkeyURLEnv)
		if url == "" {
			url = benchDefaultValkeyURL
		}
		backend, err := cache.NewValkey(context.Background(), url)
		if err != nil {
			b.Fatalf("connect valkey at %s: %v", url, err)
		}
		b.Cleanup(backend.Close)
		return NewCachedRepository(NewRepository(db), backend)
	default:
		b.Fatalf("invalid %s %q: want db, memory, miniredis or valkey", benchBackendEnv, backend)
		return nil
	}
}

// warm runs the measured call once so the read-through backends serve hits
// inside the loop; setup runs before b.Loop and is excluded from timing.
func warm(b *testing.B, call func() error) {
	b.Helper()
	if err := call(); err != nil {
		b.Fatalf("warm cache: %v", err)
	}
}

func BenchmarkPostFindBySlug(b *testing.B) {
	ctx := context.Background()
	db, slugs := benchSeed(b)
	repo := benchRepository(b, db)

	warm(b, func() error { _, err := repo.FindBySlug(ctx, slugs[0]); return err })

	b.ReportAllocs()
	for b.Loop() {
		if _, err := repo.FindBySlug(ctx, slugs[0]); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPostList(b *testing.B) {
	ctx := context.Background()
	db, _ := benchSeed(b)
	repo := benchRepository(b, db)
	filter := ListFilter{Page: 1, Limit: 20}

	warm(b, func() error { _, _, err := repo.List(ctx, "", 0, filter); return err })

	b.ReportAllocs()
	for b.Loop() {
		posts, total, err := repo.List(ctx, "", 0, filter)
		if err != nil || total != benchPostCount {
			b.Fatalf("List = %d posts, %d total, %v", len(posts), total, err)
		}
	}
}

func BenchmarkPostPopular(b *testing.B) {
	ctx := context.Background()
	db, _ := benchSeed(b)
	repo := benchRepository(b, db)

	warm(b, func() error { _, err := repo.Popular(ctx); return err })

	b.ReportAllocs()
	for b.Loop() {
		posts, err := repo.Popular(ctx)
		if err != nil || len(posts) != benchPostCount {
			b.Fatalf("Popular = %d posts, %v", len(posts), err)
		}
	}
}
